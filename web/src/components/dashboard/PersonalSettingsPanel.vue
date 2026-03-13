<template>
  <div class="panel settings-panel">
    <!-- 上部两列布局：账户信息 + 使用配置 -->
    <div class="top-row">
      <!-- 账户信息卡片 -->
      <div class="content-card half-width">
        <div class="card-header">
          <h3>账户信息</h3>
        </div>
        <div class="card-body">
          <div class="settings-form">
            <div class="form-item">
              <label>用户ID</label>
              <div class="form-value user-id">{{ userInfo?.id || '-' }}</div>
            </div>
            <div class="form-item">
              <label>用户名</label>
              <div class="form-value">{{ userInfo?.username || '-' }}</div>
            </div>
            <div class="form-item">
              <label>邮箱</label>
              <div class="form-value">{{ userInfo?.email || '-' }}</div>
            </div>
            <div class="form-item">
              <label>注册时间</label>
              <div class="form-value">{{ formatDateTime(userInfo?.created_at) }}</div>
            </div>
            <div class="form-item">
              <label>账户状态</label>
              <div class="form-value">
                <el-tag :type="userInfo?.status === 'active' ? 'success' : 'danger'" size="small">
                  {{ userInfo?.status === 'active' ? '正常' : '已禁用' }}
                </el-tag>
              </div>
            </div>
            <div class="form-item">
              <label>修改密码</label>
              <div class="form-value">
                <el-button class="change-pwd-btn" size="small" @click="showPasswordDialog = true">
                  <el-icon><Lock /></el-icon>
                  修改密码
                </el-button>
              </div>
            </div>
            <div class="form-item usage-limit-item">
              <div class="usage-metrics-line">
                <span class="usage-metric">
                  <span class="usage-key">账号总额度</span>
                  <span class="usage-value">{{ tokenAccountStats.total_count || 0 }}</span>
                </span>
                <span class="usage-divider">|</span>
                <span class="usage-metric">
                  <span class="usage-key">已使用账号</span>
                  <span class="usage-value">{{ (tokenAccountStats.expired_count || 0) + (tokenAccountStats.disabled_count || 0) }}</span>
                </span>
                <span class="usage-divider">|</span>
                <span class="usage-metric">
                  <span class="usage-key">接口调用限制</span>
                  <span class="usage-value">{{ userToken?.rate_limit_per_minute || 0 }} 次/分钟</span>
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 使用配置卡片 -->
      <div class="content-card half-width">
        <div class="card-header">
          <h3>使用配置</h3>
        </div>
        <div class="card-body">
          <div class="settings-form">
            <div class="form-item switch-item">
              <div class="switch-left">
                <div class="switch-label-row">
                  <el-icon class="switch-icon"><Document /></el-icon>
                  <span class="switch-label">对话引用附加文件</span>
                </div>
                <div class="switch-hint">开启后发起对话时会自动将附加文件的内容添加到上下文中，方便模型了解项目信息</div>
              </div>
              <div class="switch-right">
                <el-switch
                  v-model="prefixEnabled"
                  :loading="prefixLoading"
                  :disabled="prefixLoading"
                  @change="handlePrefixChange"
                />
              </div>
            </div>

          </div>
        </div>
      </div>
    </div>

    <!-- 修改密码弹窗 -->
    <el-dialog
      v-model="showPasswordDialog"
      title="修改密码"
      width="480px"
      @close="resetPasswordForm"
    >
      <el-form
        ref="passwordFormRef"
        :model="passwordForm"
        :rules="passwordRules"
        label-width="100px"
      >
        <el-form-item label="当前密码" prop="old_password">
          <el-input
            v-model="passwordForm.old_password"
            type="password"
            placeholder="请输入当前密码"
            show-password
          />
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input
            v-model="passwordForm.new_password"
            type="password"
            placeholder="请输入新密码（8位以上，包含数字和字母）"
            show-password
          />
          <div class="form-tip">密码长度至少8位，必须包含数字和字母</div>
        </el-form-item>
        <el-form-item label="确认密码" prop="confirm_password">
          <el-input
            v-model="passwordForm.confirm_password"
            type="password"
            placeholder="请再次输入新密码"
            show-password
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showPasswordDialog = false">取消</el-button>
          <el-button type="primary" :loading="passwordLoading" @click="handleChangePassword">
            确认修改
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Lock, Document } from '@element-plus/icons-vue'
import { userAuthAPI } from '@/api'

defineProps({
  userInfo: {
    type: Object,
    default: null
  },
  userToken: {
    type: Object,
    default: null
  },
  tokenAccountStats: {
    type: Object,
    default: () => ({ total_count: 0, available_count: 0, disabled_count: 0, expired_count: 0 })
  }
})

// ==================== 用户设置相关 ====================
const prefixEnabled = ref(true) // 是否引用附加文件，默认开启
const prefixLoading = ref(false)

// 加载用户设置
const loadUserSettings = async () => {
  try {
    const settings = await userAuthAPI.getSettings()
    // 默认为true，如果后端返回了值则使用后端的值
    prefixEnabled.value = settings?.prefix_enabled !== false
  } catch (error) {
    console.error('加载用户设置失败:', error)
    // 加载失败时使用默认值（开启）
    prefixEnabled.value = true
  }
}

// 处理附加文件引用开关变化
const handlePrefixChange = async (value) => {
  if (prefixLoading.value) return // 防止重复点击

  prefixLoading.value = true
  try {
    await userAuthAPI.updateSettings({
      prefix_enabled: value
    })
    ElMessage.success(value ? '附加文件引用已开启' : '附加文件引用已关闭')
  } catch (error) {
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      ElMessage.error(error.message || '更新设置失败')
    }
    // 恢复开关状态
    prefixEnabled.value = !value
  } finally {
    prefixLoading.value = false
  }
}

// 组件挂载时加载设置
onMounted(() => {
  loadUserSettings()
})

// 格式化日期时间
const formatDateTime = (dateStr) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

// 修改密码相关
const showPasswordDialog = ref(false)
const passwordLoading = ref(false)
const passwordFormRef = ref(null)
const passwordForm = ref({
  old_password: '',
  new_password: '',
  confirm_password: ''
})

// 密码复杂度验证
const validatePassword = (rule, value, callback) => {
  if (!value) {
    callback(new Error('请输入新密码'))
  } else if (value.length < 8) {
    callback(new Error('密码长度不能少于8位'))
  } else if (!/[a-zA-Z]/.test(value)) {
    callback(new Error('密码必须包含字母'))
  } else if (!/[0-9]/.test(value)) {
    callback(new Error('密码必须包含数字'))
  } else {
    callback()
  }
}

// 确认密码验证
const validateConfirmPassword = (rule, value, callback) => {
  if (!value) {
    callback(new Error('请再次输入新密码'))
  } else if (value !== passwordForm.value.new_password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const passwordRules = {
  old_password: [
    { required: true, message: '请输入当前密码', trigger: 'blur' }
  ],
  new_password: [
    { required: true, validator: validatePassword, trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

// 重置密码表单
const resetPasswordForm = () => {
  passwordForm.value = {
    old_password: '',
    new_password: '',
    confirm_password: ''
  }
  passwordFormRef.value?.resetFields()
}

// 提交修改密码
const handleChangePassword = async () => {
  try {
    await passwordFormRef.value?.validate()
    passwordLoading.value = true

    await userAuthAPI.changePassword({
      old_password: passwordForm.value.old_password,
      new_password: passwordForm.value.new_password
    })

    ElMessage.success('密码修改成功')
    showPasswordDialog.value = false
    resetPasswordForm()
  } catch (error) {
    // 如果错误已在拦截器中处理，不重复显示
    if (!error.handled && !error.silent) {
      ElMessage.error(error.message || '密码修改失败')
    }
  } finally {
    passwordLoading.value = false
  }
}
</script>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* 上部两列布局 */
.top-row {
  display: flex;
  gap: 20px;
}

.content-card {
  background: var(--card, oklch(1 0 0));
  border-radius: var(--radius-xl, 14px);
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
}

.content-card.half-width {
  flex: 1;
  min-width: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.card-header h3 {
  font-size: 16px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  margin: 0;
}

.card-body {
  padding: 16px 20px;
}

/* 设置表单 */
.settings-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-item {
  display: flex;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.form-item:last-child {
  border-bottom: none;
}

.form-item label {
  width: 100px;
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  flex-shrink: 0;
}

.form-value {
  font-size: 14px;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  font-weight: 500;
}

.form-value.highlight {
  color: var(--primary, oklch(0.21 0.006 285.885));
  font-weight: 600;
}

.form-value.user-id {
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Fira Mono', 'Droid Sans Mono', monospace;
}

/* 修改密码按钮 */
.change-pwd-btn {
  background: oklch(0.45 0.02 260) !important;
  border: none !important;
  border-radius: var(--radius-md, 8px) !important;
  color: #fff !important;
  font-weight: 500 !important;
  padding: 6px 14px !important;
  transition: all 0.2s ease !important;
}

.change-pwd-btn:hover {
  background: oklch(0.38 0.02 260) !important;
  transform: translateY(-1px);
}

.change-pwd-btn .el-icon {
  margin-right: 4px;
}

/* 开关项样式 */
.switch-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding: 16px 0 !important;
}

.switch-left {
  flex: 1;
  min-width: 0;
  padding-right: 16px;
}

.switch-label-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
}

.switch-icon {
  font-size: 16px;
  color: var(--primary, oklch(0.21 0.006 285.885));
}

.switch-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.switch-hint {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  line-height: 1.5;
}

.switch-right {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  padding-top: 2px;
}

.usage-limit-item {
  align-items: center;
  border-bottom: none !important;
  padding: 12px 0 4px !important;
}

.usage-metrics-line {
  display: flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}

.usage-metric {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
}

.usage-key {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.usage-value {
  color: var(--foreground, oklch(0.141 0.005 285.823));
  font-weight: 600;
}

.usage-divider {
  color: var(--border, oklch(0.92 0.004 286.32));
}


/* 空提示样式 */
.empty-hint {
  justify-content: space-between;
  align-items: flex-start;
}

.empty-text {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

/* 表单提示文字 */
.form-tip {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  margin-top: 6px;
  line-height: 1.4;
}

/* 对话框底部按钮区域 */
.dialog-footer {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin: 0;
  padding: 0;
}

/* 对话框样式优化 */
:deep(.el-dialog) {
  border-radius: var(--radius-xl, 14px);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.1), 0 2px 6px rgba(0, 0, 0, 0.08);
}

:deep(.el-dialog__header) {
  padding: 24px 24px 16px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

:deep(.el-dialog__body) {
  padding: 24px;
}

:deep(.el-dialog__footer) {
  padding: 16px 24px 24px;
  border-top: 1px solid var(--el-border-color-lighter);
}

/* 表单样式优化 */
:deep(.el-form-item) {
  margin-bottom: 20px;
}

:deep(.el-form-item__label) {
  font-weight: 500;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

:deep(.el-input) {
  width: 100%;
}

:deep(.el-button) {
  border-radius: var(--radius-md, 8px);
  font-weight: 500;
}

@media (max-width: 1100px) {
  .top-row {
    flex-direction: column;
  }

  .usage-metrics-line {
    flex-wrap: wrap;
    white-space: normal;
  }
}
</style>
