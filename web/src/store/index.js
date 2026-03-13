import { defineStore } from 'pinia'
import { tokenAPI, statsAPI, authAPI } from '@/api'

// 主应用store
export const useAppStore = defineStore('app', {
  state: () => ({
    // 主题
    isDark: false,
    // 侧边栏折叠状态
    sidebarCollapsed: false,
    // 加载状态
    loading: false
  }),
  
  actions: {
    toggleTheme() {
      this.isDark = !this.isDark
      document.documentElement.classList.toggle('dark', this.isDark)
      localStorage.setItem('theme', this.isDark ? 'dark' : 'light')
    },
    
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
      localStorage.setItem('sidebarCollapsed', this.sidebarCollapsed)
    },
    
    initTheme() {
      const savedTheme = localStorage.getItem('theme')
      if (savedTheme) {
        this.isDark = savedTheme === 'dark'
      } else {
        this.isDark = window.matchMedia('(prefers-color-scheme: dark)').matches
      }
      document.documentElement.classList.toggle('dark', this.isDark)
    },
    
    initSidebar() {
      const collapsed = localStorage.getItem('sidebarCollapsed')
      if (collapsed !== null) {
        this.sidebarCollapsed = JSON.parse(collapsed)
      }
    }
  }
})

// Token管理store
export const useTokenStore = defineStore('token', {
  state: () => ({
    tokens: [],
    currentToken: null,
    pagination: {
      page: 1,
      pageSize: 20,
      total: 0
    },
    loading: false
  }),
  
  getters: {
    activeTokens: (state) => state.tokens.filter(token => token.status === 'active'),
    expiredTokens: (state) => state.tokens.filter(token => token.status === 'expired'),
    disabledTokens: (state) => state.tokens.filter(token => token.status === 'disabled'),
    inactiveTokens: (state) => state.tokens.filter(token => token.status === 'disabled')
  },
  
  actions: {
    async fetchTokens(params = {}) {
      this.loading = true
      try {
        const response = await tokenAPI.list({
          page: this.pagination.page,
          page_size: this.pagination.pageSize,
          ...params
        })
        if (response && typeof response === 'object') {
          this.tokens = response.data || []
          if (response.pagination) {
            this.pagination = { ...this.pagination, ...response.pagination }
          }
        } else {
          this.tokens = []
        }
      } catch (error) {
        console.error('获取token列表失败:', error)
        this.tokens = []
      } finally {
        this.loading = false
      }
    },
    
    async createToken(data) {
      try {
        const response = await tokenAPI.create(data)
        await this.fetchTokens()
        return response
      } catch (error) {
        throw error
      }
    },
    
    async updateToken(id, data) {
      try {
        const response = await tokenAPI.update(id, data)
        await this.fetchTokens()
        return response
      } catch (error) {
        throw error
      }
    },
    
    async deleteToken(id) {
      try {
        await tokenAPI.delete(id)
        await this.fetchTokens()
      } catch (error) {
        throw error
      }
    }
  }
})

// 认证store
export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('token') || null,
    user: null,
    loading: false
  }),
  
  getters: {
    isLoggedIn: (state) => !!state.token,
    isAdmin: (state) => state.user?.role === 'admin'
  },
  
  actions: {
    async login(credentials) {
      this.loading = true
      try {
        const response = await authAPI.login(credentials)
        const { token, user } = response || {}

        if (!token) {
          throw new Error('登录响应数据格式错误')
        }

        this.token = token
        this.user = user

        // 保存到localStorage
        localStorage.setItem('token', token)

        return response
      } catch (error) {
        this.clearAuth()
        throw error
      } finally {
        this.loading = false
      }
    },
    
    async logout() {
      try {
        await authAPI.logout()
      } catch (error) {
        console.warn('登出请求失败:', error)
      } finally {
        this.clearAuth()
      }
    },
    
    async getCurrentUser() {
      if (!this.token) return null

      try {
        const response = await authAPI.me()
        this.user = response || null
        return response
      } catch (error) {
        // token可能已过期
        this.clearAuth()
        throw error
      }
    },
    
    clearAuth() {
      this.token = null
      this.user = null
      localStorage.removeItem('token')
    },
    
    // 初始化认证状态
    initAuth() {
      if (this.token) {
        // token已经在localStorage中，axios拦截器会自动添加到请求头
        // 验证token是否仍然有效
        this.getCurrentUser().catch(() => {
          this.clearAuth()
        })
      }
    }
  }
})

// 统计数据store
export const useStatsStore = defineStore('stats', {
  state: () => ({
    overview: {
      total_requests: 0,
      active_tokens: 0,
      expired_tokens: 0,
      disabled_tokens: 0,
      success_rate: 0,
      total_tokens: 0,
      success_requests: 0,
      error_requests: 0,
      total_bytes: 0
    },
    tokenStats: {},
    usageHistory: [],
    topTokens: [],
    trendData: [],
    loading: false
  }),
  
  actions: {
    async fetchOverview() {
      this.loading = true
      try {
        const response = await statsAPI.overview()

        // API拦截器已经处理了统一响应格式，直接使用返回的数据
        if (response && typeof response === 'object') {
          // 直接使用API返回的数据，包含所有字段
          this.overview = {
            total_requests: response.total_requests ?? 0,
            active_tokens: response.active_tokens ?? 0,
            expired_tokens: response.expired_tokens ?? 0,
            disabled_tokens: response.disabled_tokens ?? 0,
            success_rate: response.success_rate ?? 0,
            total_tokens: response.total_tokens ?? 0,
            success_requests: response.success_requests ?? 0,
            error_requests: response.error_requests ?? 0,
            total_bytes: response.total_bytes ?? 0
          }
        }
      } catch (error) {
        console.error('获取概览统计失败:', error)
        // 出错时保持当前数据或使用默认数据
        if (!this.overview || Object.keys(this.overview).length === 0) {
          this.overview = {
            total_requests: 0,
            active_tokens: 0,
            expired_tokens: 0,
            disabled_tokens: 0,
            success_rate: 0,
            total_tokens: 0,
            success_requests: 0,
            error_requests: 0,
            total_bytes: 0
          }
        }
      } finally {
        this.loading = false
      }
    },

    async fetchTokenStats(id) {
      try {
        const response = await statsAPI.tokenStats(id)
        // API拦截器已经处理了统一响应格式，直接使用返回的数据
        this.tokenStats[id] = response
        return response
      } catch (error) {
        console.error('获取token统计失败:', error)
        throw error
      }
    },
    
    async fetchUsageHistory(params = {}) {
      try {
        const response = await statsAPI.usage(params)
        // API拦截器已经处理了统一响应格式，直接使用返回的数据
        // 确保返回的数据是数组格式
        const data = Array.isArray(response) ? response : []
        this.usageHistory = data
        return data
      } catch (error) {
        console.error('获取使用历史失败:', error)
        // 出错时返回空数组
        this.usageHistory = []
        return []
      }
    },

    async fetchTopTokens(limit = 10) {
      try {
        const response = await statsAPI.usage({ limit })
        // API拦截器已经处理了统一响应格式，直接使用返回的数据
        this.topTokens = response || []
        return response
      } catch (error) {
        console.error('获取热门token失败:', error)
        throw error
      }
    },

    async fetchTrendData(days = 7) {
      try {
        const response = await statsAPI.trend({ days })
        // API拦截器已经处理了统一响应格式，直接使用返回的数据
        this.trendData = response || []
        return response
      } catch (error) {
        console.error('获取趋势数据失败:', error)
        throw error
      }
    }
  }
})
