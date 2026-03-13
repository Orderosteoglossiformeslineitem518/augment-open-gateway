<template>
  <el-container class="layout-container">
    <!-- 侧边栏 -->
    <el-aside :width="sidebarWidth" class="sidebar">
      <div class="logo">
        <img src="/logo.svg" alt="Logo" class="logo-img" :class="{ 'collapsed': appStore.sidebarCollapsed }">
        <transition name="logo-text">
          <span class="logo-text" v-show="!appStore.sidebarCollapsed">Augment Gateway</span>
        </transition>
      </div>
      
      <el-menu
        :default-active="$route.path"
        :collapse="appStore.sidebarCollapsed"
        :unique-opened="true"
        router
        class="sidebar-menu"
      >
        <el-menu-item
          v-for="route in menuRoutes"
          :key="route.path"
          :index="route.path"
        >
          <el-icon><component :is="route.meta.icon" /></el-icon>
          <template #title>{{ route.meta.title }}</template>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <!-- 主内容区 -->
    <el-container class="main-container">
      <!-- 顶部导航 -->
      <el-header class="header">
        <div class="header-left">
          <el-button
            type="text"
            @click="appStore.toggleSidebar"
            class="sidebar-toggle"
          >
            <el-icon><Expand v-if="appStore.sidebarCollapsed" /><Fold v-else /></el-icon>
          </el-button>
          
          <el-breadcrumb separator="/">
            <el-breadcrumb-item>{{ currentRoute?.meta?.title || '首页' }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        
        <div class="header-right">
          <el-button
            type="text"
            @click="appStore.toggleTheme"
            class="theme-toggle"
          >
            <el-icon><Moon v-if="!appStore.isDark" /><Sunny v-else /></el-icon>
          </el-button>
          
          <el-dropdown>
            <el-button type="text" class="user-menu">
              <el-icon><User /></el-icon>
              <span>{{ authStore.user?.username || '管理员' }}</span>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="showUserSettings">个人设置</el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <!-- 主内容 -->
      <el-main class="main-content">
        <ErrorBoundary>
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </ErrorBoundary>
      </el-main>
    </el-container>

    <!-- 用户设置对话框 -->
    <UserSettingsDialog
      v-model="showUserSettingsDialog"
      @success="handleUserSettingsSuccess"
    />
  </el-container>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore, useAuthStore } from '@/store'
import { ElMessage, ElMessageBox } from 'element-plus'
import ErrorBoundary from './ErrorBoundary.vue'
import UserSettingsDialog from './UserSettingsDialog.vue'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

// 响应式数据
const showUserSettingsDialog = ref(false)

// 计算属性
const sidebarWidth = computed(() => appStore.sidebarCollapsed ? '64px' : '240px')
const currentRoute = computed(() => route)

// 菜单路由
const menuRoutes = computed(() => {
  return router.getRoutes().filter(route =>
    route.meta?.title &&
    route.path.startsWith('/admin/') &&
    route.path !== '/admin/login' &&
    route.path !== '/admin' &&
    route.meta.requiresAuth !== false
  )
})

// 显示用户设置
const showUserSettings = () => {
  showUserSettingsDialog.value = true
}

// 用户设置成功回调
const handleUserSettingsSuccess = () => {
  ElMessage.success('个人信息更新成功')
}

// 处理登出
const handleLogout = async () => {
  try {
    await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })

    await authStore.logout()
    ElMessage.success('已退出登录')
    router.push('/admin')
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('退出登录失败')
    }
  }
}

// 初始化
onMounted(() => {
  // 移除用户中心的暗黑模式类，管理后台使用独立的 dark 类
  document.documentElement.classList.remove('dark-mode')
  appStore.initTheme()
  appStore.initSidebar()
  authStore.initAuth()
})
</script>

<style scoped>
.layout-container {
  height: 100vh;
}

.sidebar {
  background: linear-gradient(180deg, #1f2937 0%, #111827 100%);
  border-right: 1px solid rgba(75, 85, 99, 0.3);
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  box-shadow: 2px 0 8px rgba(0, 0, 0, 0.1);
  /* 防止内容在过渡期间跳动 */
  will-change: width;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 20px;
  border-bottom: 1px solid rgba(75, 85, 99, 0.3);
  overflow: hidden;
  background: rgba(255, 255, 255, 0.05);
}

.logo-img {
  width: 32px;
  height: 32px;
  margin-right: 12px;
  transition: all 0.3s ease;
  flex-shrink: 0;
}

.logo-img.collapsed {
  margin-right: 0;
}

.logo-text {
  font-size: 18px;
  font-weight: 600;
  color: #ffffff;
  white-space: nowrap;
  overflow: hidden;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
}

/* Logo文字过渡动画 */
.logo-text-enter-active,
.logo-text-leave-active {
  transition: all 0.3s ease;
}

.logo-text-enter-from {
  opacity: 0;
  transform: translateX(-20px);
}

.logo-text-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}

.sidebar-menu {
  border: none;
  background: transparent;
  height: calc(100vh - 60px);
  overflow-y: auto;
  overflow-x: hidden;
  /* 确保菜单过渡平滑 */
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.sidebar-menu .el-menu-item {
  color: rgba(255, 255, 255, 0.8);
  border-radius: 8px;
  margin: 4px 8px;
  padding: 0 16px;
  height: 48px;
  line-height: 48px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
  border: none;
  background: transparent !important;
  /* 防止布局跳动 */
  will-change: transform, background-color;
}

/* 统一图标样式，防止尺寸变化 */
.sidebar-menu .el-menu-item .el-icon {
  width: 18px !important;
  height: 18px !important;
  font-size: 18px !important;
  margin-right: 8px !important;
  transition: none !important;
  flex-shrink: 0 !important;
  display: inline-flex !important;
  align-items: center !important;
  justify-content: center !important;
}

/* 折叠状态下的菜单项居中 */
.sidebar-menu.el-menu--collapse .el-menu-item {
  margin: 4px auto;
  padding: 0 !important;
  display: flex !important;
  justify-content: center !important;
  align-items: center !important;
  width: 48px !important;
  height: 48px !important;
  text-align: center;
  position: relative;
  background: transparent !important;
}

.sidebar-menu.el-menu--collapse .el-menu-item .el-icon {
  width: 18px !important;
  height: 18px !important;
  font-size: 18px !important;
  margin: 0 !important;
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%) !important;
  transition: none !important;
}

.sidebar-menu.el-menu--collapse .el-menu-item .el-menu-item__title {
  display: none !important;
}

/* 折叠状态下的悬停效果 */
.sidebar-menu.el-menu--collapse .el-menu-item:hover {
  transform: none;
  background: transparent !important;
}

.sidebar-menu.el-menu--collapse .el-menu-item:hover .el-icon {
  transform: translate(-50%, -50%) !important;
}

.sidebar-menu.el-menu--collapse .el-menu-item.is-active {
  width: 48px !important;
  background: transparent !important;
}

.sidebar-menu.el-menu--collapse .el-menu-item.is-active::before {
  left: -4px;
  width: 2px;
  height: 16px;
}

.sidebar-menu .el-menu-item:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.1) !important;
  transform: translateX(2px);
}

.sidebar-menu .el-menu-item.is-active {
  color: #ffffff;
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.8) 0%, rgba(103, 58, 183, 0.8) 100%) !important;
  box-shadow: 0 4px 12px rgba(64, 158, 255, 0.3);
}

.sidebar-menu .el-menu-item .el-menu-item__title {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  color: inherit;
}

.main-container {
  background: var(--bg-color-page);
}

.header {
  background: var(--bg-color);
  border-bottom: 1px solid var(--border-color-lighter);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sidebar-toggle,
.theme-toggle,
.user-menu {
  color: var(--text-color-regular);
  transition: color 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.sidebar-toggle:hover,
.theme-toggle:hover,
.user-menu:hover {
  color: var(--primary-color);
}

/* 确保切换按钮图标稳定 */
.sidebar-toggle .el-icon {
  width: 16px !important;
  height: 16px !important;
  font-size: 16px !important;
  transition: none !important;
  display: inline-flex !important;
  align-items: center !important;
  justify-content: center !important;
}

.main-content {
  padding: 24px;
  overflow-y: auto;
  background: var(--bg-color-page);
}

/* 动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* 响应式 */
@media (max-width: 768px) {
  .header {
    padding: 0 16px;
  }
  
  .main-content {
    padding: 16px;
  }
  
  .logo {
    padding: 0 16px;
  }
}
</style>
