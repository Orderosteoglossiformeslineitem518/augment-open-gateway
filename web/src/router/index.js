import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    name: 'Home',
    component: () => import('@/views/HomePage.vue'),
    meta: {
      title: 'AugmentGateway - 一站式享受 AugmentCode 服务',
      requiresAuth: false,
      noSuffix: true
    }
  },
  {
    path: '/login',
    name: 'UserLogin',
    component: () => import('@/views/UserLogin.vue'),
    meta: {
      title: '用户登录',
      requiresAuth: false
    }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/Register.vue'),
    meta: {
      title: '用户注册',
      requiresAuth: false
    }
  },
  {
    path: '/admin',
    name: 'AdminLogin',
    component: () => import('@/views/Login.vue'),
    meta: {
      title: '管理员登录',
      requiresAuth: false
    }
  },
  {
    path: '/admin/dashboard',
    name: 'Dashboard',
    component: () => import('@/views/Dashboard.vue'),
    meta: {
      title: '仪表板',
      icon: 'DataBoard',
      requiresAuth: true
    }
  },
  {
    path: '/admin/tokens',
    name: 'Tokens',
    component: () => import('@/views/Tokens.vue'),
    meta: {
      title: 'Token管理',
      icon: 'Key',
      requiresAuth: true
    }
  },
  {
    path: '/admin/request-records',
    name: 'RequestRecords',
    component: () => import('@/views/RequestRecords.vue'),
    meta: {
      title: '请求记录',
      icon: 'Document',
      requiresAuth: true
    }
  },
  {
    path: '/admin/proxy-management',
    name: 'ProxyManagement',
    component: () => import('@/views/ProxyManagement.vue'),
    meta: {
      title: '代理管理',
      icon: 'Share',
      requiresAuth: true
    }
  },
  {
    path: '/admin/remote-models',
    name: 'RemoteModelManagement',
    component: () => import('@/views/RemoteModelManagement.vue'),
    meta: {
      title: '模型同步',
      icon: 'Connection',
      requiresAuth: true
    }
  },
  {
    path: '/admin/user-management',
    name: 'UserManagement',
    component: () => import('@/views/UserManagement.vue'),
    meta: {
      title: '用户管理',
      icon: 'User',
      requiresAuth: true
    }
  },
  {
    path: '/admin/notification-management',
    name: 'NotificationManagement',
    component: () => import('@/views/NotificationManagement.vue'),
    meta: {
      title: '插件公告',
      icon: 'Bell',
      requiresAuth: true
    }
  },
  {
    path: '/admin/system-announcement',
    name: 'SystemAnnouncementManagement',
    component: () => import('@/views/SystemAnnouncementManagement.vue'),
    meta: {
      title: '公告管理',
      icon: 'Notification',
      requiresAuth: true
    }
  },
  {
    path: '/admin/invitation-codes',
    name: 'InvitationCodeManagement',
    component: () => import('@/views/InvitationCodeManagement.vue'),
    meta: {
      title: '邀请码管理',
      icon: 'Ticket',
      requiresAuth: true
    }
  },
  {
    path: '/admin/system-config',
    name: 'SystemConfig',
    component: () => import('@/views/SystemConfig.vue'),
    meta: {
      title: '系统配置',
      icon: 'Setting',
      requiresAuth: true
    }
  },
  {
    path: '/user/dashboard',
    name: 'UserDashboard',
    component: () => import('@/views/UserDashboard.vue'),
    meta: {
      title: '用户中心',
      requiresUserAuth: true,
      noSuffix: true
    }
  },
  {
    path: '/about',
    name: 'About',
    component: () => import('@/views/AboutPage.vue'),
    meta: {
      title: '关于',
      requiresAuth: false
    }
  },

  // 通配符路由 - 必须放在最后
  {
    path: '/:pathMatch(.*)*',
    redirect: '/'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach(async (to, from, next) => {
  // 设置页面标题
  if (to.meta.title) {
    document.title = to.meta.noSuffix ? to.meta.title : `${to.meta.title} - Augment Gateway`
  }

  // 动态导入store以避免循环依赖
  const { useAuthStore } = await import('@/store')
  const authStore = useAuthStore()

  // 处理管理后台登录页面的逻辑
  if (to.path === '/admin') {
    // 如果已经登录，直接跳转到仪表板
    if (authStore.isLoggedIn) {
      next('/admin/dashboard')
      return
    }
    // 未登录，正常显示登录页
    next()
    return
  }

  // OAuth 2.0 端点直接放行（API端点，不需要前端路由处理）
  if (to.path === '/authorize' || to.path === '/token') {
    next()
    return
  }

  // 公开页面处理（登录、注册页面）
  const publicPages = ['/', '/register']
  if (publicPages.includes(to.path)) {
    // 如果用户已登录且访问首页，重定向到用户控制台
    if (to.path === '/' && localStorage.getItem('user_token')) {
      next('/user/dashboard')
      return
    }
    next()
    return
  }

  // 检查用户页面认证（需要用户登录）
  if (to.meta.requiresUserAuth) {
    const userToken = localStorage.getItem('user_token')
    if (!userToken) {
      // 未登录，重定向到用户登录页
      next('/')
      return
    }
  }

  // 检查是否需要认证（管理后台页面）
  if (to.path.startsWith('/admin/') && to.meta.requiresAuth !== false) {
    if (!authStore.isLoggedIn) {
      // 未登录，重定向到管理员登录页
      next('/admin')
      return
    }
  }

  next()
})

export default router
