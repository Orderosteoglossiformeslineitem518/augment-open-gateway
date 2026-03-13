<template>
  <div class="dashboard-page">
    <!-- 全局加载遮罩 -->
    <div v-if="initialLoading" class="global-loading-overlay">
      <div class="loading-content">
        <el-icon class="loading-icon"><Loading /></el-icon>
        <p>正在加载用户信息...</p>
      </div>
    </div>

    <!-- 顶部导航栏 -->
    <header class="page-header">
      <div class="header-left">
        <img src="/logo.svg" alt="AugmentGateway" class="header-logo" @click="handleLogoClick" />
        <span class="header-title">AugmentGateway</span>
        <span v-if="systemVersion" class="header-version">{{ systemVersion }}</span>
      </div>
      <div class="header-right">
        <!-- 主题切换按钮 -->
        <el-tooltip :content="themeTooltip" placement="bottom" :show-after="300">
          <button class="theme-toggle-btn" @click="cycleThemeMode">
            <el-icon><Sunny v-if="themeMode === 'light'" /><Moon v-if="themeMode === 'dark'" /><Monitor v-if="themeMode === 'system'" /></el-icon>
          </button>
        </el-tooltip>
        <!-- 公告按钮 -->
        <button class="notification-btn" @click="openNotification" title="公告">
          <el-icon><Bell /></el-icon>
          <span v-if="hasUnreadAnnouncements" class="notification-dot"></span>
        </button>
        <div class="user-info-dropdown">
          <el-dropdown trigger="click" @command="handleUserCommand">
            <div class="user-trigger">
              <el-icon class="user-icon"><User /></el-icon>
              <span class="user-name">{{ userInfo?.name || userInfo?.username || '用户' }}</span>
              <el-icon class="dropdown-arrow"><ArrowDown /></el-icon>
            </div>
            <template #dropdown>
              <el-dropdown-menu class="user-dropdown-menu">
                <el-dropdown-item command="logout" class="logout-item">
                  <el-icon><SwitchButton /></el-icon>
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
    </header>

    <!-- 主体内容 -->
    <div class="dashboard-container">
      <!-- 左侧菜单栏 -->
      <aside :class="['sidebar', { collapsed: sidebarCollapsed }]">
        <nav class="sidebar-nav">
          <div
            v-for="item in menuItems"
            :key="item.key"
            :class="['nav-item', { active: activeMenu === item.key }]"
            @click="activeMenu = item.key"
            :title="sidebarCollapsed ? item.label : ''"
          >
            <el-icon><component :is="item.icon" /></el-icon>
            <span class="nav-label">{{ item.label }}</span>
          </div>
        </nav>
        <div class="sidebar-toggle" @click="sidebarCollapsed = !sidebarCollapsed">
          <el-icon>
            <Fold v-if="!sidebarCollapsed" />
            <Expand v-else />
          </el-icon>
        </div>
      </aside>

      <!-- 右侧内容区 -->
      <main :class="['main-content', { 'sidebar-collapsed': sidebarCollapsed }]">
        <!-- 数据面板 -->
        <DataPanel
          v-if="activeMenu === 'data'"
          ref="dataPanelRef"
          :token-account-stats="tokenAccountStats"
          :user-info="userInfo"
          :user-token="userToken"
          :api-url="apiUrl"
          :usage-stats="usageStats"
          :usage-stats-loading="usageStatsLoading"
          v-model:usage-stats-days="usageStatsDays"
          :is-maintenance-mode="isMaintenanceMode"
          :maintenance-message="maintenanceMessage"
          :no-token-message="noTokenMessage"
          @fetch-usage-stats="fetchUsageStats"
        />

        <!-- 账号面板 -->
        <AccountPanel
          v-if="activeMenu === 'accounts'"
          :token-allocations="tokenAllocations"
          :token-list-loading="tokenListLoading"
          v-model:page="tokenAllocationPage"
          v-model:page-size="tokenAllocationPageSize"
          :total="tokenAllocationTotal"
          v-model:search-query="tokenSearchQuery"
          v-model:status-filter="tokenStatusFilter"
          v-model:token-type-filter="tokenTypeFilter"
          @fetch-token-allocations="fetchTokenAllocations"
          @fetch-token-account-stats="fetchTokenAccountStats"
        />

        <!-- 外部渠道面板 -->
        <ExternalChannelPanel
          v-if="activeMenu === 'channels'"
          :external-channels="externalChannels"
          :loading="externalChannelsLoading"
          v-model:page="externalChannelsPage"
          v-model:page-size="externalChannelsPageSize"
          v-model:search-keyword="channelSearchKeyword"
          :internal-models="internalModels"
          @fetch-external-channels="fetchExternalChannels"
        />

        <!-- 插件下载面板 -->
        <PluginDownloadPanel
          v-if="activeMenu === 'plugins'"
          :plugins="plugins"
          :loading="pluginsLoading"
          v-model:page="pluginsPage"
          v-model:page-size="pluginsPageSize"
          :total="pluginsTotal"
          v-model:version-keyword="pluginVersionKeyword"
          @fetch-plugins="fetchPlugins"
        />

        <!-- 定时监测面板 -->
        <MonitorPanel
          v-if="activeMenu === 'monitor'"
        />

        <!-- 个人设置 -->
        <PersonalSettingsPanel
          v-if="activeMenu === 'settings'"
          :user-info="userInfo"
          :user-token="userToken"
          :token-account-stats="tokenAccountStats"
        />
      </main>
    </div>

    <!-- 页面底部版权信息 -->
    <footer class="page-footer">
      <p>© 2026 Augment Gateway. All rights reserved.</p>
    </footer>

    <!-- 公告弹窗 -->
    <el-dialog
      v-model="showNotification"
      title="公告"
      width="500px"
      class="notification-modal"
    >
      <div class="notification-content">
        <p v-if="notifications.length === 0" class="no-notification">暂无公告</p>
        <div v-else class="notification-list">
          <div v-for="item in notifications" :key="item.id" class="notification-item">
            <h4>{{ item.title }}</h4>
            <p>{{ item.content }}</p>
            <span class="notification-time">{{ item.created_at }}</span>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 使用教程引导弹窗 -->
    <el-dialog
      v-model="showTutorialPopup"
      :show-close="true"
      :close-on-click-modal="true"
      :close-on-press-escape="true"
      width="400px"
      class="tutorial-modal"
    >
      <div class="tutorial-content">
        <h3 class="tutorial-title">欢迎使用 AGC</h3>
        <div class="tutorial-hints">
          <p class="tutorial-hint-item">
            <el-icon><Reading /></el-icon>
            首次使用请先查看教程了解使用方式
          </p>
          <p class="tutorial-hint-item">
            <el-icon><Bell /></el-icon>
            点击顶部公告按钮可以查看近期更新日志
          </p>
        </div>
        <div class="tutorial-actions">
          <el-button class="tutorial-btn secondary" @click="openTutorial">
            查看使用教程
          </el-button>
          <el-button type="primary" class="tutorial-btn primary" @click="hideTutorialToday">
            今日不再显示
          </el-button>
        </div>
      </div>
    </el-dialog>

    <!-- 烟花彩蛋 -->
    <FireworksCanvas v-if="showFireworks" :active="showFireworks" @ended="onFireworksEnded" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Loading,
  SwitchButton,
  User,
  ArrowDown,
  Setting,
  DataAnalysis,
  Wallet,
  Fold,
  Expand,
  Link,
  Download,
  Bell,
  Sunny,
  Moon,
  Monitor,
  Reading,
  Odometer
} from '@element-plus/icons-vue'
import { userAuthAPI, userTokenAPI, externalChannelAPI, systemAnnouncementAPI, systemAPI, pluginAPI } from '@/api'
import { DataPanel, AccountPanel, ExternalChannelPanel, PluginDownloadPanel, PersonalSettingsPanel, MonitorPanel } from '@/components/dashboard'
import FireworksCanvas from '@/components/dashboard/FireworksCanvas.vue'

const router = useRouter()
const route = useRoute()

// 菜单配置
const menuItems = [
  { key: 'data', label: '数据面板', icon: DataAnalysis },
  { key: 'accounts', label: '账号面板', icon: Wallet },
  { key: 'channels', label: '外部渠道', icon: Link },
  { key: 'plugins', label: '插件下载', icon: Download },
  { key: 'monitor', label: '定时监测', icon: Odometer },
  { key: 'settings', label: '个人设置', icon: Setting }
]

// 响应式数据
const activeMenu = ref(localStorage.getItem('user_active_menu') || 'data')
const sidebarCollapsed = ref(false)
const loading = ref(false)
const initialLoading = ref(false)
const refreshDisabled = ref(false)
const userInfo = ref(null)
const userToken = ref(null)
const noTokenMessage = ref('')
const apiUrl = ref(window.location.origin + '/proxy/')
const isMaintenanceMode = ref(false)
const maintenanceMessage = ref('')

// 数据面板引用
const dataPanelRef = ref(null)

// TOKEN分配列表
const tokenAllocations = ref([])
const tokenAllocationPage = ref(1)
const tokenAllocationPageSize = ref(10)
const tokenAllocationTotal = ref(0)
const tokenListLoading = ref(false)
const tokenSearchQuery = ref('')
const tokenStatusFilter = ref('')
const tokenTypeFilter = ref('')

// TOKEN账号统计
const tokenAccountStats = ref({ total_count: 0, available_count: 0, disabled_count: 0, expired_count: 0 })

// 外部渠道相关
const externalChannels = ref([])
const externalChannelsLoading = ref(false)
const externalChannelsPage = ref(1)
const externalChannelsPageSize = ref(10)
const channelSearchKeyword = ref('')
const internalModels = ref([])

// 使用统计
const usageStatsDays = ref(7)
const usageStats = ref([])
const usageStatsLoading = ref(false)

// 插件下载相关
const plugins = ref([])
const pluginsLoading = ref(false)
const pluginsPage = ref(1)
const pluginsPageSize = ref(10)
const pluginsTotal = ref(0)
const pluginVersionKeyword = ref('')

// 公告相关
const showNotification = ref(false)
const notifications = ref([])
const hasUnreadAnnouncements = ref(false)

// 使用教程引导弹窗相关
const showTutorialPopup = ref(false)
const TUTORIAL_URL = ''

// 获取今日日期字符串 (YYYY-MM-DD)
const getTodayDateString = () => {
  const today = new Date()
  const year = today.getFullYear()
  const month = String(today.getMonth() + 1).padStart(2, '0')
  const day = String(today.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

// 检查今日是否已选择不显示教程弹窗
const shouldShowTutorialPopup = () => {
  const hiddenDate = localStorage.getItem('tutorial_popup_hidden')
  return hiddenDate !== getTodayDateString()
}

// 打开使用教程
const openTutorial = () => {
  window.open(TUTORIAL_URL, '_blank')
  showTutorialPopup.value = false
}

// 今日不再显示
const hideTutorialToday = () => {
  localStorage.setItem('tutorial_popup_hidden', getTodayDateString())
  showTutorialPopup.value = false
}

// 烟花彩蛋相关
const showFireworks = ref(false)
let logoClickCount = 0
let logoClickTimer = null

// 处理 Logo 点击
const handleLogoClick = () => {
  // 如果烟花正在播放，点击则关闭
  if (showFireworks.value) {
    showFireworks.value = false
    return
  }

  logoClickCount++

  // 清除之前的计时器
  if (logoClickTimer) clearTimeout(logoClickTimer)

  // 2秒内点击3次触发烟花
  if (logoClickCount >= 3) {
    showFireworks.value = true
    logoClickCount = 0
    return
  }

  // 2秒后重置计数
  logoClickTimer = setTimeout(() => {
    logoClickCount = 0
  }, 2000)
}

// 烟花结束回调
const onFireworksEnded = () => {
  showFireworks.value = false
}

// 主题模式: 'light' | 'dark' | 'system'
const themeMode = ref(localStorage.getItem('themeMode') || 'system')
const isDarkMode = ref(false)

// 系统版本号
const systemVersion = ref('')

// 主题切换提示文字
const themeTooltip = computed(() => {
  if (themeMode.value === 'light') return '当前：亮色模式'
  if (themeMode.value === 'dark') return '当前：暗色模式'
  return '当前：跟随系统'
})

// 应用主题（带过渡动画）
const applyTheme = (animate = false) => {
  let shouldBeDark = false
  if (themeMode.value === 'dark') {
    shouldBeDark = true
  } else if (themeMode.value === 'system') {
    shouldBeDark = window.matchMedia('(prefers-color-scheme: dark)').matches
  }
  isDarkMode.value = shouldBeDark

  // 仅在用户主动切换时启用过渡动画，避免首屏加载闪烁
  if (animate) {
    document.documentElement.classList.add('theme-transitioning')
  }
  document.documentElement.classList.toggle('dark-mode', shouldBeDark)
  if (animate) {
    // 过渡完成后移除类，避免常驻 transition 影响滚动性能
    setTimeout(() => {
      document.documentElement.classList.remove('theme-transitioning')
    }, 350)
  }
  // 兼容旧的 darkMode 存储
  localStorage.setItem('darkMode', shouldBeDark ? 'true' : 'false')
}

// 循环切换主题: light → dark → system → light
const cycleThemeMode = () => {
  const modes = ['light', 'dark', 'system']
  const currentIndex = modes.indexOf(themeMode.value)
  themeMode.value = modes[(currentIndex + 1) % modes.length]
  localStorage.setItem('themeMode', themeMode.value)
  applyTheme(true) // 用户主动切换，启用过渡动画
}

// 监听系统主题变化（仅 system 模式下生效）
let systemThemeMediaQuery = null
const handleSystemThemeChange = () => {
  if (themeMode.value === 'system') {
    applyTheme()
  }
}

// 处理用户下拉菜单命令
const handleUserCommand = (command) => {
  if (command === 'refresh') {
    refreshData()
  } else if (command === 'logout') {
    logout()
  }
}

// 获取TOKEN分配列表
const fetchTokenAllocations = async () => {
  tokenListLoading.value = true
  try {
    const data = await userTokenAPI.getTokenAllocations({
      page: tokenAllocationPage.value,
      page_size: tokenAllocationPageSize.value,
      search: tokenSearchQuery.value,
      status: tokenStatusFilter.value,
      token_type: tokenTypeFilter.value
    })
    tokenAllocations.value = data?.list || []
    tokenAllocationTotal.value = data?.total || 0
  } catch (error) {
    console.error('获取TOKEN分配列表失败:', error)
    tokenAllocations.value = []
  } finally {
    tokenListLoading.value = false
  }
}

// 获取使用统计
const fetchUsageStats = async () => {
  usageStatsLoading.value = true
  try {
    const data = await userTokenAPI.getUsageStats({ days: usageStatsDays.value })
    usageStats.value = data?.stats || []
  } catch (error) {
    console.error('获取使用统计失败:', error)
    usageStats.value = []
  } finally {
    usageStatsLoading.value = false
  }
}

// 获取TOKEN账号统计
const fetchTokenAccountStats = async () => {
  try {
    const data = await userTokenAPI.getTokenAccountStats()
    tokenAccountStats.value = data || { total_count: 0, available_count: 0, disabled_count: 0, expired_count: 0 }
  } catch (error) {
    console.error('获取TOKEN账号统计失败:', error)
    tokenAccountStats.value = { total_count: 0, available_count: 0, disabled_count: 0, expired_count: 0 }
  }
}

// 获取外部渠道列表
const fetchExternalChannels = async () => {
  externalChannelsLoading.value = true
  try {
    const data = await externalChannelAPI.getList()
    externalChannels.value = data?.list || []
    internalModels.value = data?.internal_models || []
  } catch (error) {
    console.error('获取外部渠道列表失败:', error)
    externalChannels.value = []
  } finally {
    externalChannelsLoading.value = false
  }
}

// 获取系统公告
const fetchNotifications = async () => {
  try {
    // 使用带未读状态的接口
    const res = await systemAnnouncementAPI.getPublishedWithUnread()
    notifications.value = res.announcements || []
    hasUnreadAnnouncements.value = res.has_unread || false
  } catch (e) {
    // 如果用户未登录或请求失败，降级使用公开接口
    try {
      const res = await systemAnnouncementAPI.getPublished()
      notifications.value = res.announcements || []
      hasUnreadAnnouncements.value = false
    } catch {
      notifications.value = []
      hasUnreadAnnouncements.value = false
    }
  }
}

// 打开公告弹窗并标记已读
const openNotification = async () => {
  showNotification.value = true
  // 如果有未读公告，标记为已读
  if (hasUnreadAnnouncements.value) {
    try {
      await systemAnnouncementAPI.markAsRead()
      hasUnreadAnnouncements.value = false
    } catch (e) {
      console.error('标记公告已读失败:', e)
    }
  }
}

// 获取插件列表
const fetchPlugins = async () => {
  pluginsLoading.value = true
  try {
    const data = await pluginAPI.getList({
      page: pluginsPage.value,
      page_size: pluginsPageSize.value,
      version: pluginVersionKeyword.value
    })
    plugins.value = data?.list || []
    pluginsTotal.value = data?.total || 0
  } catch (error) {
    console.error('获取插件列表失败:', error)
    plugins.value = []
    pluginsTotal.value = 0
  } finally {
    pluginsLoading.value = false
  }
}

// 获取系统版本号
const fetchSystemVersion = async () => {
  try {
    const res = await systemAPI.getVersion()
    systemVersion.value = res?.version || ''
  } catch (e) {
    systemVersion.value = ''
  }
}

// 刷新数据
const refreshData = async (showSuccessMessage = true) => {
  if (loading.value || refreshDisabled.value) return

  if (!userInfo.value?.id) {
    ElMessage.warning('无法获取用户ID，请重新登录')
    return
  }

  try {
    loading.value = true
    refreshDisabled.value = true

    const response = await userAuthAPI.me()

    if (response) {
      userInfo.value = response
      localStorage.setItem('user_info', JSON.stringify(response))

      if (response.token_status === 'active' && response.api_token) {
        userToken.value = {
          token: response.api_token,
          status: response.token_status,
          max_requests: response.max_requests,
          used_requests: response.used_requests,
          rate_limit_per_minute: response.rate_limit_per_minute
        }
        noTokenMessage.value = ''
        isMaintenanceMode.value = false
        maintenanceMessage.value = ''
      } else {
        userToken.value = null
        noTokenMessage.value = '您当前没有可用的API令牌，请联系管理员分配。'
      }
    }

    if (showSuccessMessage) {
      ElMessage.success('数据刷新成功')
    }
  } catch (error) {
    console.error('刷新数据失败:', error)
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      const message = error.response?.data?.msg || error.message || '数据刷新失败，请稍后重试'
      ElMessage.error(message)
    }
  } finally {
    loading.value = false
    setTimeout(() => { refreshDisabled.value = false }, 3000)
  }
}

// 退出登录
const logout = async () => {
  try {
    await ElMessageBox.confirm('确定要退出登录吗？', '确认退出', {
      confirmButtonText: '退出',
      cancelButtonText: '取消',
      type: 'warning',
      customClass: 'logout-confirm-dialog',
      confirmButtonClass: 'logout-confirm-btn',
      cancelButtonClass: 'logout-cancel-btn'
    })

    localStorage.removeItem('user_info')
    localStorage.removeItem('user_token')
    localStorage.removeItem('user_refresh_token')
    localStorage.removeItem('user_token_expires_at')
    localStorage.removeItem('user_active_menu')

    router.push('/')
    ElMessage.success('已退出登录')
  } catch (error) {
    // 用户取消
  }
}

// 处理OAuth回调
const handleOAuthCallback = async () => {
  const loginStatus = route.query.login
  const message = decodeURIComponent(route.query.message || '')

  if (!loginStatus) return

  try {
    initialLoading.value = true

    if (loginStatus === 'success') {
      const userParam = route.query.user

      if (userParam) {
        try {
          userInfo.value = JSON.parse(decodeURIComponent(userParam))
          localStorage.setItem('user_info', JSON.stringify(userInfo.value))
          await refreshData(false)
        } catch (parseError) {
          console.error('解析用户信息失败:', parseError)
          if (!parseError.handled) {
            ElMessage.error(parseError.message || '解析用户信息失败')
          }
        }
      }
    } else {
      ElMessage.error(message || '登录失败')
      router.push('/')
    }
  } catch (error) {
    console.error('处理登录回调失败:', error)
    if (!error.handled) {
      ElMessage.error(error.message || '处理登录回调失败')
    }
  } finally {
    initialLoading.value = false
    router.replace({ query: {} })
  }
}

// 初始化数据加载
const loadInitialData = () => {
  const storedUser = localStorage.getItem('user_info')

  if (storedUser) {
    try {
      userInfo.value = JSON.parse(storedUser)
    } catch (e) {
      console.error('解析用户信息失败:', e)
      userInfo.value = null
    }
  }

  if (userInfo.value?.id) {
    refreshData(false)
  } else {
    noTokenMessage.value = '您当前没有可用的令牌，请联系管理员处理！'
  }
}

// 监听菜单切换，保存到localStorage并重新初始化图表
watch(activeMenu, (newVal) => {
  localStorage.setItem('user_active_menu', newVal)

  if (newVal === 'data') {
    nextTick(() => {
      if (usageStats.value.length > 0 && dataPanelRef.value) {
        dataPanelRef.value.initChart()
      }
    })
  }

  // 切换到外部渠道页面时，重新加载数据以获取最新的is_bound状态
  if (newVal === 'channels') {
    fetchExternalChannels()
  }
})

// 组件挂载时加载数据
onMounted(() => {
  // 初始化主题模式
  // 迁移旧的 darkMode 设置到新的 themeMode
  if (!localStorage.getItem('themeMode') && localStorage.getItem('darkMode') === 'true') {
    themeMode.value = 'dark'
    localStorage.setItem('themeMode', 'dark')
  }
  applyTheme()

  // 监听系统主题变化
  systemThemeMediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  systemThemeMediaQuery.addEventListener('change', handleSystemThemeChange)

  if (route.query.login) {
    handleOAuthCallback()
  } else {
    loadInitialData()
  }

  fetchTokenAllocations()
  fetchUsageStats()
  fetchTokenAccountStats()
  fetchExternalChannels()
  fetchPlugins()
  fetchNotifications()
  fetchSystemVersion()

  // 检查是否需要显示使用教程引导弹窗
  if (shouldShowTutorialPopup()) {
    // 延迟显示，确保页面加载完成
    setTimeout(() => {
      showTutorialPopup.value = true
    }, 500)
  }
})

// 组件卸载时移除暗黑模式类和系统主题监听，避免影响管理后台
onUnmounted(() => {
  document.documentElement.classList.remove('dark-mode')
  if (systemThemeMediaQuery) {
    systemThemeMediaQuery.removeEventListener('change', handleSystemThemeChange)
  }
})
</script>


<style scoped>
.dashboard-page {
  min-height: 100vh;
  background: var(--background, oklch(1 0 0));
  display: flex;
  flex-direction: column;
  font-family: var(--font-sans, 'Inter', 'Noto Sans SC', system-ui, sans-serif);
  overscroll-behavior: none;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

/* 顶部导航栏 */
.page-header {
  padding: 8px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--background, oklch(1 0 0));
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-logo {
  width: 28px;
  height: 28px;
  cursor: pointer;
  transition: transform 0.2s ease;
}

.header-logo:hover {
  transform: scale(1.1);
}

.header-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.header-version {
  font-family: var(--font-mono, 'SF Mono', monospace);
  font-size: 11px;
  font-weight: 500;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  padding: 2px 8px;
  border-radius: var(--radius-sm, 6px);
  margin-left: 8px;
  letter-spacing: 0.02em;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.user-trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  cursor: pointer;
  transition: opacity 0.2s ease;
}

.user-trigger:hover {
  opacity: 0.8;
}

.user-icon {
  font-size: 20px;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.user-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dropdown-arrow {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

/* 主体容器 */
.dashboard-container {
  display: flex;
  flex: 1;
  width: 100%;
  padding: 60px 24px 16px 24px;
  gap: 16px;
}

/* 左侧菜单栏 */
.sidebar {
  width: 180px;
  flex-shrink: 0;
  position: fixed;
  left: 0;
  top: 44px;
  height: calc(100vh - 44px);
  padding: 16px 24px;
  background: var(--background, oklch(1 0 0));
  border-right: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow-y: auto;
  overflow-x: hidden;
  display: flex;
  flex-direction: column;
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1), padding 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  z-index: 99;
}

.sidebar.collapsed {
  width: 80px;
  padding: 16px 12px;
}

.sidebar-nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-radius: var(--radius-md, 8px);
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease, padding 0.3s cubic-bezier(0.4, 0, 0.2, 1), gap 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
}

.sidebar.collapsed .nav-item {
  justify-content: center;
  padding: 12px;
  gap: 0;
}

.nav-item:hover {
  background: var(--accent, oklch(0.967 0.001 286.375));
  color: var(--accent-foreground, oklch(0.21 0.006 285.885));
}

.nav-item.active {
  background: var(--primary, oklch(0.21 0.006 285.885));
  color: var(--primary-foreground, oklch(0.985 0 0));
}

.nav-item .el-icon {
  font-size: 18px;
  flex-shrink: 0;
}

.nav-label {
  overflow: hidden;
  text-overflow: ellipsis;
  transition: opacity 0.2s ease, max-width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  max-width: 120px;
  opacity: 1;
}

.sidebar.collapsed .nav-label {
  max-width: 0;
  opacity: 0;
}

/* 折叠按钮 */
.sidebar-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 10px;
  cursor: pointer;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  border-radius: var(--radius-md, 8px);
  transition: background-color 0.2s ease, color 0.2s ease;
}

.sidebar-toggle:hover {
  background: var(--accent, oklch(0.967 0.001 286.375));
  color: var(--accent-foreground, oklch(0.21 0.006 285.885));
}

.sidebar-toggle .el-icon {
  font-size: 18px;
}

/* 右侧内容区 */
.main-content {
  flex: 1;
  min-width: 0;
  margin-left: 196px;
  transition: margin-left 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.main-content.sidebar-collapsed {
  margin-left: 96px;
}

/* 页脚 */
.page-footer {
  padding: 16px 32px;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.page-footer p {
  margin: 0;
  font-size: 13px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}


/* 全局加载遮罩 */
.global-loading-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: oklch(1 0 0 / 90%);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}

.loading-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  background: var(--card, oklch(1 0 0));
  padding: 40px 60px;
  border-radius: var(--radius-xl, 14px);
  box-shadow: var(--shadow-lg);
}

.loading-icon {
  font-size: 40px;
  color: var(--primary, oklch(0.21 0.006 285.885));
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.loading-content p {
  margin: 0;
  font-size: 16px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-weight: 500;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .dashboard-container {
    flex-direction: column;
    padding: 60px 16px 16px 16px;
  }
  
  .sidebar {
    width: 100% !important;
    position: relative;
    left: 0;
    top: 0;
    height: auto;
    overflow-y: visible;
  }
  
  .sidebar-nav {
    flex-direction: row;
    gap: 8px;
    overflow-x: auto;
  }
  
  .sidebar-toggle {
    display: none;
  }
  
  .nav-item {
    flex-shrink: 0;
    padding: 10px 16px;
  }
  
  .nav-label {
    display: inline !important;
    max-width: none !important;
    opacity: 1 !important;
  }
  
  .main-content,
  .main-content.sidebar-collapsed {
    margin-left: 0;
  }
  
  .page-header {
    padding: 12px 16px;
  }
  
  .header-title {
    display: none;
  }
  
  .user-name {
    display: none;
  }
}
</style>

<style>
/* 页面基础样式 */
html, body {
  overscroll-behavior: none;
  background: var(--background, oklch(1 0 0));
}

/* 退出登录确认弹窗样式 */
.logout-confirm-dialog {
  border-radius: 16px !important;
  padding: 24px !important;
}

.logout-confirm-dialog .el-message-box__header {
  padding-bottom: 12px;
}

.logout-confirm-dialog .el-message-box__title {
  font-size: 18px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.logout-confirm-dialog .el-message-box__content {
  padding: 16px 0;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 15px;
}

.logout-confirm-dialog .el-message-box__btns {
  padding-top: 16px;
}

.logout-confirm-btn {
  background: var(--primary, oklch(0.21 0.006 285.885)) !important;
  border-color: var(--primary, oklch(0.21 0.006 285.885)) !important;
  border-radius: var(--radius-md, 8px) !important;
  padding: 10px 24px !important;
  font-weight: 500 !important;
  color: var(--primary-foreground, oklch(0.985 0 0)) !important;
}

.logout-confirm-btn:hover {
  background: oklch(0.3 0.006 285.885) !important;
  border-color: oklch(0.3 0.006 285.885) !important;
}

.logout-cancel-btn {
  border-radius: var(--radius-md, 8px) !important;
  padding: 10px 24px !important;
  font-weight: 500 !important;
  border-color: var(--border, oklch(0.92 0.004 286.32)) !important;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938)) !important;
}

.logout-cancel-btn:hover {
  border-color: var(--ring, oklch(0.705 0.015 286.067)) !important;
  color: var(--foreground, oklch(0.141 0.005 285.823)) !important;
  background: var(--accent, oklch(0.967 0.001 286.375)) !important;
}

/* 用户下拉菜单样式 */
.user-dropdown-menu {
  border-radius: var(--radius-lg, 10px) !important;
  padding: 6px !important;
  min-width: 120px !important;
  box-shadow: var(--shadow-md) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32)) !important;
}

.user-dropdown-menu .el-dropdown-menu__item {
  border-radius: 6px !important;
  padding: 8px 12px !important;
  font-size: 13px !important;
  font-weight: 500 !important;
  margin: 2px 0 !important;
}

.user-dropdown-menu .logout-item {
  color: var(--foreground, oklch(0.141 0.005 285.823)) !important;
  display: flex !important;
  align-items: center !important;
  gap: 6px !important;
}

.user-dropdown-menu .logout-item:hover {
  background: var(--accent, oklch(0.967 0.001 286.375)) !important;
  color: var(--accent-foreground, oklch(0.21 0.006 285.885)) !important;
}

/* 主题切换按钮样式 */
.theme-toggle-btn {
  padding: 8px;
  border: none;
  background: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s ease;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  border-radius: var(--radius-md, 8px);
}

.theme-toggle-btn:hover {
  background: var(--accent, oklch(0.967 0.001 286.375));
}

.theme-toggle-btn .el-icon {
  font-size: 20px;
}

/* 公告按钮样式 */
.notification-btn {
  padding: 8px;
  border: none;
  background: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s ease;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  border-radius: var(--radius-md, 8px);
  position: relative;
}

.notification-btn:hover {
  background: var(--accent, oklch(0.967 0.001 286.375));
}

.notification-btn .el-icon {
  font-size: 20px;
}

/* 公告未读红点 */
.notification-dot {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 8px;
  height: 8px;
  background-color: #ef4444;
  border-radius: 50%;
  border: 1.5px solid #fff;
}

/* 公告弹窗样式 */
.notification-content {
  max-height: 400px;
  overflow-y: auto;
}

.no-notification {
  text-align: center;
  color: #86868b;
  padding: 40px 0;
  font-size: 14px;
}

.notification-list {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.notification-item {
  padding: 16px;
  border-radius: var(--radius-lg, 10px);
  background: var(--secondary, oklch(0.967 0.001 286.375));
  margin-bottom: 12px;
  transition: background-color 0.2s ease;
}

.notification-item:last-child {
  margin-bottom: 0;
}

.notification-item:hover {
  background: var(--accent, oklch(0.967 0.001 286.375));
}

.notification-item h4 {
  font-size: 15px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  margin: 0 0 8px;
  line-height: 1.4;
}

.notification-item p {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  margin: 0 0 10px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.notification-time {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

/* 黑暗模式样式 */
:root.dark-mode .dashboard-page {
  background: oklch(0.141 0.005 285.823);
}

:root.dark-mode .page-header {
  background: oklch(0.141 0.005 285.823 / 95%);
  border-bottom: 1px solid oklch(1 0 0 / 10%);
}

:root.dark-mode .header-title {
  color: oklch(0.985 0 0);
}

:root.dark-mode .header-version {
  color: oklch(0.705 0.015 286.067);
  border-color: oklch(1 0 0 / 15%);
  background: transparent;
}

:root.dark-mode .header-logo {
  filter: invert(1) hue-rotate(180deg);
}

:root.dark-mode .theme-toggle-btn,
:root.dark-mode .notification-btn {
  color: oklch(0.985 0 0);
}

:root.dark-mode .theme-toggle-btn:hover,
:root.dark-mode .notification-btn:hover {
  background: oklch(1 0 0 / 10%);
}

:root.dark-mode .notification-dot {
  border-color: oklch(0.141 0.005 285.823);
}

:root.dark-mode .user-icon,
:root.dark-mode .user-name {
  color: oklch(0.985 0 0);
}

:root.dark-mode .sidebar {
  background: oklch(0.141 0.005 285.823);
  border-right: 1px solid oklch(1 0 0 / 10%);
}


:root.dark-mode .nav-item {
  color: oklch(0.705 0.015 286.067);
}

:root.dark-mode .nav-item:hover {
  background: rgb(55, 65, 81);
  color: oklch(0.985 0 0);
}

:root.dark-mode .nav-item.active {
  background: oklch(0.92 0.004 286.32);
  color: oklch(0.21 0.006 285.885);
}

:root.dark-mode .sidebar-toggle {
  color: oklch(0.705 0.015 286.067);
}

:root.dark-mode .sidebar-toggle:hover {
  background: rgb(55, 65, 81);
  color: oklch(0.985 0 0);
}


:root.dark-mode .page-footer p {
  color: oklch(0.705 0.015 286.067);
}

:root.dark-mode .global-loading-overlay {
  background: oklch(0.141 0.005 285.823 / 95%);
}

:root.dark-mode .loading-content {
  background: oklch(0.21 0.006 285.885);
  box-shadow: var(--shadow-lg);
}

:root.dark-mode .loading-content p {
  color: oklch(0.705 0.015 286.067);
}

/* 公告弹窗对话框样式 */
.notification-modal.el-dialog {
  border-radius: var(--radius-lg, 10px) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.notification-modal .el-dialog__header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.notification-modal .el-dialog__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.notification-modal .el-dialog__body {
  padding: 20px;
}

/* 黑暗模式弹窗样式 */
:root.dark-mode .notification-modal.el-dialog {
  background: oklch(0.21 0.006 285.885) !important;
  border-color: oklch(1 0 0 / 10%) !important;
}

:root.dark-mode .notification-modal .el-dialog__header {
  border-bottom-color: oklch(1 0 0 / 10%) !important;
}

:root.dark-mode .notification-modal .el-dialog__title {
  color: oklch(0.985 0 0) !important;
}

:root.dark-mode .notification-item {
  background: oklch(0.274 0.006 286.033);
}

:root.dark-mode .notification-item:hover {
  background: oklch(0.32 0.006 286);
}

:root.dark-mode .notification-item h4 {
  color: oklch(0.985 0 0);
}

:root.dark-mode .notification-item p {
  color: oklch(0.705 0.015 286.067);
}

:root.dark-mode .notification-time {
  color: oklch(0.552 0.016 285.938);
}

:root.dark-mode .notification-item {
  border-bottom-color: oklch(1 0 0 / 10%);
}

:root.dark-mode .no-notification {
  color: oklch(0.705 0.015 286.067);
}

/* 黑暗模式下拉菜单 */
:root.dark-mode .user-dropdown-menu {
  background: oklch(0.21 0.006 285.885) !important;
  border: 1px solid oklch(1 0 0 / 10%) !important;
}

:root.dark-mode .user-dropdown-menu .el-dropdown-menu__item {
  color: oklch(0.985 0 0) !important;
  background: transparent !important;
}

:root.dark-mode .user-dropdown-menu .el-dropdown-menu__item:hover {
  background: oklch(1 0 0 / 8%) !important;
  color: oklch(0.985 0 0) !important;
}

:root.dark-mode .user-dropdown-menu .el-dropdown-menu__item:focus,
:root.dark-mode .user-dropdown-menu .el-dropdown-menu__item.is-active {
  background: transparent !important;
  color: oklch(0.985 0 0) !important;
}

/* 使用教程弹窗样式 */
.tutorial-modal.el-dialog {
  border-radius: 16px !important;
  overflow: hidden;
}

.tutorial-modal .el-dialog__header {
  padding: 12px 12px 0;
  margin-right: 0;
}

.tutorial-modal .el-dialog__headerbtn {
  top: 12px;
  right: 12px;
  width: 28px;
  height: 28px;
  font-size: 16px;
}

.tutorial-modal .el-dialog__body {
  padding: 20px 24px 24px;
}

.tutorial-content {
  text-align: center;
}

.tutorial-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  margin: 0 0 14px 0;
}

.tutorial-hints {
  background: var(--secondary, oklch(0.967 0.001 286.375));
  border-radius: var(--radius-lg, 10px);
  padding: 14px 16px;
  margin: 0 0 20px 0;
}

.tutorial-hint-item {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.tutorial-hint-item:first-child {
  margin-bottom: 10px;
}

.tutorial-hint-item .el-icon {
  color: var(--primary, oklch(0.21 0.006 285.885));
  font-size: 15px;
  flex-shrink: 0;
}

.tutorial-actions {
  display: flex;
  flex-direction: row;
  gap: 12px;
}

.tutorial-btn {
  flex: 1;
  height: 40px;
  border-radius: var(--radius-lg, 10px);
  font-size: 14px;
  font-weight: 500;
  transition: background-color 0.2s ease, border-color 0.2s ease, color 0.2s ease;
  padding: 0 16px;
}

.tutorial-btn.primary {
  background: var(--primary, oklch(0.21 0.006 285.885));
  border-color: var(--primary, oklch(0.21 0.006 285.885));
  color: var(--primary-foreground, oklch(0.985 0 0));
}

.tutorial-btn.primary:hover {
  background: oklch(0.3 0.006 285.885);
  border-color: oklch(0.3 0.006 285.885);
}

.tutorial-btn.secondary {
  background: transparent;
  border-color: var(--border, oklch(0.92 0.004 286.32));
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.tutorial-btn.secondary:hover {
  background: var(--accent, oklch(0.967 0.001 286.375));
  border-color: var(--border, oklch(0.92 0.004 286.32));
  color: var(--accent-foreground, oklch(0.21 0.006 285.885));
}

/* 黑暗模式教程弹窗样式 */
:root.dark-mode .tutorial-modal.el-dialog {
  background: oklch(0.21 0.006 285.885) !important;
  border-color: oklch(1 0 0 / 10%) !important;
}

:root.dark-mode .tutorial-title {
  color: oklch(0.985 0 0);
}

:root.dark-mode .tutorial-hints {
  background: oklch(0.274 0.006 286.033);
}

:root.dark-mode .tutorial-hint-item {
  color: oklch(0.705 0.015 286.067);
}

:root.dark-mode .tutorial-btn.primary {
  background: oklch(0.92 0.004 286.32);
  border-color: oklch(0.92 0.004 286.32);
  color: oklch(0.21 0.006 285.885);
}

:root.dark-mode .tutorial-btn.primary:hover {
  background: oklch(0.85 0.004 286.32);
  border-color: oklch(0.85 0.004 286.32);
}

:root.dark-mode .tutorial-btn.secondary {
  border-color: oklch(1 0 0 / 15%);
  color: oklch(0.705 0.015 286.067);
}

:root.dark-mode .tutorial-btn.secondary:hover {
  background: oklch(1 0 0 / 8%);
  border-color: oklch(1 0 0 / 20%);
  color: oklch(0.985 0 0);
}
</style>
