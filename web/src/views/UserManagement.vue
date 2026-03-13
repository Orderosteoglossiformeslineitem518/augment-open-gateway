<template>
  <div class="user-management-page">
    <!-- 筛选表单 -->
    <el-card class="filter-card">
      <el-form :model="filters" inline>
        <el-form-item label="状态">
          <el-select v-model="filters.status" placeholder="全部状态" clearable @change="loadUsers" style="width: 140px">
            <el-option label="活跃" value="active" />
            <el-option label="封禁" value="banned" />
            <el-option label="未激活" value="inactive" />
          </el-select>
        </el-form-item>
        <el-form-item label="搜索">
          <el-input 
            v-model="filters.keyword" 
            placeholder="用户名或邮箱" 
            clearable 
            @clear="loadUsers"
            @keyup.enter="loadUsers"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadUsers">搜索</el-button>
          <el-button @click="resetFilters">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 用户列表 -->
    <el-card class="table-card">
      <el-table :data="users" v-loading="loading" empty-text="暂无用户数据">
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="username" label="用户名" width="150" />
        <el-table-column prop="email" label="邮箱" min-width="200">
          <template #default="{ row }">
            {{ row.email || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="can_use_shared_tokens" label="共享权限" width="100">
          <template #default="{ row }">
            <el-switch 
              v-model="row.can_use_shared_tokens" 
              @change="handleToggleShared(row)"
              :loading="row.loading"
            />
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="last_login" label="最近登录" width="180">
          <template #default="{ row }">
            {{ formatDate(row.last_login) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button 
              v-if="row.status === 'active'" 
              size="small" 
              type="danger"
              @click="handleBan(row)"
            >
              封禁
            </el-button>
            <el-button 
              v-else 
              size="small" 
              type="success"
              @click="handleUnban(row)"
            >
              解封
            </el-button>
            <el-button size="small" @click="showEditDialog(row)">编辑</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadUsers"
          @current-change="loadUsers"
        />
      </div>
    </el-card>

    <!-- 编辑用户对话框 -->
    <el-dialog v-model="showEditDialogVisible" title="编辑用户" width="500px">
      <el-form :model="editForm" label-width="100px">
        <el-form-item label="用户名">
          <el-input v-model="editForm.username" disabled />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="editForm.email" placeholder="用户邮箱" />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="editForm.status" style="width: 100%">
            <el-option label="活跃" value="active" />
            <el-option label="封禁" value="banned" />
            <el-option label="未激活" value="inactive" />
          </el-select>
        </el-form-item>
        <el-form-item label="共享权限">
          <el-switch v-model="editForm.can_use_shared_tokens" />
        </el-form-item>
        <el-form-item label="最大请求数">
          <el-input-number v-model="editForm.max_requests" :min="-1" :max="100000" style="width: 100%" />
          <div class="form-tip">-1 表示无限制</div>
        </el-form-item>
        <el-form-item label="频率限制">
          <el-input-number v-model="editForm.rate_limit_per_minute" :min="1" :max="1000" style="width: 100%" />
          <div class="form-tip">每分钟最大请求数</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveUser" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { userManagementAPI } from '@/api'

// 响应式数据
const loading = ref(false)
const saving = ref(false)
const users = ref([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const filters = reactive({
  status: '',
  keyword: ''
})

const showEditDialogVisible = ref(false)
const editForm = reactive({
  id: null,
  username: '',
  email: '',
  status: '',
  can_use_shared_tokens: false,
  max_requests: -1,
  rate_limit_per_minute: 30
})

// 获取状态类型
const getStatusType = (status) => {
  switch (status) {
    case 'active': return 'success'
    case 'banned': return 'danger'
    case 'inactive': return 'info'
    default: return 'info'
  }
}

// 获取状态文本
const getStatusText = (status) => {
  switch (status) {
    case 'active': return '活跃'
    case 'banned': return '封禁'
    case 'inactive': return '未激活'
    default: return status
  }
}

// 格式化日期
const formatDate = (dateString) => {
  if (!dateString) return '-'
  return new Date(dateString).toLocaleString('zh-CN')
}

// 加载用户列表
const loadUsers = async () => {
  try {
    loading.value = true
    const response = await userManagementAPI.list({
      page: currentPage.value,
      page_size: pageSize.value,
      status: filters.status,
      keyword: filters.keyword
    })
    users.value = response.list || []
    total.value = response.total || 0
  } catch (error) {
    console.error('加载用户列表失败:', error)
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      ElMessage.error('加载用户列表失败')
    }
  } finally {
    loading.value = false
  }
}

// 重置筛选条件
const resetFilters = () => {
  filters.status = ''
  filters.keyword = ''
  loadUsers()
}

// 封禁用户
const handleBan = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要封禁用户 "${row.username}" 吗？`, '确认封禁', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    await userManagementAPI.banUser(row.id)
    ElMessage.success('用户已封禁')
    loadUsers()
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error('封禁失败')
    }
  }
}

// 解封用户
const handleUnban = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要解封用户 "${row.username}" 吗？`, '确认解封', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    await userManagementAPI.unbanUser(row.id)
    ElMessage.success('用户已解封')
    loadUsers()
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error('解封失败')
    }
  }
}

// 切换共享权限
const handleToggleShared = async (row) => {
  try {
    row.loading = true
    await userManagementAPI.toggleSharedPermission(row.id, row.can_use_shared_tokens)
    ElMessage.success('权限已更新')
  } catch (error) {
    // 恢复原状态
    row.can_use_shared_tokens = !row.can_use_shared_tokens
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      ElMessage.error('更新失败')
    }
  } finally {
    row.loading = false
  }
}

// 显示编辑对话框
const showEditDialog = (row) => {
  Object.assign(editForm, {
    id: row.id,
    username: row.username,
    email: row.email || '',
    status: row.status,
    can_use_shared_tokens: row.can_use_shared_tokens || false,
    max_requests: row.max_requests || -1,
    rate_limit_per_minute: row.rate_limit_per_minute || 30
  })
  showEditDialogVisible.value = true
}

// 保存用户
const saveUser = async () => {
  try {
    saving.value = true
    await userManagementAPI.update(editForm.id, {
      email: editForm.email,
      status: editForm.status,
      can_use_shared_tokens: editForm.can_use_shared_tokens,
      max_requests: editForm.max_requests,
      rate_limit_per_minute: editForm.rate_limit_per_minute
    })
    ElMessage.success('保存成功')
    showEditDialogVisible.value = false
    loadUsers()
  } catch (error) {
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      ElMessage.error('保存失败')
    }
  } finally {
    saving.value = false
  }
}

// 组件挂载时加载数据
onMounted(() => {
  loadUsers()
})
</script>

<style scoped>
.user-management-page {
  width: 100%;
  min-height: calc(100vh - 140px);
}

.filter-card {
  margin-bottom: 24px;
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.1);
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

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

.form-tip {
  font-size: 12px;
  color: var(--text-color-placeholder);
  margin-top: 4px;
}

.text-muted {
  color: var(--text-color-placeholder);
}


</style>
