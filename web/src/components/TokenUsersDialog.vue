<template>
  <el-dialog
    :model-value="modelValue"
    :title="`TOKEN使用用户 - ${token?.token ? maskToken(token.token) : ''}`"
    width="900px"
    @update:model-value="$emit('update:modelValue', $event)"
    @open="loadUsers"
  >
    <div class="users-content" v-loading="loading">
      <!-- 用户统计信息 -->
      <div class="users-summary" v-if="!loading">
        <el-alert
          :title="`当前共有 ${users.length} 个用户正在使用此TOKEN`"
          type="info"
          :closable="false"
          show-icon
        />
      </div>

      <!-- 用户列表 -->
      <div class="users-list" v-if="users.length > 0">
        <el-table :data="users" stripe>
          <el-table-column prop="user_token_name" label="用户令牌名称" min-width="150">
            <template #default="{ row }">
              <div class="token-name">
                <el-icon><Key /></el-icon>
                <span>{{ row.user_token_name || '未命名' }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column prop="user_token" label="用户令牌" min-width="220">
            <template #default="{ row }">
              <div class="token-value">
                <code>{{ maskToken(row.user_token) }}</code>
                <el-button
                  type="text"
                  size="small"
                  @click="copyToken(row.user_token)"
                  title="复制完整令牌"
                >
                  <el-icon><CopyDocument /></el-icon>
                </el-button>
              </div>
            </template>
          </el-table-column>

          <el-table-column prop="last_used_at" label="最后使用时间" width="180">
            <template #default="{ row }">
              <div class="time-info" v-if="row.last_used_at">
                <el-icon><Clock /></el-icon>
                <span>{{ formatDate(row.last_used_at) }}</span>
              </div>
              <span v-else class="text-muted">未使用</span>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 空状态 -->
      <div class="empty-state" v-else-if="!loading">
        <el-empty description="当前没有用户使用此TOKEN" />
      </div>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="$emit('update:modelValue', false)">关闭</el-button>
        <el-button type="primary" @click="refreshUsers" :loading="loading">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { formatDate, copyToClipboard } from '@/utils'
import { tokenAPI } from '@/api'

const props = defineProps({
  modelValue: Boolean,
  token: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['update:modelValue'])

// 响应式数据
const loading = ref(false)
const users = ref([])

// 计算属性
const tokenDisplay = computed(() => {
  return props.token?.token ? maskToken(props.token.token) : ''
})

// 方法
const maskToken = (token) => {
  if (!token) return ''
  if (token.length <= 16) return token
  return token.substring(0, 8) + '****' + token.substring(token.length - 8)
}

const copyToken = async (token) => {
  try {
    await copyToClipboard(token)
    ElMessage.success('令牌已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

const loadUsers = async () => {
  if (!props.token?.id) return
  
  loading.value = true
  try {
    const response = await tokenAPI.getTokenUsers(props.token.id)
    users.value = response || []
  } catch (error) {
    console.error('加载TOKEN使用用户失败:', error)
    ElMessage.error('加载用户列表失败')
    users.value = []
  } finally {
    loading.value = false
  }
}

const refreshUsers = () => {
  loadUsers()
}

// 监听token变化
watch(() => props.token, (newToken) => {
  if (newToken) {
    users.value = []
  }
})
</script>

<style scoped>
.users-content {
  min-height: 300px;
}

.users-summary {
  margin-bottom: 20px;
}

.users-list {
  margin-top: 20px;
}

.token-name {
  display: flex;
  align-items: center;
  gap: 8px;
}

.token-value {
  display: flex;
  align-items: center;
  gap: 8px;
}

.token-value code {
  background: var(--el-fill-color-light);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 12px;
}

.time-info {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.text-muted {
  color: var(--el-text-color-placeholder);
  font-style: italic;
}

.empty-state {
  padding: 40px 0;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

/* 对话框样式优化 */
:deep(.el-dialog) {
  border-radius: 12px;
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

/* 表格样式优化 */
:deep(.el-table) {
  border-radius: 8px;
  overflow: hidden;
}

:deep(.el-table th) {
  background-color: var(--el-fill-color-light);
  font-weight: 600;
}

:deep(.el-table td) {
  padding: 12px 0;
}

/* 警告框样式 */
:deep(.el-alert) {
  border-radius: 8px;
  border: none;
  background: linear-gradient(135deg, #e3f2fd 0%, #f3e5f5 100%);
}
</style>
