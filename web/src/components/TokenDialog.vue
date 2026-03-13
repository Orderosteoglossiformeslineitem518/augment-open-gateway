<template>
  <el-dialog
    :model-value="modelValue"
    :title="isEdit ? '编辑Token' : '创建Token'"
    width="600px"
    @update:model-value="$emit('update:modelValue', $event)"
    @close="handleClose"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="120px"
      @submit.prevent="handleSubmit"
    >
      <!-- 提交方式提示 -->
      <el-alert
        type="info"
        :closable="false"
        style="margin-bottom: 16px;"
      >
        <template #default>
          <span style="font-size: 13px;">
            提交方式：<strong>AuthSession</strong> 或 <strong>TOKEN + 租户地址</strong>（二选一）
          </span>
        </template>
      </el-alert>

      <el-form-item label="AuthSession" prop="auth_session">
        <el-input
          v-model="form.auth_session"
          type="textarea"
          :rows="3"
          placeholder="输入AuthSession，系统将自动获取TOKEN、租户地址等信息，必须以.eJ或.ey开头"
          maxlength="1000"
          show-word-limit
        />
        <div class="form-tip">
          可选填写 AuthSession 信息，系统将自动获取相关信息
        </div>
      </el-form-item>

      <el-form-item label="TOKEN" prop="token">
        <el-input
          v-model="form.token"
          placeholder="输入TOKEN值（64位）"
          maxlength="64"
          show-word-limit
          :disabled="isEdit"
        />
        <div class="form-tip">TOKEN值必须唯一，创建后不可修改</div>
      </el-form-item>

      <el-form-item label="租户地址" prop="tenant_address">
        <el-input
          v-model="form.tenant_address"
          placeholder="输入租户地址，必须以https://开头"
          maxlength="255"
        />
        <div class="form-tip">
          请输入完整的租户地址，包含协议
        </div>
      </el-form-item>

      <el-form-item label="PortalUrl" prop="portal_url">
        <el-input
          v-model="form.portal_url"
          placeholder="订阅地址（可选），使用AuthSession时会自动获取"
          maxlength="500"
        />
      </el-form-item>

      <el-form-item label="积分类型" prop="account_type" v-if="!isEdit">
        <el-radio-group v-model="form.account_type">
          <el-radio label="34000">34000</el-radio>
          <el-radio label="30000">30000</el-radio>
          <el-radio label="24000">24000</el-radio>
          <el-radio label="4000">4000</el-radio>
          <el-radio label="0">0</el-radio>
        </el-radio-group>
      </el-form-item>

      <el-form-item label="代理地址" prop="replace_proxy_url" v-if="!isEdit">
        <el-input
          v-model="form.replace_proxy_url"
          placeholder="如：https://xxx.deno.dev/（可选）"
          maxlength="255"
        />
      </el-form-item>

      <el-form-item label="状态" prop="status" v-if="isEdit">
        <el-radio-group v-model="form.status">
          <el-radio label="active">正常</el-radio>
          <el-radio label="expired">已过期</el-radio>
          <el-radio label="disabled">已禁用</el-radio>
        </el-radio-group>
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="handleClose">取消</el-button>
        <el-button type="primary" :loading="loading" @click="handleSubmit">
          {{ isEdit ? '更新' : '创建' }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { useTokenStore } from '@/store'

const props = defineProps({
  modelValue: Boolean,
  token: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['update:modelValue', 'success'])

const tokenStore = useTokenStore()
const formRef = ref()
const loading = ref(false)

// 表单数据
const form = ref({
  token: '',
  tenant_address: '',
  replace_proxy_url: '',
  auth_session: '',
  portal_url: '',
  account_type: '30000', // 默认30000积分
  max_requests: 30000, // 默认30000
  expires_at: null, // 永不过期
  status: 'active',
  enhanced_enabled: false // 默认关闭增强功能
})

// 计算属性
const isEdit = computed(() => !!props.token?.id)

// 表单验证规则
const rules = {
  auth_session: [
    {
      validator: (_, value, callback) => {
        if (!value) {
          callback()
          return
        }

        const trimmedValue = value.trim()
        if (!trimmedValue.startsWith('.eJ') && !trimmedValue.startsWith('.ey')) {
          callback(new Error('AuthSession必须以.eJ或.ey开头'))
          return
        }

        callback()
      },
      trigger: 'blur'
    }
  ],
  token: [
    {
      validator: (_, value, callback) => {
        if (!value) {
          callback()
          return
        }

        const trimmedValue = value.trim()
        if (trimmedValue.length > 0 && trimmedValue.length !== 64) {
          callback(new Error('TOKEN必须为64位字符串'))
          return
        }

        callback()
      },
      trigger: 'blur'
    }
  ],
  tenant_address: [
    {
      validator: (_, value, callback) => {
        if (!value) {
          callback()
          return
        }

        // 自动补充末尾的斜杠
        let normalizedValue = value.trim()
        if (!normalizedValue.endsWith('/')) {
          normalizedValue = normalizedValue + '/'
          // 更新表单值
          form.value.tenant_address = normalizedValue
        }

        // 验证格式
        if (!normalizedValue.startsWith('https://')) {
          callback(new Error('租户地址必须以https://开头'))
          return
        }

        callback()
      },
      trigger: 'blur'
    }
  ],
  proxy_url: [
    {
      validator: (_, value, callback) => {
        if (!value) {
          callback()
          return
        }

        const trimmedValue = value.trim()
        if (!trimmedValue.startsWith('https://')) {
          callback(new Error('代理地址必须以https://开头'))
          return
        }

        callback()
      },
      trigger: 'blur'
    }
  ],
  replace_proxy_url: [
    {
      validator: (_, value, callback) => {
        if (!value) {
          callback()
          return
        }

        const trimmedValue = value.trim()
        if (!trimmedValue.startsWith('https://')) {
          callback(new Error('替换代理地址必须以https://开头'))
          return
        }

        // 验证域名白名单
        try {
          const url = new URL(trimmedValue)
          const hostname = url.hostname
          const allowedDomains = ['.deno.dev', '.deno.net', '.vercel.app', '.supabase.co']
          const isAllowed = allowedDomains.some(domain => hostname.endsWith(domain))
          if (!isAllowed) {
            callback(new Error('只支持 .deno.dev、.deno.net、.vercel.app、.supabase.co 域名'))
            return
          }
        } catch (e) {
          callback(new Error('请输入有效的URL地址'))
          return
        }

        callback()
      },
      trigger: 'blur'
    }
  ]
}

// 时间格式转换函数
const formatDateTime = (dateTime) => {
  if (!dateTime) return null

  // 如果已经是正确格式，直接返回
  if (typeof dateTime === 'string' && /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(dateTime)) {
    return dateTime
  }

  // 处理ISO格式或其他格式的时间
  const date = new Date(dateTime)
  if (isNaN(date.getTime())) {
    return null
  }

  // 转换为后端期望的格式: 2006-01-02 15:04:05
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
}

// 方法
const resetForm = () => {
  form.value = {
    token: '',
    tenant_address: '',
    replace_proxy_url: '',
    auth_session: '',
    portal_url: '',
    account_type: '30000', // 默认30000积分
    max_requests: 30000, // 默认30000
    expires_at: null, // 永不过期
    status: 'active',
    enhanced_enabled: false
  }

  nextTick(() => {
    formRef.value?.clearValidate()
  })
}

// 监听props变化
watch(() => props.token, (newToken) => {
  if (newToken) {
    // 编辑模式，填充表单
    form.value = {
      token: newToken.token || '',
      tenant_address: newToken.tenant_address || '',
      replace_proxy_url: '',
      auth_session: newToken.auth_session || '',
      portal_url: newToken.portal_url || '',
      account_type: String(newToken.max_requests) || '30000',
      max_requests: newToken.max_requests || 30000,
      expires_at: formatDateTime(newToken.expires_at) || null,
      status: newToken.status || 'active',
      enhanced_enabled: newToken.enhanced_enabled || false
    }
  } else {
    // 创建模式，重置表单
    resetForm()
  }
}, { immediate: true })

// 监听对话框显示状态
watch(() => props.modelValue, (visible) => {
  if (visible && !props.token) {
    resetForm()
  }
})

const handleClose = () => {
  emit('update:modelValue', false)
}

const handleSubmit = async () => {
  if (!formRef.value) return

  // 手动验证：Auth Session 或 (TOKEN + 租户地址) 必须填写其一
  const hasAuthSession = form.value.auth_session.trim().length > 0
  const hasToken = form.value.token.trim().length > 0
  const hasTenantAddress = form.value.tenant_address.trim().length > 0

  // 编辑模式下，如果已有TOKEN和租户地址，不需要验证
  if (!isEdit.value) {
    if (!hasAuthSession && (!hasToken || !hasTenantAddress)) {
      ElMessage.error('请填写AuthSession或(TOKEN+租户地址)')
      return
    }
  }

  try {
    await formRef.value.validate()

    loading.value = true

    // 根据账号类型设置 max_requests
    const parsedType = parseInt(form.value.account_type)
    const maxRequests = isNaN(parsedType) ? 30000 : parsedType

    // 提交前清理数据，移除首尾空格
    const submitData = {
      ...form.value,
      auth_session: form.value.auth_session.trim(),
      token: form.value.token.trim(),
      tenant_address: form.value.tenant_address.trim(),
      portal_url: form.value.portal_url.trim(),
      replace_proxy_url: form.value.replace_proxy_url.trim(),
      max_requests: maxRequests
    }

    // 确保时间格式正确
    if (submitData.expires_at) {
      submitData.expires_at = formatDateTime(submitData.expires_at)
    }

    if (isEdit.value) {
      await tokenStore.updateToken(props.token.id, submitData)
      ElMessage.success('Token更新成功')
    } else {
      await tokenStore.createToken(submitData)
      ElMessage.success('Token创建成功')
    }

    emit('success')
    handleClose()
  } catch (error) {
    if (error.errors) {
      // 表单验证错误
      return
    }
    ElMessage.error(isEdit.value ? 'Token更新失败' : 'Token创建失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* 对话框内容区域样式优化 */
:deep(.el-dialog) {
  border-radius: 12px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.1), 0 2px 6px rgba(0, 0, 0, 0.08);
}

:deep(.el-dialog__header) {
  padding: 24px 24px 16px 24px;
  border-bottom: 1px solid var(--border-color-extra-light);
  text-align: center;
}

:deep(.el-dialog__title) {
  font-weight: 600;
}

:deep(.el-dialog__body) {
  padding: 24px 24px 16px 24px;
  /* 确保左右边距完全对称 */
  margin: 0;
  box-sizing: border-box;
}

:deep(.el-dialog__footer) {
  padding: 16px 24px 24px 24px;
  border-top: 1px solid var(--border-color-extra-light);
  background-color: var(--bg-color-page);
}

/* 表单样式优化 */
:deep(.el-form) {
  /* 确保表单内容区域对称 */
  padding: 0;
  margin: 0;
}

:deep(.el-form-item) {
  margin-bottom: 20px;
}

:deep(.el-form-item__label) {
  font-weight: 500;
  color: var(--text-color-primary);
  line-height: 32px;
  padding-right: 12px;
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

:deep(.el-form-item__content) {
  /* 确保输入框区域对齐 */
  flex: 1;
  min-width: 0;
}

/* 输入框样式优化 */
:deep(.el-input),
:deep(.el-textarea),
:deep(.el-input-number),
:deep(.el-date-editor) {
  width: 100%;
}

:deep(.el-input__wrapper) {
  border-radius: 8px;
  box-shadow: 0 0 0 1px var(--el-border-color) !important;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

:deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--el-color-primary-light-5) !important;
}

:deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px var(--el-color-primary) !important;
}

:deep(.el-textarea__inner) {
  border-radius: 8px;
  box-shadow: 0 0 0 1px var(--el-border-color) !important;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

:deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px var(--el-color-primary-light-5) !important;
}

:deep(.el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px var(--el-color-primary) !important;
}

/* 表单提示文字 */
.form-tip {
  font-size: 12px;
  color: var(--text-color-secondary);
  margin-top: 6px;
  line-height: 1.4;
}

/* 对话框底部按钮区域 */
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin: 0;
  padding: 0;
}

:deep(.el-button) {
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

/* 响应式设计 */
@media (max-width: 768px) {
  :deep(.el-dialog) {
    margin: 5vh auto;
    width: 95% !important;
  }

  :deep(.el-dialog__header),
  :deep(.el-dialog__body),
  :deep(.el-dialog__footer) {
    padding-left: 16px;
    padding-right: 16px;
  }

  :deep(.el-form-item__label) {
    text-align: left;
    width: 100% !important;
    margin-bottom: 8px;
  }

  :deep(.el-form-item__content) {
    margin-left: 0 !important;
  }
}

/* 确保输入组件的背景色一致 */
:deep(.el-input-group__prepend) {
  background-color: var(--bg-color-page);
  border-radius: 8px 0 0 8px;
}

/* 数字输入框样式 */
:deep(.el-input-number) {
  width: 100%;
}

:deep(.el-input-number .el-input__wrapper) {
  padding-left: 11px;
  padding-right: 11px;
}

/* 日期选择器样式 */
:deep(.el-date-editor.el-input) {
  width: 100%;
}

/* 单选按钮组样式 */
:deep(.el-radio-group) {
  display: flex;
  gap: 16px;
}

:deep(.el-radio) {
  margin-right: 0;
}

:deep(.el-radio__label) {
  font-weight: 500;
}
</style>
