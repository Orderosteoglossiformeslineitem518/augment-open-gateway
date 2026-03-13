<template>
  <el-dialog
    v-model="visible"
    title="个人设置"
    width="450px"
    :before-close="handleClose"
    class="user-settings-dialog"
  >
    <div class="settings-form">
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="100px"
        label-position="left"
        size="default"
        class="compact-form"
      >
        <el-form-item label="用户名" prop="username">
          <el-input
            v-model="form.username"
            placeholder="请输入新用户名"
            :prefix-icon="User"
            clearable
          />
        </el-form-item>

        <el-divider content-position="center" class="password-divider">
          <span class="divider-text">修改密码</span>
        </el-divider>

        <el-form-item label="当前密码" prop="oldPassword">
          <el-input
            v-model="form.oldPassword"
            type="password"
            placeholder="请输入当前密码"
            :prefix-icon="Lock"
            show-password
            clearable
          />
        </el-form-item>

        <el-form-item label="新密码" prop="newPassword">
          <el-input
            v-model="form.newPassword"
            type="password"
            placeholder="请输入新密码（至少6位）"
            :prefix-icon="Lock"
            show-password
            clearable
          />
        </el-form-item>

        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            placeholder="请再次输入新密码"
            :prefix-icon="Lock"
            show-password
            clearable
          />
        </el-form-item>
      </el-form>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="handleClose" :disabled="loading">
          取消
        </el-button>
        <el-button 
          type="primary" 
          @click="handleSubmit"
          :loading="loading"
        >
          保存
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
import { useAuthStore } from '@/store'
import { authAPI } from '@/api'

// Props
const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  }
})

// Emits
const emit = defineEmits(['update:modelValue', 'success'])

// Store
const authStore = useAuthStore()

// Refs
const formRef = ref()
const loading = ref(false)

// Computed
const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

// Form data
const form = reactive({
  username: '',
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

// Form rules
const rules = reactive({
  username: [
    { min: 2, max: 50, message: '用户名长度在 2 到 50 个字符', trigger: 'blur' }
  ],
  oldPassword: [
    { 
      validator: (rule, value, callback) => {
        if (form.newPassword && !value) {
          callback(new Error('修改密码时必须输入当前密码'))
        } else {
          callback()
        }
      }, 
      trigger: 'blur' 
    }
  ],
  newPassword: [
    { min: 6, message: '新密码长度不能少于6位', trigger: 'blur' }
  ],
  confirmPassword: [
    { 
      validator: (rule, value, callback) => {
        if (form.newPassword && value !== form.newPassword) {
          callback(new Error('两次输入的密码不一致'))
        } else {
          callback()
        }
      }, 
      trigger: 'blur' 
    }
  ]
})

// Watch dialog visibility
watch(visible, (newVal) => {
  if (newVal) {
    resetForm()
    // 初始化用户名
    if (authStore.user?.username) {
      form.username = authStore.user.username
    }
  }
})

// Methods
const resetForm = () => {
  if (formRef.value) {
    formRef.value.resetFields()
  }
  Object.assign(form, {
    username: '',
    oldPassword: '',
    newPassword: '',
    confirmPassword: ''
  })
}

const handleClose = () => {
  if (loading.value) return
  visible.value = false
  resetForm()
}

const handleSubmit = async () => {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
    
    // 检查是否有修改
    const hasUsernameChange = form.username && form.username !== authStore.user?.username
    const hasPasswordChange = form.newPassword
    
    if (!hasUsernameChange && !hasPasswordChange) {
      ElMessage.warning('请至少修改用户名或密码')
      return
    }

    loading.value = true

    // 构建请求数据
    const updateData = {}
    if (hasUsernameChange) {
      updateData.username = form.username
    }
    if (hasPasswordChange) {
      updateData.old_password = form.oldPassword
      updateData.new_password = form.newPassword
    }

    // 调用API
    const response = await authAPI.updateProfile(updateData)
    
    // 更新store中的用户信息
    if (response && response.username) {
      authStore.user = { ...authStore.user, ...response }
    }

    ElMessage.success('个人信息更新成功')
    emit('success')
    handleClose()
  } catch (error) {
    console.error('更新个人信息失败:', error)
    // 只有当错误未被拦截器处理过时才显示错误消息，避免重复提示
    if (!error.handled && !error.silent) {
      ElMessage.error(error.message || '更新失败，请重试')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.user-settings-dialog :deep(.el-dialog) {
  border-radius: 12px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
  background: #ffffff;
}

.user-settings-dialog :deep(.el-dialog__header) {
  padding: 24px;
  border-bottom: 1px solid #f3f4f6;
  background: #ffffff;
}

.user-settings-dialog :deep(.el-dialog__title) {
  font-size: 18px;
  font-weight: 600;
  color: #111827;
  text-align: center;
}

.user-settings-dialog :deep(.el-dialog__body) {
  padding: 24px;
  background: #ffffff;
}

.compact-form :deep(.el-form-item) {
  margin-bottom: 20px;
}

.compact-form :deep(.el-form-item__label) {
  font-weight: 500;
  color: #374151;
  font-size: 14px;
}

.compact-form :deep(.el-input__wrapper) {
  border-radius: 6px;
  transition: all 0.2s ease;
}

.password-divider {
  margin: 24px 0;
}

.password-divider :deep(.el-divider__text) {
  background: #ffffff;
  padding: 0 12px;
}

.compact-form :deep(.el-form-item__error) {
  font-size: 12px;
  margin-top: 4px;
  padding-left: 0;
}

.compact-form :deep(.el-input.is-error .el-input__wrapper) {
  box-shadow: 0 0 0 1px #f56c6c;
}

.compact-form :deep(.el-input.is-error .el-input__wrapper:focus) {
  box-shadow: 0 0 0 2px #f56c6c;
}

.divider-text {
  font-size: 14px;
  color: #409eff;
  font-weight: 500;
}

.dialog-footer {
  display: flex;
  justify-content: center;
  gap: 16px;
  padding: 0 24px 24px 24px;
  border-top: 1px solid #f3f4f6;
  background: #ffffff;
}

.dialog-footer .el-button {
  border-radius: 6px;
  padding: 8px 20px;
  font-weight: 500;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .user-settings-dialog :deep(.el-dialog) {
    width: 95% !important;
    margin: 5vh auto;
  }

  .compact-form :deep(.el-form-item__label) {
    width: 80px !important;
    font-size: 13px;
  }

  .compact-form :deep(.el-form-item__content) {
    max-width: 220px;
  }

  .compact-form :deep(.el-form-item) {
    margin-bottom: 20px;
  }

  .dialog-footer {
    padding: 16px 20px 20px;
  }

  .dialog-footer .el-button {
    padding: 8px 20px;
    min-width: 70px;
  }
}
</style>
