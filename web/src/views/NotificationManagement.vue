<template>
  <div class="notification-management-page">
    <!-- 筛选表单 -->
    <el-card class="filter-card">
      <el-form inline>
        <el-form-item>
          <el-button type="primary" @click="showCreateDialog">
            <el-icon><Plus /></el-icon>
            新增公告
          </el-button>
          <el-button @click="loadNotifications" :loading="loading">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 公告列表 -->
    <el-card class="content-card" v-loading="loading">

      <el-table :data="notifications" style="width: 100%; table-layout: fixed;">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="notification_id" label="通知ID" width="120">
          <template #default="{ row }">
            <span :title="row.notification_id">
              {{ row.notification_id ? row.notification_id.substring(0, 8) + '...' : '-' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="level" label="等级" width="100">
          <template #default="{ row }">
            <el-tag :type="getLevelType(row.level)">
              {{ getLevelText(row.level) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="message" label="公告内容" min-width="150" />
        <el-table-column prop="action_title" label="操作标题" width="120">
          <template #default="{ row }">
            {{ row.action_title || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="action_url" label="操作链接" width="200">
          <template #default="{ row }">
            <span v-if="row.action_url" :title="row.action_url">
              {{ row.action_url.length > 30 ? row.action_url.substring(0, 30) + '...' : row.action_url }}
            </span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="display_type" label="显示类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.display_type === 1 ? 'info' : 'warning'">
              {{ row.display_type === 1 ? 'TOAST' : 'BANNER' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="enabled" label="启用状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'">
              {{ row.enabled ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="read_count" label="已读数量" width="100">
          <template #default="{ row }">
            <span v-if="row.read_count !== undefined">{{ row.read_count }}</span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.updated_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="editNotification(row)" style="width: 50px;">
              编辑
            </el-button>
            <el-button
              :type="row.enabled ? 'warning' : 'success'"
              size="small"
              @click="toggleNotificationStatus(row)"
              :loading="statusLoadingMap.get(row.id) || false"
              style="width: 50px;"
            >
              {{ row.enabled ? '禁用' : '启用' }}
            </el-button>
            <el-button type="danger" size="small" @click="deleteNotification(row)" style="width: 50px;">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 空状态 -->
      <div v-if="!loading && notifications.length === 0" class="empty-state">
        <el-empty description="暂无公告数据">
          <el-button type="primary" @click="showCreateDialog">创建第一个公告</el-button>
        </el-empty>
      </div>
    </el-card>

    <!-- 创建/编辑公告对话框 -->
    <el-dialog
      :title="dialogTitle"
      v-model="dialogVisible"
      width="500px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="notificationFormRef"
        :model="notificationForm"
        :rules="formRules"
        label-width="100px"
      >
        <el-form-item label="通知ID" prop="notification_id">
          <el-input
            v-model="notificationForm.notification_id"
            placeholder="请输入通知ID（UUID格式，留空自动生成）"
            maxlength="36"
          >
            <template #append>
              <el-button @click="generateNewNotificationId" type="primary">生成新ID</el-button>
            </template>
          </el-input>
          <div style="margin-top: 5px; font-size: 12px; color: #999;">
            通知ID用于客户端识别公告，建议使用UUID格式。留空时系统会自动生成。
          </div>
        </el-form-item>
        <el-form-item label="公告等级" prop="level">
          <el-select v-model="notificationForm.level" placeholder="请选择公告等级" style="width: 100%">
            <el-option :value="1" label="信息">
              <el-tag type="success">信息</el-tag>
            </el-option>
            <el-option :value="2" label="警告">
              <el-tag type="warning">警告</el-tag>
            </el-option>
            <el-option :value="3" label="错误">
              <el-tag type="danger">错误</el-tag>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="公告内容" prop="message">
          <el-input
            v-model="notificationForm.message"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="请输入公告内容（最多35个字符）"
            maxlength="35"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="操作标题" prop="action_title">
          <el-input
            v-model="notificationForm.action_title"
            placeholder="请输入操作按钮标题（可选，最多50个字符）"
            maxlength="50"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="操作链接" prop="action_url">
          <el-input
            v-model="notificationForm.action_url"
            placeholder="请输入操作按钮链接（可选，最多255个字符）"
            maxlength="255"
          />
        </el-form-item>
        <el-form-item label="显示类型" prop="display_type">
          <el-select v-model="notificationForm.display_type" placeholder="请选择显示类型" style="width: 100%">
            <el-option :value="1" label="TOAST">
              <el-tag type="info">TOAST（吐司通知）</el-tag>
            </el-option>
            <el-option :value="2" label="BANNER">
              <el-tag type="warning">BANNER（横幅通知）</el-tag>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="启用状态" prop="enabled">
          <el-select v-model="notificationForm.enabled" placeholder="请选择启用状态" style="width: 100%">
            <el-option :value="false" label="禁用">
              <el-tag type="info">禁用（创建后需要手动启用）</el-tag>
            </el-option>
            <el-option :value="true" label="启用">
              <el-tag type="success">启用（将自动禁用其他公告）</el-tag>
            </el-option>
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="saveNotification" :loading="saving">
            {{ isEdit ? '更新' : '创建' }}
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { notificationAPI } from '@/api'

// 响应式数据
const loading = ref(false)
const saving = ref(false)
const notifications = ref([])
const dialogVisible = ref(false)
const isEdit = ref(false)
const currentNotificationId = ref(null)
const notificationFormRef = ref()
const statusLoadingMap = ref(new Map()) // 用于管理每个公告的状态切换加载状态

// 表单数据
const notificationForm = reactive({
  notification_id: '',
  level: 1,
  message: '',
  action_title: '',
  action_url: '',
  display_type: 2,
  enabled: false
})

// 表单验证规则
const formRules = {
  notification_id: [
    { max: 36, message: '通知ID不能超过36个字符', trigger: 'blur' }
  ],
  level: [
    { required: true, message: '请选择公告等级', trigger: 'change' }
  ],
  message: [
    { required: true, message: '请输入公告内容', trigger: 'blur' },
    { max: 35, message: '公告内容不能超过35个字符', trigger: 'blur' }
  ],
  action_title: [
    { max: 50, message: '操作标题不能超过50个字符', trigger: 'blur' }
  ],
  action_url: [
    { max: 255, message: '操作链接不能超过255个字符', trigger: 'blur' }
  ],
  display_type: [
    { required: true, message: '请选择显示类型', trigger: 'change' }
  ],
  enabled: [
    { required: true, message: '请选择启用状态', trigger: 'change' }
  ]
}

// 计算属性
const dialogTitle = computed(() => isEdit.value ? '编辑公告' : '新增公告')

// 获取等级类型
const getLevelType = (level) => {
  const types = { 1: 'success', 2: 'warning', 3: 'danger' }
  return types[level] || 'info'
}

// 获取等级文本
const getLevelText = (level) => {
  const texts = { 1: '信息', 2: '警告', 3: '错误' }
  return texts[level] || '未知'
}

// 格式化时间
const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

// 加载公告列表
const loadNotifications = async () => {
  try {
    loading.value = true
    const response = await notificationAPI.getList()
    notifications.value = response.list || []
  } catch (error) {
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      ElMessage.error('加载公告列表失败')
    }
  } finally {
    loading.value = false
  }
}

// 显示创建对话框
const showCreateDialog = () => {
  isEdit.value = false
  currentNotificationId.value = null
  notificationForm.notification_id = ''
  notificationForm.level = 1
  notificationForm.message = ''
  notificationForm.action_title = ''
  notificationForm.action_url = ''
  notificationForm.display_type = 2
  notificationForm.enabled = false
  dialogVisible.value = true
}

// 编辑公告
const editNotification = (notification) => {
  isEdit.value = true
  currentNotificationId.value = notification.id
  notificationForm.notification_id = notification.notification_id || ''
  notificationForm.level = notification.level
  notificationForm.message = notification.message
  notificationForm.action_title = notification.action_title || ''
  notificationForm.action_url = notification.action_url || ''
  notificationForm.display_type = notification.display_type || 2
  notificationForm.enabled = notification.enabled || false
  dialogVisible.value = true
}

// 保存公告
const saveNotification = async () => {
  try {
    await notificationFormRef.value.validate()
    saving.value = true

    if (isEdit.value) {
      await notificationAPI.update(currentNotificationId.value, notificationForm)
      ElMessage.success('公告更新成功')
    } else {
      await notificationAPI.create(notificationForm)
      ElMessage.success('公告创建成功')
    }

    dialogVisible.value = false
    loadNotifications()
  } catch (error) {
    // 只有当错误未被拦截器处理过时才显示错误消息，避免重复提示
    if (error.message && !error.handled) {
      ElMessage.error(error.message)
    }
  } finally {
    saving.value = false
  }
}

// 切换公告启用状态
const toggleNotificationStatus = async (notification) => {
  const action = notification.enabled ? '禁用' : '启用'
  const confirmMessage = notification.enabled
    ? `确定要禁用公告"${notification.message}"吗？`
    : `确定要启用公告"${notification.message}"吗？启用后将自动禁用其他公告。`

  try {
    await ElMessageBox.confirm(
      confirmMessage,
      `确认${action}`,
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )

    // 设置加载状态
    statusLoadingMap.value.set(notification.id, true)

    if (notification.enabled) {
      await notificationAPI.disable(notification.id)
      ElMessage.success('公告禁用成功')
    } else {
      await notificationAPI.enable(notification.id)
      ElMessage.success('公告启用成功')
    }

    // 重新加载列表
    loadNotifications()
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error(`${action}公告失败`)
    }
  } finally {
    // 清除加载状态
    statusLoadingMap.value.delete(notification.id)
  }
}

// 删除公告
const deleteNotification = async (notification) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除公告"${notification.message}"吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )

    await notificationAPI.delete(notification.id)
    ElMessage.success('公告删除成功')
    loadNotifications()
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error('删除公告失败')
    }
  }
}

// 生成新的通知ID
const generateNewNotificationId = () => {
  // 生成UUID格式的通知ID
  notificationForm.notification_id = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0
    const v = c === 'x' ? r : (r & 0x3 | 0x8)
    return v.toString(16)
  })
  ElMessage.success('已生成新的通知ID')
}

// 组件挂载时加载数据
onMounted(() => {
  loadNotifications()
})
</script>

<style scoped>
.notification-management-page {
  width: 100%;
  min-height: calc(100vh - 140px);
}

.filter-card {
  margin-bottom: 24px;
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.content-card {
  border-radius: 16px;
  border: 1px solid rgba(102, 126, 234, 0.2);
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(10px);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.empty-state {
  padding: 40px 0;
  text-align: center;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

/* 确保表格操作按钮稳定性 */
.el-table .el-table__cell {
  white-space: nowrap;
}

.el-table .el-button {
  margin-right: 8px;
}

.el-table .el-button:last-child {
  margin-right: 0;
}


</style>
