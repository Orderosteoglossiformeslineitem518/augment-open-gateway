<template>
  <div class="system-announcement-page">
    <!-- 筛选表单 -->
    <el-card class="filter-card">
      <el-form inline>
        <el-form-item>
          <el-button type="primary" @click="showCreateDialog">
            <el-icon><Plus /></el-icon>
            新增公告
          </el-button>
          <el-button @click="loadAnnouncements" :loading="loading">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 公告列表 -->
    <el-card class="content-card" v-loading="loading">

      <el-table :data="announcements" style="width: 100%; table-layout: fixed;">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="title" label="公告标题" min-width="200">
          <template #default="{ row }">
            <span :title="row.title">
              {{ row.title.length > 30 ? row.title.substring(0, 30) + '...' : row.title }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="content" label="公告内容" min-width="300">
          <template #default="{ row }">
            <span :title="row.content">
              {{ row.content.length > 50 ? row.content.substring(0, 50) + '...' : row.content }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="发布状态" width="120">
          <template #default="{ row }">
            <el-tag :type="row.status === 'published' ? 'success' : 'info'">
              {{ row.status === 'published' ? '已发布' : '已取消' }}
            </el-tag>
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
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="editAnnouncement(row)" style="width: 50px;">
              编辑
            </el-button>
            <el-button
              :type="row.status === 'published' ? 'warning' : 'success'"
              size="small"
              @click="toggleAnnouncementStatus(row)"
              :loading="statusLoadingMap.get(row.id) || false"
              style="width: 50px;"
            >
              {{ row.status === 'published' ? '取消' : '发布' }}
            </el-button>
            <el-button type="danger" size="small" @click="deleteAnnouncement(row)" style="width: 50px;">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 空状态 -->
      <div v-if="!loading && announcements.length === 0" class="empty-state">
        <el-empty description="暂无公告数据">
          <el-button type="primary" @click="showCreateDialog">创建第一个公告</el-button>
        </el-empty>
      </div>
    </el-card>

    <!-- 创建/编辑公告对话框 -->
    <el-dialog
      :title="dialogTitle"
      v-model="dialogVisible"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="announcementFormRef"
        :model="announcementForm"
        :rules="formRules"
        label-width="100px"
      >
        <el-form-item label="公告标题" prop="title">
          <el-input
            v-model="announcementForm.title"
            placeholder="请输入公告标题（最多200个字符）"
            maxlength="200"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="公告内容" prop="content">
          <el-input
            v-model="announcementForm.content"
            type="textarea"
            :autosize="{ minRows: 4, maxRows: 10 }"
            placeholder="请输入公告内容"
          />
        </el-form-item>
        <el-form-item label="发布状态" prop="status">
          <el-select v-model="announcementForm.status" placeholder="请选择发布状态" style="width: 100%">
            <el-option value="published" label="已发布">
              <el-tag type="success">已发布</el-tag>
            </el-option>
            <el-option value="cancelled" label="已取消">
              <el-tag type="info">已取消</el-tag>
            </el-option>
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="saveAnnouncement" :loading="saving">
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
import { systemAnnouncementAPI } from '@/api'

// 响应式数据
const loading = ref(false)
const saving = ref(false)
const announcements = ref([])
const dialogVisible = ref(false)
const isEdit = ref(false)
const currentAnnouncementId = ref(null)
const announcementFormRef = ref()
const statusLoadingMap = ref(new Map())

// 表单数据
const announcementForm = reactive({
  title: '',
  content: '',
  status: 'published'
})

// 表单验证规则
const formRules = {
  title: [
    { required: true, message: '请输入公告标题', trigger: 'blur' },
    { max: 200, message: '公告标题不能超过200个字符', trigger: 'blur' }
  ],
  content: [
    { required: true, message: '请输入公告内容', trigger: 'blur' }
  ],
  status: [
    { required: true, message: '请选择发布状态', trigger: 'change' }
  ]
}

// 计算属性
const dialogTitle = computed(() => isEdit.value ? '编辑公告' : '新增公告')

// 格式化时间
const formatTime = (time) => {
  if (!time) return '-'
  return new Date(time).toLocaleString('zh-CN')
}

// 加载公告列表
const loadAnnouncements = async () => {
  try {
    loading.value = true
    const response = await systemAnnouncementAPI.getList()
    announcements.value = response.list || []
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
  currentAnnouncementId.value = null
  announcementForm.title = ''
  announcementForm.content = ''
  announcementForm.status = 'published'
  dialogVisible.value = true
}

// 编辑公告
const editAnnouncement = (announcement) => {
  isEdit.value = true
  currentAnnouncementId.value = announcement.id
  announcementForm.title = announcement.title || ''
  announcementForm.content = announcement.content || ''
  announcementForm.status = announcement.status || 'published'
  dialogVisible.value = true
}

// 保存公告
const saveAnnouncement = async () => {
  try {
    await announcementFormRef.value.validate()
    saving.value = true

    if (isEdit.value) {
      await systemAnnouncementAPI.update(currentAnnouncementId.value, announcementForm)
      ElMessage.success('公告更新成功')
    } else {
      await systemAnnouncementAPI.create(announcementForm)
      ElMessage.success('公告创建成功')
    }

    dialogVisible.value = false
    loadAnnouncements()
  } catch (error) {
    // 只有当错误未被拦截器处理过时才显示错误消息，避免重复提示
    if (error.message && !error.handled) {
      ElMessage.error(error.message)
    }
  } finally {
    saving.value = false
  }
}

// 切换公告发布状态
const toggleAnnouncementStatus = async (announcement) => {
  const action = announcement.status === 'published' ? '取消' : '发布'
  const confirmMessage = announcement.status === 'published'
    ? `确定要取消公告"${announcement.title}"吗？`
    : `确定要发布公告"${announcement.title}"吗？`

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
    statusLoadingMap.value.set(announcement.id, true)

    if (announcement.status === 'published') {
      await systemAnnouncementAPI.cancel(announcement.id)
      ElMessage.success('公告取消成功')
    } else {
      await systemAnnouncementAPI.publish(announcement.id)
      ElMessage.success('公告发布成功')
    }

    // 重新加载列表
    loadAnnouncements()
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error(`${action}公告失败`)
    }
  } finally {
    // 清除加载状态
    statusLoadingMap.value.delete(announcement.id)
  }
}

// 删除公告
const deleteAnnouncement = async (announcement) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除公告"${announcement.title}"吗？`,
      '确认删除',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )

    await systemAnnouncementAPI.delete(announcement.id)
    ElMessage.success('公告删除成功')
    loadAnnouncements()
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error('删除公告失败')
    }
  }
}

// 组件挂载时加载数据
onMounted(() => {
  loadAnnouncements()
})
</script>

<style scoped>
.system-announcement-page {
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
