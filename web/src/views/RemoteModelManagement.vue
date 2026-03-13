<template>
  <div class="remote-model-page">
    <!-- 操作栏 -->
    <el-card class="filter-card">
      <div class="action-bar">
        <div class="action-left">
          <el-tag type="info" size="large">共 {{ models.length }} 个模型</el-tag>
          <span class="sync-tip">模型数据从远程API同步</span>
        </div>
        <div class="action-right">
          <el-button type="primary" @click="syncModels" :loading="syncing">
            <el-icon><Refresh /></el-icon>
            同步远程模型
          </el-button>
        </div>
      </div>
    </el-card>

    <!-- 模型列表 -->
    <el-card class="table-card">
      <el-table
        :data="models"
        v-loading="loading"
        empty-text="暂无远程模型数据，请先同步"
        stripe
      >
        <el-table-column prop="model_name" label="模型名称" width="200">
          <template #default="{ row }">
            <div class="model-name-cell">
              <span class="model-name">{{ row.model_name }}</span>
              <el-tag v-if="row.is_default" type="warning" size="small" effect="dark" class="default-tag">默认</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.description" class="desc-text">{{ row.description }}</span>
            <span v-else class="no-description">-</span>
          </template>
        </el-table-column>
        <el-table-column label="透传" width="100" align="center">
          <template #default="{ row }">
            <div class="passthrough-cell">
              <el-tag
                :type="getPassthroughStatusType(row)"
                size="small"
              >
                {{ getPassthroughStatusText(row) }}
              </el-tag>
              <span v-if="row.allow_shared_token_passthrough && row.passthrough_expires_at" class="expires-hint">
                {{ formatShortDate(row.passthrough_expires_at) }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="synced_at" label="同步时间" width="170">
          <template #default="{ row }">
            {{ formatDate(row.synced_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="300" fixed="right">
          <template #default="{ row }">
            <div class="action-cell">
              <el-switch
                v-model="row.allow_shared_token_passthrough"
                @change="(val) => handlePassthroughChange(row, val)"
                size="small"
                inline-prompt
                active-text="开"
                inactive-text="关"
              />
              <el-button
                v-if="row.allow_shared_token_passthrough"
                type="warning"
                size="small"
                plain
                @click="openExpiresDialog(row)"
              >
                截止日期
              </el-button>
              <el-button
                v-if="!row.is_default"
                type="primary"
                size="small"
                plain
                @click="setDefaultModel(row)"
              >
                设为默认
              </el-button>
              <el-button
                type="danger"
                size="small"
                plain
                @click="deleteModel(row)"
              >
                删除
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 截止日期设置对话框 -->
    <el-dialog
      v-model="expiresDialogVisible"
      title="设置透传截止日期"
      width="400px"
      :close-on-click-modal="false"
    >
      <div class="expires-dialog-content">
        <p class="expires-model-name">模型：<strong>{{ expiresEditRow?.model_name }}</strong></p>
        <el-date-picker
          v-model="expiresEditValue"
          type="datetime"
          placeholder="留空表示永久有效"
          format="YYYY-MM-DD HH:mm"
          value-format="YYYY-MM-DDTHH:mm:ss"
          clearable
          :disabled-date="disablePastDate"
          style="width: 100%"
        />
        <p class="expires-hint-text">留空或清除日期表示永久有效，只能选择未来的时间</p>
      </div>
      <template #footer>
        <el-button @click="expiresDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveExpiresDate">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { remoteModelAPI } from '@/api'

// 响应式数据
const loading = ref(false)
const syncing = ref(false)
const models = ref([])
const expiresDialogVisible = ref(false)
const expiresEditRow = ref(null)
const expiresEditValue = ref('')

// 获取透传状态类型
const getPassthroughStatusType = (row) => {
  if (!row.allow_shared_token_passthrough) return 'info'
  if (row.passthrough_expires_at) {
    const expires = new Date(row.passthrough_expires_at)
    if (expires < new Date()) return 'danger'
  }
  return 'success'
}

// 获取透传状态文本
const getPassthroughStatusText = (row) => {
  if (!row.allow_shared_token_passthrough) return '未开启'
  if (row.passthrough_expires_at) {
    const expires = new Date(row.passthrough_expires_at)
    if (expires < new Date()) return '已过期'
  }
  return '生效中'
}

// 格式化日期
const formatDate = (dateString) => {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleString('zh-CN')
}

// 格式化短日期（用于透传状态列）
const formatShortDate = (dateString) => {
  if (!dateString) return ''
  const d = new Date(dateString)
  return `${(d.getMonth()+1).toString().padStart(2,'0')}-${d.getDate().toString().padStart(2,'0')} ${d.getHours().toString().padStart(2,'0')}:${d.getMinutes().toString().padStart(2,'0')}`
}

// 加载模型列表
const loadModels = async () => {
  try {
    loading.value = true
    const response = await remoteModelAPI.getList()
    models.value = response.data || response || []
  } catch (error) {
    console.error('加载远程模型列表失败:', error)
    if (!error.handled && !error.silent) {
      const message = error.response?.data?.msg || error.message || '加载远程模型列表失败'
      ElMessage.error(message)
    }
    models.value = []
  } finally {
    loading.value = false
  }
}

// 同步远程模型
const syncModels = async () => {
  try {
    syncing.value = true
    const response = await remoteModelAPI.sync()
    const msg = response.msg || '同步成功'
    const newCount = response.data?.new_count || 0
    ElMessage.success(`${msg}，新增 ${newCount} 个模型`)
    await loadModels()
  } catch (error) {
    console.error('同步远程模型失败:', error)
    if (!error.handled && !error.silent) {
      const message = error.response?.data?.msg || error.message || '同步远程模型失败'
      ElMessage.error(message)
    }
  } finally {
    syncing.value = false
  }
}

// 处理透传开关变更
const handlePassthroughChange = async (row, val) => {
  try {
    await remoteModelAPI.updatePassthrough(row.id, {
      allow_shared_token_passthrough: val,
      passthrough_expires_at: row.passthrough_expires_at || ''
    })
    ElMessage.success(val ? '已开启共享账号透传' : '已关闭共享账号透传')
  } catch (error) {
    // 回滚
    row.allow_shared_token_passthrough = !val
    if (!error.handled && !error.silent) {
      const message = error.response?.data?.msg || '更新透传配置失败'
      ElMessage.error(message)
    }
  }
}

// 禁用过去的日期
const disablePastDate = (date) => {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  return date.getTime() < today.getTime()
}

// 打开截止日期对话框
const openExpiresDialog = (row) => {
  expiresEditRow.value = row
  expiresEditValue.value = row.passthrough_expires_at || ''
  expiresDialogVisible.value = true
}

// 保存截止日期
const saveExpiresDate = async () => {
  const row = expiresEditRow.value
  if (!row) return

  // 校验不能选择过去的时间
  if (expiresEditValue.value) {
    const selected = new Date(expiresEditValue.value)
    if (selected <= new Date()) {
      ElMessage.warning('截止日期不能是过去的时间，请选择未来的时间')
      return
    }
  }

  try {
    await remoteModelAPI.updatePassthrough(row.id, {
      allow_shared_token_passthrough: row.allow_shared_token_passthrough,
      passthrough_expires_at: expiresEditValue.value || ''
    })
    row.passthrough_expires_at = expiresEditValue.value || null
    ElMessage.success(expiresEditValue.value ? '已设置透传截止日期' : '已设置为永久有效')
    expiresDialogVisible.value = false
  } catch (error) {
    if (!error.handled && !error.silent) {
      const message = error.response?.data?.msg || '更新截止日期失败'
      ElMessage.error(message)
    }
  }
}

// 设置默认模型
const setDefaultModel = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定将「${row.model_name}」设为默认模型吗？客户端获取模型列表时将返回此模型为默认。`,
      '设置默认模型',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'info'
      }
    )

    await remoteModelAPI.setDefault(row.id)
    ElMessage.success(`已将「${row.model_name}」设为默认模型`)
    await loadModels()
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      const message = error.response?.data?.msg || '设置默认模型失败'
      ElMessage.error(message)
    }
  }
}

// 删除模型
const deleteModel = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定删除远程模型「${row.model_name}」吗？此操作不可恢复。`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    await remoteModelAPI.delete(row.id)
    ElMessage.success('模型删除成功')
    await loadModels()

  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      const message = error.response?.data?.msg || '删除模型失败'
      ElMessage.error(message)
    }
  }
}

// 组件挂载时加载数据
onMounted(() => {
  loadModels()
})
</script>

<style scoped>
.remote-model-page {
  width: 100%;
  min-height: calc(100vh - 140px);
}

.filter-card {
  margin-bottom: 24px;
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.action-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.action-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sync-tip {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.table-card {
  margin-bottom: 24px;
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.1);
  transition: all 0.3s ease;
}

.table-card:hover {
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12);
}

.model-name-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.model-name {
  font-family: 'Monaco', 'Menlo', 'Consolas', monospace;
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.default-tag {
  flex-shrink: 0;
}

.desc-text {
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.no-description {
  color: var(--el-text-color-placeholder);
}

.action-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: nowrap;
}

.action-cell .el-button {
  padding: 5px 10px;
}

.passthrough-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.expires-hint {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  line-height: 1.2;
}

.expires-dialog-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.expires-model-name {
  margin: 0;
  font-size: 14px;
  color: var(--el-text-color-regular);
}

.expires-hint-text {
  margin: 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
</style>
