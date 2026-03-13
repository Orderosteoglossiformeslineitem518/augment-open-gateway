<template>
  <el-dialog
    v-model="visible"
    :show-close="true"
    :close-on-click-modal="true"
    :close-on-press-escape="true"
    width="420px"
    class="auth-modal"
    @close="handleClose"
  >
    <div class="modal-content">
      <h2 class="modal-title">登录</h2>
      
      <el-form
        ref="loginFormRef"
        :model="loginForm"
        :rules="loginRules"
        class="auth-form"
        @submit.prevent="handleLogin"
      >
        <div class="form-group">
          <label class="input-label"><span class="required">*</span> 用户名或邮箱</label>
          <el-form-item prop="username">
            <el-input
              v-model="loginForm.username"
              placeholder="name@example.com"
              :disabled="loading"
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
            />
          </el-form-item>
        </div>

        <!-- Turnstile 人机验证 -->
        <div class="turnstile-section">
          <div v-if="turnstileLoading" class="turnstile-loading">
            <el-icon class="loading-icon"><Loading /></el-icon>
            <span>人机验证加载中...</span>
          </div>
          <div v-show="!turnstileLoading" id="login-turnstile-widget" class="turnstile-widget"></div>
        </div>

        <el-button
          type="primary"
          :loading="loading"
          :disabled="!turnstileToken"
          @click="handleLogin"
          class="submit-btn"
        >
          {{ loading ? '登录中...' : '登录' }}
        </el-button>
        
        <div class="switch-hint">
          还没有账号？ <a href="#" @click.prevent="switchToRegister">立即注册</a>
        </div>
      </el-form>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, watch, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import { userAuthAPI } from '@/api'

const props = defineProps({
  modelValue: Boolean
})

const emit = defineEmits(['update:modelValue', 'switch-to-register'])

const router = useRouter()
const loginFormRef = ref(null)
const loading = ref(false)
const turnstileToken = ref('')
const turnstileWidgetId = ref(null)
const turnstileLoading = ref(false)

const visible = ref(props.modelValue)

watch(() => props.modelValue, (val) => {
  visible.value = val
  if (val) {
    nextTick(() => {
      setTimeout(() => initTurnstile(), 100)
    })
  }
})

watch(visible, (val) => {
  emit('update:modelValue', val)
})

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

const handleClose = () => {
  visible.value = false
  loginForm.username = ''
  loginForm.password = ''
  loginFormRef.value?.resetFields()
  // 清理 Turnstile
  turnstileToken.value = ''
  if (window.turnstile && turnstileWidgetId.value) {
    window.turnstile.remove(turnstileWidgetId.value)
    turnstileWidgetId.value = null
  }
}

// Turnstile 人机验证相关
const initTurnstile = () => {
  turnstileLoading.value = true
  // 脚本已在 App.vue 预加载，这里只需等待加载完成
  if (window.turnstile) {
    renderTurnstile()
  } else {
    // 等待脚本加载完成（最多等待 5 秒）
    let attempts = 0
    const maxAttempts = 50
    const checkInterval = setInterval(() => {
      attempts++
      if (window.turnstile) {
        clearInterval(checkInterval)
        renderTurnstile()
      } else if (attempts >= maxAttempts) {
        clearInterval(checkInterval)
        turnstileLoading.value = false
        console.error('Turnstile 脚本加载超时')
      }
    }, 100)
  }
}

const renderTurnstile = () => {
  const container = document.getElementById('login-turnstile-widget')
  if (!window.turnstile || !container) {
    turnstileLoading.value = false
    return
  }

  // 先清理旧 widget
  if (turnstileWidgetId.value) {
    try { window.turnstile.remove(turnstileWidgetId.value) } catch (e) {}
    turnstileWidgetId.value = null
  }
  container.innerHTML = ''

  const isLocalhost = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
  const siteKey = isLocalhost ? '1x00000000000000000000AA' : '0x4AAAAAACH-xo8bXfQMEWZ1'

  try {
    turnstileWidgetId.value = window.turnstile.render('#login-turnstile-widget', {
      sitekey: siteKey,
      theme: 'light',
      size: 'normal',
      callback: (token) => { turnstileToken.value = token },
      'error-callback': () => { turnstileToken.value = '' },
      'expired-callback': () => { turnstileToken.value = '' },
      'timeout-callback': () => { turnstileToken.value = '' }
    })
    // 渲染启动后立即结束 loading，让用户能看到验证框
    turnstileLoading.value = false
  } catch (e) {
    turnstileLoading.value = false
    console.error('Turnstile 渲染失败:', e)
  }
}

const switchToRegister = () => {
  handleClose()
  emit('switch-to-register')
}

const handleLogin = async () => {
  if (!loginFormRef.value) return

  // 检查人机验证
  if (!turnstileToken.value) {
    ElMessage.error('请完成人机验证')
    return
  }

  try {
    await loginFormRef.value.validate()
    
    loading.value = true
    const result = await userAuthAPI.login({
      ...loginForm,
      turnstile_token: turnstileToken.value
    })
    
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
    handleClose()
    router.push('/user/dashboard')
  } catch (error) {
    // 只有当错误未被拦截器处理过时才显示错误消息，避免重复提示
    if (!error.fields && !error.handled) {
      ElMessage.error(error.message || '登录失败')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.modal-content {
  padding: 20px;
}

.modal-title {
  font-size: 28px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  margin: 0 0 32px 0;
  text-align: center;
}

.form-group {
  margin-bottom: 20px;
}

.input-label {
  display: block;
  font-size: 14px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  margin-bottom: 8px;
}

.required {
  color: #f56c6c;
  margin-right: 4px;
}

:deep(.el-input__wrapper) {
  background-color: var(--secondary, oklch(0.967 0.001 286.375));
  box-shadow: none !important;
  border: 1px solid transparent;
  border-radius: var(--radius-xl, 14px);
  padding: 8px 12px;
  height: 48px;
  transition: all 0.2s ease;
}

:deep(.el-input__wrapper:hover),
:deep(.el-input__wrapper.is-focus) {
  background-color: var(--card, oklch(1 0 0));
  border-color: var(--foreground, oklch(0.141 0.005 285.823));
}

:deep(.el-input__inner) {
  font-size: 15px;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.submit-btn {
  width: 100%;
  height: 52px;
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
}

.switch-hint {
  margin-top: 24px;
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  text-align: center;
}

.switch-hint a {
  color: var(--primary, oklch(0.21 0.006 285.885));
  text-decoration: none;
  font-weight: 600;
}

.switch-hint a:hover {
  text-decoration: underline;
}

/* Turnstile 人机验证样式 */
.turnstile-section {
  text-align: center;
  margin: 16px 0;
  padding: 8px;
  background: var(--secondary, oklch(0.967 0.001 286.375));
  border-radius: var(--radius-xl, 14px);
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  min-height: 65px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.turnstile-widget {
  display: flex;
  justify-content: center;
}

.turnstile-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 14px;
}

.turnstile-loading .loading-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.submit-btn:disabled {
  background: oklch(0.7 0.1 260) !important;
  border-color: oklch(0.7 0.1 260) !important;
  color: var(--card, oklch(1 0 0)) !important;
}
</style>

<style>
.auth-modal.el-dialog,
.el-overlay .auth-modal.el-dialog {
  border-radius: var(--radius-lg, 10px) !important;
  overflow: hidden;
}

.auth-modal .el-dialog__header,
.el-overlay .auth-modal .el-dialog__header {
  padding: 16px 16px 0;
  margin-right: 0;
}

.auth-modal .el-dialog__headerbtn,
.el-overlay .auth-modal .el-dialog__headerbtn {
  top: 16px;
  right: 16px;
  width: 32px;
  height: 32px;
  font-size: 18px;
}

.auth-modal .el-dialog__body,
.el-overlay .auth-modal .el-dialog__body {
  padding: 20px;
}
</style>
