import axios from 'axios'
import { ElMessage } from 'element-plus'

// 创建axios实例
const api = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器
api.interceptors.request.use(
  config => {
    // 从localStorage获取token并添加到请求头
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

// 响应拦截器
api.interceptors.response.use(
  async response => {
    const { data } = response

    // 检查是否为统一响应格式
    if (data && typeof data === 'object' && 'code' in data && 'msg' in data) {
      // 统一响应格式处理
      if (data.code === 200) {
        // 成功响应，返回data字段
        return data.data
      } else {
        // 错误响应，显示错误消息并抛出错误
        const message = data.msg || '请求失败'

        // 401错误特殊处理：清除token并跳转登录页
        if (data.code === 401) {
          localStorage.removeItem('token')
          // 清除store中的认证状态
          const { useAuthStore } = await import('@/store')
          const authStore = useAuthStore()
          authStore.clearAuth()

          // 判断当前是否在管理后台，跳转到对应的登录页
          if (window.location.pathname.startsWith('/admin')) {
            if (window.location.pathname !== '/admin') {
              window.location.href = '/admin'
            }
          }
        }
        // 封禁错误只显示一次，不在这里显示（让调用方处理）
        const isBanned = message.includes('封禁') || message.includes('禁用')
        if (!isBanned && data.code !== 401) {
          // 非401/封禁错误显示错误消息
          ElMessage.error(message)
        }

        // 抛出错误以便在catch中处理
        const error = new Error(message)
        error.code = data.code
        error.data = data.data
        // 标记错误已在拦截器中显示过（封禁错误或已显示的错误）
        error.handled = isBanned || (data.code !== 401)
        throw error
      }
    }

    // 兼容旧格式或其他格式
    return data
  },
  async error => {
    const { response } = error
    let message = '请求失败'
    let code = 500

    if (response) {
      // 检查响应数据是否为统一格式
      if (response.data && typeof response.data === 'object' && 'code' in response.data && 'msg' in response.data) {
        // 统一响应格式的错误
        code = response.data.code
        message = response.data.msg
      } else {
        // 传统格式的错误处理
        code = response.status
        switch (response.status) {
          case 400:
            message = response.data?.error || '请求参数错误'
            break
          case 401:
            message = '登录已过期，请重新登录'
            break
          case 403:
            message = '禁止访问'
            break
          case 404:
            message = '资源不存在'
            break
          case 429:
            message = '请求过于频繁'
            break
          case 500:
            message = '服务器内部错误'
            break
          default:
            message = response.data?.error || `请求失败 (${response.status})`
        }
      }

      // 401错误特殊处理：清除token并跳转登录页
      if (code === 401) {
        localStorage.removeItem('token')
        // 清除store中的认证状态
        const { useAuthStore } = await import('@/store')
        const authStore = useAuthStore()
        authStore.clearAuth()

        // 判断当前是否在管理后台，跳转到对应的登录页
        if (window.location.pathname.startsWith('/admin')) {
          if (window.location.pathname !== '/admin') {
            window.location.href = '/admin'
          }
        }
      }
    } else if (error.code === 'ECONNABORTED') {
      message = '请求超时'
    } else if (error.code === 'ERR_NETWORK') {
      message = '网络连接失败，请检查网络或确认后端服务已启动'
    } else {
      message = '网络错误'
    }

    // 只在非401错误时显示错误消息，401错误已经有提示
    if (code !== 401) {
      ElMessage.error(message)
    }

    // 创建统一的错误对象
    const unifiedError = new Error(message)
    unifiedError.code = code
    unifiedError.response = response
    unifiedError.handled = true // 标记错误已在拦截器中显示过
    return Promise.reject(unifiedError)
  }
)

// 认证API
export const authAPI = {
  // 用户登录
  login(data) {
    return api.post('/auth/login', data)
  },

  // 用户登出
  logout() {
    return api.post('/auth/logout')
  },

  // 获取当前用户信息
  me() {
    return api.get('/auth/me')
  },

  // 更新用户信息
  updateProfile(data) {
    return api.put('/auth/profile', data)
  }
}

// Token API
export const tokenAPI = {
  // 获取token列表
  list(params = {}) {
    return api.get('/tokens', { params })
  },
  
  // 创建token
  create(data) {
    return api.post('/tokens', data)
  },
  
  // 获取token详情
  get(id) {
    return api.get(`/tokens/${id}`)
  },
  
  // 更新token
  update(id, data) {
    return api.put(`/tokens/${id}`, data)
  },
  
  // 删除token
  delete(id) {
    return api.delete(`/tokens/${id}`)
  },

  // 获取TOKEN使用用户列表
  getTokenUsers(id) {
    return api.get(`/tokens/${id}/users`)
  },

  // 验证token
  validate(token) {
    return api.get('/tokens/validate', { params: { token } })
  },

  // 获取token统计
  stats() {
    return api.get('/tokens/stats')
  },

  // 批量刷新AuthSession
  batchRefreshAuthSession(tokenIds) {
    return api.post('/tokens/batch-refresh-auth-session', { token_ids: tokenIds })
  },

  // 获取TOKEN封禁原因（通过AuthSession调用远程API）
  getBanReason(id) {
    return api.get(`/tokens/${id}/ban-reason`)
  }
}

// 用户管理 API（管理员）
export const userManagementAPI = {
  // 获取用户列表
  list(params = {}) {
    return api.get('/users', { params })
  },

  // 更新用户信息
  update(id, data) {
    return api.put(`/users/${id}`, data)
  },

  // 封禁用户
  banUser(id) {
    return api.post(`/users/${id}/ban`)
  },

  // 解封用户
  unbanUser(id) {
    return api.post(`/users/${id}/unban`)
  },

  // 切换用户共享权限
  toggleSharedPermission(id, canUseShared) {
    return api.post(`/users/${id}/toggle-shared`, { can_use_shared: canUseShared })
  }
}

// 用户认证 API
export const userAuthAPI = {
  // 用户注册
  register(data) {
    return api.post('/user-auth/register', data)
  },

  // 用户登录
  login(data) {
    return api.post('/user-auth/login', data)
  },

  // 用户登出
  logout() {
    return userRequest({
      url: '/user-auth/logout',
      method: 'post'
    })
  },

  // 刷新令牌
  refresh(refreshToken) {
    return api.post('/user-auth/refresh', { refresh_token: refreshToken })
  },

  // 获取当前用户信息
  me() {
    return userRequest({
      url: '/user/me',
      method: 'get'
    })
  },

  // 更新用户信息
  updateProfile(data) {
    return userRequest({
      url: '/user/profile',
      method: 'put',
      data
    })
  },

  // 修改密码
  changePassword(data) {
    return userRequest({
      url: '/user/change-password',
      method: 'post',
      data
    })
  },

  // 重新生成API令牌
  regenerateToken() {
    return userRequest({
      url: '/user/regenerate-token',
      method: 'post'
    })
  },

  // 获取用户设置
  getSettings() {
    return userRequest({
      url: '/user/settings',
      method: 'get'
    })
  },

  // 更新用户设置
  updateSettings(data) {
    return userRequest({
      url: '/user/settings',
      method: 'put',
      data
    })
  }
}

// 公告管理API
export const notificationAPI = {
  // 获取公告列表
  getList() {
    return api.get('/notifications')
  },

  // 创建公告
  create(data) {
    return api.post('/notifications', data)
  },

  // 获取公告详情
  get(id) {
    return api.get(`/notifications/${id}`)
  },

  // 更新公告
  update(id, data) {
    return api.put(`/notifications/${id}`, data)
  },

  // 删除公告
  delete(id) {
    return api.delete(`/notifications/${id}`)
  },

  // 启用公告
  enable(id) {
    return api.post(`/notifications/${id}/enable`)
  },

  // 禁用公告
  disable(id) {
    return api.post(`/notifications/${id}/disable`)
  }
}

// 统计API
export const statsAPI = {
  // 获取概览统计
  overview() {
    return api.get('/stats/overview')
  },

  // 获取请求趋势
  trend(params = {}) {
    return api.get('/stats/trend', { params })
  },

  // 获取token统计
  tokenStats(id) {
    return api.get(`/stats/tokens/${id}`)
  },

  // 获取使用历史
  usage(params = {}) {
    return api.get('/stats/usage', { params })
  },

  // 清理旧日志
  cleanup(days = 30) {
    return api.post('/stats/cleanup', { days })
  }
}

// 创建用户API的axios实例（用于用户身份验证的API）
const userRequest = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 防止登录过期提示重复显示的标志
let isUserTokenExpiredHandling = false

// 防止重复刷新token的标志
let isRefreshing = false
// 等待刷新完成的请求队列
let refreshSubscribers = []

// 页面初始加载标志：用于区分页面刷新和用户操作中token过期的场景
// 页面刷新时token过期直接跳转，操作中过期才显示弹窗
let isPageInitialLoading = true

// 页面加载完成后设置为false（延迟一小段时间确保初始请求已发出）
if (typeof window !== 'undefined') {
  setTimeout(() => {
    isPageInitialLoading = false
  }, 2000)
}

// 通知所有等待的请求继续执行
const onRefreshed = (newToken) => {
  refreshSubscribers.forEach(callback => callback(newToken))
  refreshSubscribers = []
}

// 添加请求到等待队列
const addRefreshSubscriber = (callback) => {
  refreshSubscribers.push(callback)
}

// 刷新token函数
const refreshUserToken = async () => {
  const refreshToken = localStorage.getItem('user_refresh_token')
  if (!refreshToken) {
    return null
  }

  try {
    // 直接使用 axios 发送刷新请求，避免使用被拦截的实例
    const response = await axios.post('/api/v1/user-auth/refresh', { refresh_token: refreshToken })
    if (response.data && response.data.code === 200 && response.data.data) {
      const { token, expires_in } = response.data.data
      // 只更新访问令牌，刷新令牌保持不变（刷新令牌过期后需重新登录）
      localStorage.setItem('user_token', token)
      // 计算新的过期时间
      if (expires_in) {
        const expiresAt = new Date(Date.now() + expires_in * 1000).toISOString()
        localStorage.setItem('user_token_expires_at', expiresAt)
      }
      return token
    }
    return null
  } catch (error) {
    return null
  }
}

// 用户请求拦截器
userRequest.interceptors.request.use(
  config => {
    // 如果已经检测到登录过期，直接取消后续请求
    if (isUserTokenExpiredHandling) {
      const cancelError = new Error('登录已过期，请求已取消')
      cancelError.silent = true
      return Promise.reject(cancelError)
    }
    // 从localStorage获取用户JWT令牌并添加到请求头
    const userToken = localStorage.getItem('user_token')
    if (userToken) {
      config.headers.Authorization = `Bearer ${userToken}`
    }
    return config
  },
  error => {
    return Promise.reject(error)
  }
)

// 用户响应拦截器
userRequest.interceptors.response.use(
  async response => {
    const { data, config: originalRequest } = response

    // 检查是否为统一响应格式
    if (data && typeof data === 'object' && 'code' in data && 'msg' in data) {
      // 统一响应格式处理
      if (data.code === 200) {
        // 成功响应，返回data字段
        return data.data
      } else {
        // 错误响应，显示错误消息并抛出错误
        const message = data.msg || '请求失败'

        // 401 错误：尝试刷新 token
        if (data.code === 401 && !originalRequest._retry) {
          // 如果已经在刷新中，将请求加入等待队列
          if (isRefreshing) {
            return new Promise((resolve, reject) => {
              addRefreshSubscriber(newToken => {
                if (newToken) {
                  originalRequest._retry = true // 防止刷新后的请求再次触发刷新
                  originalRequest.headers.Authorization = `Bearer ${newToken}`
                  resolve(userRequest(originalRequest))
                } else {
                  reject(new Error(message))
                }
              })
            })
          }

          originalRequest._retry = true
          isRefreshing = true

          try {
            const newToken = await refreshUserToken()
            if (newToken) {
              // 刷新成功，更新请求头并重新发送原请求
              originalRequest.headers.Authorization = `Bearer ${newToken}`
              // 通知等待队列中的请求
              onRefreshed(newToken)
              isRefreshing = false
              return userRequest(originalRequest)
            }
          } catch (refreshError) {
            // 刷新失败
          }

          isRefreshing = false
          onRefreshed(null)

          // 刷新失败，清除token并跳转
          localStorage.removeItem('user_token')
          localStorage.removeItem('user_refresh_token')
          localStorage.removeItem('user_token_expires_at')
          localStorage.removeItem('user_info')
          // 判断当前是否在用户页面，跳转到首页
          if (window.location.pathname.startsWith('/user/') && !isUserTokenExpiredHandling) {
            isUserTokenExpiredHandling = true
            if (isPageInitialLoading) {
              window.location.href = '/'
            } else {
              ElMessage.warning('登录过期，请重新登录')
              setTimeout(() => {
                window.location.href = '/'
              }, 1500)
            }
          }
          const silentError = new Error(message)
          silentError.code = data.code
          silentError.silent = true
          return Promise.reject(silentError)
        }

        // 账号被封禁：清除用户token并跳转到首页
        const isBanned = message.includes('封禁') || message.includes('禁用')
        if (data.code === 403 || isBanned) {
          localStorage.removeItem('user_token')
          localStorage.removeItem('user_refresh_token')
          localStorage.removeItem('user_token_expires_at')
          localStorage.removeItem('user_info')
          // 判断当前是否在用户页面，跳转到首页（防止重复提示）
          if (window.location.pathname.startsWith('/user/') && !isUserTokenExpiredHandling) {
            isUserTokenExpiredHandling = true
            // 页面初始加载时token过期直接跳转，用户操作中过期才显示弹窗
            if (isPageInitialLoading) {
              // 页面刷新时token过期，直接静默跳转
              window.location.href = '/'
            } else {
              // 用户操作过程中token过期，显示弹窗后跳转
              ElMessage.warning(message)
              setTimeout(() => {
                window.location.href = '/'
              }, 1500)
            }
          }
          // 封禁/登录过期错误不再传播，避免组件重复显示消息
          const silentError = new Error(message)
          silentError.code = data.code
          silentError.silent = true // 标记为静默错误
          return Promise.reject(silentError)
        }

        // 非登录/封禁错误显示错误消息
        ElMessage.error(message)

        // 创建统一的错误对象
        const unifiedError = new Error(message)
        unifiedError.code = data.code
        unifiedError.response = response
        unifiedError.handled = true // 标记错误已在拦截器中显示过
        return Promise.reject(unifiedError)
      }
    }

    // 非统一响应格式，直接返回原始响应
    return response
  },
  async error => {
    // 错误处理逻辑（处理 HTTP 非2xx 状态码）
    // 注：后端返回 HTTP 200 + body code 401，所以 401 刷新逻辑在成功响应拦截器中处理
    let message = '请求失败'
    let code = 500

    if (error.response) {
      // 检查响应数据是否为统一格式
      if (error.response.data && typeof error.response.data === 'object' && 'code' in error.response.data && 'msg' in error.response.data) {
        // 统一响应格式的错误
        code = error.response.data.code
        message = error.response.data.msg
      } else {
        // 传统格式的错误处理
        code = error.response.status
        message = error.response.data?.error || error.response.data?.msg || `请求失败 (${error.response.status})`
      }
    } else if (error.code === 'ECONNABORTED') {
      message = '请求超时'
    } else if (error.code === 'ERR_NETWORK') {
      message = '网络连接失败，请检查网络或确认后端服务已启动'
    } else {
      message = '网络错误'
    }

    // 显示错误消息（非静默错误）
    if (!error.silent) {
      ElMessage.error(message)
    }

    // 创建统一的错误对象
    const unifiedError = new Error(message)
    unifiedError.code = code
    unifiedError.response = error.response
    unifiedError.handled = true // 标记错误已在拦截器中显示过
    return Promise.reject(unifiedError)
  }
)

// 用户代理相关API（需要用户身份验证）
export const proxyAPI = {
  // 提交代理地址
  submitProxy(data) {
    return userRequest({
      url: '/user/proxy/submit',
      method: 'post',
      data
    })
  },

  // 获取用户提交记录
  getUserSubmissions(params) {
    return userRequest({
      url: '/user/proxy/submissions',
      method: 'get',
      params
    })
  },

  // 检查提交限制
  checkSubmissionLimit() {
    return userRequest({
      url: '/user/proxy/check-limit',
      method: 'get'
    })
  }
}

// 管理员代理相关API（需要管理员身份验证）
export const adminProxyAPI = {
  // 获取代理列表
  getProxies(params) {
    return api.get('/proxies', { params })
  },

  // 创建代理
  createProxy(data) {
    return api.post('/proxies', data)
  },

  // 更新代理状态
  updateProxyStatus(id, data) {
    return api.put(`/proxies/${id}/status`, data)
  },

  // 审核通过代理
  approveProxy(id) {
    return api.post(`/proxies/${id}/approve`)
  },

  // 审核拒绝代理
  rejectProxy(id, data) {
    return api.post(`/proxies/${id}/reject`, data)
  },

  // 删除代理
  deleteProxy(id) {
    return api.delete(`/proxies/${id}`)
  }
}

// 共享账户相关API（需要用户身份验证）
export const sharedAccountAPI = {
  // 提交TOKEN账号
  submitToken(data) {
    return userRequest({
      url: '/user/shared-account/submit',
      method: 'post',
      data
    })
  },

  // 获取用户提交记录
  getUserSubmissions(params) {
    return userRequest({
      url: '/user/shared-account/submissions',
      method: 'get',
      params
    })
  },

  // 检查提交限制
  checkSubmissionLimit() {
    return userRequest({
      url: '/user/shared-account/check-limit',
      method: 'get'
    })
  },

  // 禁用TOKEN
  disableToken(tokenId) {
    return userRequest({
      url: `/user/shared-account/tokens/${tokenId}/disable`,
      method: 'post'
    })
  },

  // 更新TOKEN的代理地址
  updateProxyAddress(tokenId, proxyAddress) {
    return userRequest({
      url: `/user/shared-account/tokens/${tokenId}/update-proxy`,
      method: 'post',
      data: { proxy_address: proxyAddress }
    })
  }
}

// 为了向后兼容，导出单独的函数
export const submitSharedToken = sharedAccountAPI.submitToken
export const getSharedTokenSubmissions = sharedAccountAPI.getUserSubmissions
export const checkSharedTokenSubmissionLimit = sharedAccountAPI.checkSubmissionLimit
export const disableSharedToken = sharedAccountAPI.disableToken
export const updateSharedTokenProxy = sharedAccountAPI.updateProxyAddress

// 邀请码管理 API
export const invitationCodeAPI = {
  // 获取邀请码列表
  list(params = {}) {
    return api.get('/invitation-codes', { params })
  },

  // 生成邀请码
  generate(data) {
    return api.post('/invitation-codes/generate', data)
  },

  // 删除单个邀请码
  delete(id) {
    return api.delete(`/invitation-codes/${id}`)
  },

  // 验证邀请码（公开接口）
  validate(code) {
    return api.get('/invitation-codes/validate', { params: { code } })
  }
}

// 外部渠道管理 API（需要用户登录）
export const externalChannelAPI = {
  // 获取外部渠道列表
  getList() {
    return userRequest({
      url: '/user/external-channels',
      method: 'get'
    })
  },

  // 创建外部渠道
  create(data) {
    return userRequest({
      url: '/user/external-channels',
      method: 'post',
      data
    })
  },

  // 获取外部渠道详情
  getById(id) {
    return userRequest({
      url: `/user/external-channels/${id}`,
      method: 'get'
    })
  },

  // 更新外部渠道
  update(id, data) {
    return userRequest({
      url: `/user/external-channels/${id}`,
      method: 'put',
      data
    })
  },

  // 删除外部渠道
  delete(id) {
    return userRequest({
      url: `/user/external-channels/${id}`,
      method: 'delete'
    })
  },

  // 获取内部模型列表
  getInternalModels() {
    return userRequest({
      url: '/user/external-channels/internal-models',
      method: 'get'
    })
  },

  // 测试外部渠道连通性
  test(id, model) {
    return userRequest({
      url: `/user/external-channels/${id}/test`,
      method: 'post',
      data: { model },
      timeout: 60000 // 测试接口可能需要较长时间，设置60秒超时
    })
  },

  // 获取外部渠道使用统计
  getUsageStats(id, days = 7) {
    return userRequest({
      url: `/user/external-channels/${id}/usage-stats`,
      method: 'get',
      params: { days }
    })
  },

  // 获取外部渠道可用模型列表
  // 创建模式传递 api_endpoint + api_key
  // 编辑模式传递 api_endpoint + channel_id（后端查询 API Key）
  fetchAvailableModels(apiEndpoint, apiKey = null, channelId = null) {
    const data = { api_endpoint: apiEndpoint }
    if (channelId) {
      data.channel_id = channelId
    } else if (apiKey) {
      data.api_key = apiKey
    }
    return userRequest({
      url: '/user/external-channels/fetch-models',
      method: 'post',
      data,
      timeout: 30000 // 获取模型列表可能需要较长时间
    })
  }
}

// 用户TOKEN管理和统计 API（需要用户登录）
export const userTokenAPI = {
  // 提交TOKEN账号
  submitToken(data) {
    return userRequest({
      url: '/user/tokens/submit',
      method: 'post',
      data
    })
  },

  // 获取用户的TOKEN分配列表
  getTokenAllocations(params = {}) {
    return userRequest({
      url: '/user/token-allocations',
      method: 'get',
      params
    })
  },

  // 获取用户使用统计（按日）
  getUsageStats(params = {}) {
    return userRequest({
      url: '/user/usage-stats',
      method: 'get',
      params
    })
  },

  // 获取用户统计概览
  getUsageStatsOverview() {
    return userRequest({
      url: '/user/usage-stats/overview',
      method: 'get'
    })
  },

  // 获取用户TOKEN账号统计
  getTokenAccountStats() {
    return userRequest({
      url: '/user/token-account-stats',
      method: 'get'
    })
  },

  // 禁用用户的TOKEN账号
  disableToken(tokenId) {
    return userRequest({
      url: `/user/tokens/${tokenId}/disable`,
      method: 'post'
    })
  },

  // 删除用户的TOKEN账号（仅自有账号可删除）
  deleteToken(tokenId) {
    return userRequest({
      url: `/user/tokens/${tokenId}`,
      method: 'delete'
    })
  },

  // 切换当前使用的TOKEN账号
  switchToken(tokenId) {
    return userRequest({
      url: `/user/tokens/${tokenId}/switch`,
      method: 'post'
    })
  },

  // 获取可切换的TOKEN列表
  getAvailableTokensForSwitch() {
    return userRequest({
      url: '/user/tokens/available-for-switch',
      method: 'get'
    })
  },

  // 增强TOKEN（绑定外部渠道）
  enhanceToken(tokenId, channelId) {
    return userRequest({
      url: `/user/tokens/${tokenId}/enhance`,
      method: 'post',
      data: { channel_id: channelId }
    })
  },

  // 解除TOKEN增强绑定
  removeTokenEnhance(tokenId) {
    return userRequest({
      url: `/user/tokens/${tokenId}/enhance`,
      method: 'delete'
    })
  },

  // 获取TOKEN增强信息
  getTokenEnhanceInfo(tokenId) {
    return userRequest({
      url: `/user/tokens/${tokenId}/enhance`,
      method: 'get'
    })
  }
}

// 插件下载 API（需要用户登录）
export const pluginAPI = {
  // 获取插件列表
  getList(params = {}) {
    return userRequest({
      url: '/user/plugins',
      method: 'get',
      params
    })
  },

  // 获取插件下载信息
  getDownloadInfo(id) {
    return userRequest({
      url: `/user/plugins/${id}/download`,
      method: 'get'
    })
  }
}

// 系统信息 API（公开接口）
export const systemAPI = {
  // 获取系统版本号
  getVersion() {
    return api.get('/system/version')
  }
}

// 系统配置管理 API（管理后台）
export const systemConfigAPI = {
  // 获取系统配置
  get() {
    return api.get('/system-config')
  },

  // 更新系统配置
  update(data) {
    return api.put('/system-config', data)
  },

  // 获取系统统计信息
  stats() {
    return api.get('/system-config/stats')
  }
}

// 系统公告管理 API（管理后台）
export const systemAnnouncementAPI = {
  // 获取公告列表
  getList() {
    return api.get('/system-announcements')
  },

  // 创建公告
  create(data) {
    return api.post('/system-announcements', data)
  },

  // 获取公告详情
  get(id) {
    return api.get(`/system-announcements/${id}`)
  },

  // 更新公告
  update(id, data) {
    return api.put(`/system-announcements/${id}`, data)
  },

  // 删除公告
  delete(id) {
    return api.delete(`/system-announcements/${id}`)
  },

  // 发布公告
  publish(id) {
    return api.post(`/system-announcements/${id}/publish`)
  },

  // 取消公告
  cancel(id) {
    return api.post(`/system-announcements/${id}/cancel`)
  },

  // 获取已发布的公告（公开接口，不需要登录）
  getPublished() {
    return api.get('/system-announcements/published')
  },

  // 获取已发布的公告（用户端，包含未读状态）
  getPublishedWithUnread() {
    return userRequest({
      url: '/user/announcements',
      method: 'get'
    })
  },

  // 标记公告为已读
  markAsRead() {
    return userRequest({
      url: '/user/announcements/mark-read',
      method: 'put'
    })
  }
}

// 渠道监测 API（需要用户登录）
export const monitorAPI = {
  // 获取监测配置列表
  getConfigs(params) {
    return userRequest({
      url: '/user/monitor/configs',
      method: 'get',
      params
    })
  },

  // 创建监测配置
  createConfig(data) {
    return userRequest({
      url: '/user/monitor/configs',
      method: 'post',
      data
    })
  },

  // 获取监测配置详情
  getConfigDetail(id) {
    return userRequest({
      url: `/user/monitor/configs/${id}`,
      method: 'get'
    })
  },

  // 更新监测配置
  updateConfig(id, data) {
    return userRequest({
      url: `/user/monitor/configs/${id}`,
      method: 'put',
      data
    })
  },

  // 启用/禁用监测配置
  toggleConfigStatus(id, status) {
    return userRequest({
      url: `/user/monitor/configs/${id}/status`,
      method: 'patch',
      data: { status }
    })
  },

  // 删除监测配置
  deleteConfig(id) {
    return userRequest({
      url: `/user/monitor/configs/${id}`,
      method: 'delete'
    })
  },

  // 主动触发监测
  triggerCheck(id) {
    return userRequest({
      url: `/user/monitor/configs/${id}/trigger`,
      method: 'post',
      timeout: 60000
    })
  },

  // 获取渠道可用模型列表
  getChannelModels(channelId) {
    return userRequest({
      url: `/user/monitor/channels/${channelId}/models`,
      method: 'get',
      timeout: 30000
    })
  }
}

// 远程模型管理 API（管理后台）
export const remoteModelAPI = {
  // 获取远程模型列表
  getList() {
    return api.get('/remote-models')
  },

  // 手动触发同步
  sync() {
    return api.post('/remote-models/sync')
  },

  // 更新共享账号透传配置
  updatePassthrough(id, data) {
    return api.put(`/remote-models/${id}/passthrough`, data)
  },

  // 设置默认模型
  setDefault(id) {
    return api.post(`/remote-models/${id}/set-default`)
  },

  // 删除远程模型
  delete(id) {
    return api.delete(`/remote-models/${id}`)
  }
}

export default api
