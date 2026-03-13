<template>
  <div class="login-page">
    <!-- 顶部导航 -->
    <header class="page-header">
      <div class="header-left">
        <img src="/logo.svg" alt="AugmentGateway" class="header-logo" />
        <span class="header-title">AugmentGateway</span>
      </div>
      <div class="header-right">
        <router-link to="/" class="nav-btn active">登录</router-link>
        <router-link to="/register" class="nav-btn">注册</router-link>
      </div>
    </header>

    <!-- 主体内容 -->
    <main class="login-main">
      <!-- Logo和标题区域 - 表单外部 -->
      <div class="brand-section">
        <img src="/logo.svg" alt="AugmentGateway" class="brand-logo" />
        <span class="brand-subtitle">您的贴心Augment助手</span>
      </div>

      <div class="login-card">
        <h2 class="card-title">登录</h2>

        <el-form
          ref="loginFormRef"
          :model="loginForm"
          :rules="loginRules"
          class="login-form"
          @submit.prevent="handleLogin"
        >
          <div class="form-group">
            <label class="input-label"><span class="required">*</span> 用户名或邮箱</label>
            <el-form-item prop="username">
              <el-input
                v-model="loginForm.username"
                placeholder="name@example.com"
                :disabled="loading"
                class="custom-input"
              />
            </el-form-item>
          </div>

          <div class="form-group">
            <label class="input-label"><span class="required">*</span> 密码</label>
            <el-form-item prop="password">
              <el-input
                v-model="loginForm.password"
                type="password"
                placeholder="请输入密码"
                :disabled="loading"
                show-password
                @keyup.enter="handleLogin"
                class="custom-input"
              />
            </el-form-item>
          </div>

          <el-button
            type="primary"
            :loading="loading"
            @click="handleLogin"
            class="submit-btn"
          >
            {{ loading ? '登录中...' : '登录' }}
          </el-button>
          
          <div class="register-hint">
            还没有账号？ <router-link to="/register">立即注册</router-link>
          </div>
        </el-form>
      </div>
    </main>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { userAuthAPI } from '@/api'

const router = useRouter()
const loginFormRef = ref(null)
const loading = ref(false)

const loginForm = reactive({
  username: '',
  password: ''
})

const loginRules = {
  username: [
    { required: true, message: '请输入用户名或邮箱', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码长度不能少于6位', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  if (!loginFormRef.value) return

  try {
    await loginFormRef.value.validate()

    loading.value = true
    const result = await userAuthAPI.login(loginForm)

    if (result.token) {
      localStorage.setItem('user_token', result.token)
    }
    if (result.refresh_token) {
      localStorage.setItem('user_refresh_token', result.refresh_token)
    }
    // 使用 expires_in 计算过期时间
    if (result.expires_in) {
      const expiresAt = new Date(Date.now() + result.expires_in * 1000).toISOString()
      localStorage.setItem('user_token_expires_at', expiresAt)
    }
    if (result.user) {
      localStorage.setItem('user_info', JSON.stringify(result.user))
    }

    ElMessage.success('登录成功')
    router.push('/user/dashboard')
  } catch (error) {
    // 如果错误已在拦截器中处理（如封禁错误），则不重复显示
    if (!error.fields && !error.handled) {
      ElMessage.error(error.message || '登录失败')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  background: linear-gradient(135deg, oklch(0.97 0.02 85) 0%, oklch(0.96 0.015 220) 50%, oklch(0.98 0.01 240) 100%);
  display: flex;
  flex-direction: column;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
}

.page-header {
  padding: 20px 40px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 10;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-logo {
  width: 28px;
  height: 28px;
}

.header-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.header-right {
  display: flex;
  gap: 16px;
}

.nav-btn {
  text-decoration: none;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-weight: 500;
  font-size: 14px;
  padding: 8px 16px;
  border-radius: var(--radius-md, 8px);
  transition: all 0.2s ease;
}

.nav-btn:hover {
  color: var(--foreground, oklch(0.141 0.005 285.823));
  background: oklch(1 0 0 / 0.5);
}

.nav-btn.active {
  color: var(--card, oklch(1 0 0));
  background: var(--primary, oklch(0.21 0.006 285.885));
}

.login-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 20px;
  gap: 24px;
}

.login-card {
  width: 100%;
  max-width: 420px;
  background: var(--card, oklch(1 0 0));
  border-radius: 24px;
  padding: 40px;
  box-shadow: 0 20px 40px oklch(0 0 0 / 0.08);
  text-align: center;
}

.brand-section {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.brand-logo {
  width: 32px;
  height: 32px;
}

.brand-subtitle {
  font-size: 20px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  position: relative;
  background: linear-gradient(
    90deg,
    oklch(0.141 0.005 285.823) 0%,
    oklch(0.141 0.005 285.823) 40%,
    oklch(0.7 0.17 55) 50%,
    oklch(0.141 0.005 285.823) 60%,
    oklch(0.141 0.005 285.823) 100%
  );
  background-size: 200% 100%;
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  animation: shimmer 3s ease-in-out infinite;
}

@keyframes shimmer {
  0% {
    background-position: 100% 0;
  }
  100% {
    background-position: -100% 0;
  }
}

.card-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  margin: 0 0 32px 0;
}

.form-group {
  margin-bottom: 20px;
  text-align: left;
}

.input-label {
  display: block;
  font-size: 14px;
  font-weight: 500;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  margin-bottom: 8px;
}

.required {
  color: #f56c6c;
  margin-right: 4px;
}

/* 覆盖 Element Plus 样式 */
:deep(.el-input__wrapper) {
  background-color: var(--secondary, oklch(0.967 0.001 286.375));
  box-shadow: none !important;
  border: 1px solid transparent;
  border-radius: var(--radius-xl, 14px);
  padding: 8px 12px;
  height: 44px;
  transition: all 0.2s ease;
}

:deep(.el-input__wrapper:hover),
:deep(.el-input__wrapper.is-focus) {
  background-color: var(--card, oklch(1 0 0));
  border-color: var(--border, oklch(0.92 0.004 286.32));
  box-shadow: 0 0 0 4px oklch(0.92 0.004 286.32 / 0.4) !important;
}

:deep(.el-input__inner) {
  font-size: 15px;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.submit-btn {
  width: 100%;
  height: 48px;
  background: var(--primary, oklch(0.21 0.006 285.885));
  border-color: var(--primary, oklch(0.21 0.006 285.885));
  border-radius: var(--radius-xl, 14px);
  font-size: 16px;
  font-weight: 600;
  margin-top: 12px;
  transition: all 0.2s ease;
}

.submit-btn:hover {
  background: oklch(0.3 0.006 285.885);
  border-color: oklch(0.3 0.006 285.885);
  transform: translateY(-1px);
}

.register-hint {
  margin-top: 24px;
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.register-hint a {
  color: var(--primary, oklch(0.21 0.006 285.885));
  text-decoration: none;
  font-weight: 600;
  margin-left: 4px;
}

.register-hint a:hover {
  text-decoration: underline;
}

@media (max-width: 480px) {
  .login-card {
    padding: 30px 20px;
    border-radius: 20px;
  }
  
  .page-header {
    padding: 16px 20px;
  }
  
  .header-title {
    display: none;
  }
  
  .brand-logo {
    width: 28px;
    height: 28px;
  }
  
  .brand-subtitle {
    font-size: 16px;
  }
}
</style>
