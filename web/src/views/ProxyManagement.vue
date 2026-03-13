<template>
  <div class="proxy-management-page">
    <!-- 筛选表单 -->
    <el-card class="filter-card">
      <el-form :model="filters" inline>
        <el-form-item label="状态">
          <el-select v-model="filters.status" placeholder="全部状态" clearable @change="loadProxies" style="width: 140px">
            <el-option label="待审核" value="pending" />
            <el-option label="有效" value="valid" />
            <el-option label="无效" value="invalid" />
          </el-select>
        </el-form-item>
        <el-form-item label="用户ID">
          <el-input
            v-model="filters.userId"
            placeholder="用户ID"
            clearable
            @clear="loadProxies"
            @keyup.enter="loadProxies"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadProxies">搜索</el-button>
          <el-button @click="resetFilters">重置</el-button>
          <el-button type="primary" @click="showCreateDialogFunc">
            <el-icon><Plus /></el-icon>
            添加代理
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 代理列表 -->
    <el-card class="table-card">
      <el-table
        :data="proxies"
        v-loading="loading"
        empty-text="暂无代理数据"
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="proxy_url" label="代理地址" min-width="250">
          <template #default="{ row }">
            <span class="proxy-url">{{ row.proxy_url }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="user" label="提交用户" width="120">
          <template #default="{ row }">
            <span v-if="row.user">
              {{ row.user.username || `ID: ${row.user_id}` }}
            </span>
            <span v-else-if="row.user_id" class="user-id">
              用户ID: {{ row.user_id }}
            </span>
            <span v-else class="admin-added">管理员添加</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="description" label="备注" min-width="150">
          <template #default="{ row }">
            <span v-if="row.description">{{ row.description }}</span>
            <span v-else class="no-description">-</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.status === 'pending'"
              type="warning"
              size="small"
              @click="showReviewDialog(row)"
            >
              审核
            </el-button>
            <el-button
              type="primary"
              size="small"
              @click="showEditDialogFunc(row)"
            >
              编辑
            </el-button>
            <el-button
              type="danger"
              size="small"
              @click="deleteProxy(row)"
            >
              删除
            </el-button>
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
          @size-change="loadProxies"
          @current-change="loadProxies"
        />
      </div>
    </el-card>

    <!-- 编辑代理对话框 -->
    <el-dialog v-model="showEditDialogVisible" title="编辑代理" width="500px">
      <el-form :model="editForm" :rules="editRules" ref="editFormRef" label-width="100px">
        <el-form-item label="代理地址" prop="proxyUrl">
          <el-input v-model="editForm.proxyUrl" placeholder="https://xxx-xxxxxxx-xx.deno.dev/ 或 https://xxx-xxxxxxx-xx.deno.net/" />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-select v-model="editForm.status">
            <el-option label="待审核" value="pending" />
            <el-option label="有效" value="valid" />
            <el-option label="无效" value="invalid" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注说明" prop="description">
          <el-input v-model="editForm.description" type="textarea" :rows="3" placeholder="可选" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEditDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="updateProxy" :loading="updating">确定</el-button>
      </template>
    </el-dialog>

    <!-- 审核代理对话框 -->
    <el-dialog v-model="showReviewDialogVisible" title="审核代理" width="500px">
      <div class="review-content">
        <div class="proxy-info">
          <h4>代理信息</h4>
          <p><strong>代理地址：</strong>{{ reviewForm.proxy_url }}</p>
          <p><strong>提交用户：</strong>{{ getUserDisplayText(reviewForm) }}</p>
          <p><strong>提交时间：</strong>{{ formatDate(reviewForm.created_at) }}</p>
        </div>

        <el-divider />

        <el-form :model="reviewForm" label-width="100px">
          <el-form-item label="审核结果" required>
            <el-radio-group v-model="reviewForm.reviewResult">
              <el-radio value="approve">
                <el-icon><Check /></el-icon>
                通过审核
              </el-radio>
              <el-radio value="reject">
                <el-icon><Close /></el-icon>
                拒绝申请
              </el-radio>
            </el-radio-group>
          </el-form-item>

          <el-form-item
            v-if="reviewForm.reviewResult === 'reject'"
            label="拒绝原因"
            required
          >
            <el-input
              v-model="reviewForm.reason"
              type="textarea"
              :rows="3"
              placeholder="请输入拒绝原因，将作为反馈给用户的说明"
            />
          </el-form-item>

          <el-form-item
            v-if="reviewForm.reviewResult === 'approve'"
            label="审核备注"
          >
            <el-input
              v-model="reviewForm.approveNote"
              type="textarea"
              :rows="2"
              placeholder="可选：添加审核通过的备注说明"
            />
          </el-form-item>
        </el-form>
      </div>

      <template #footer>
        <el-button @click="showReviewDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          @click="submitReview"
          :loading="reviewing"
          :disabled="!reviewForm.reviewResult || (reviewForm.reviewResult === 'reject' && !reviewForm.reason)"
        >
          <el-icon v-if="reviewForm.reviewResult === 'approve'"><Check /></el-icon>
          <el-icon v-else-if="reviewForm.reviewResult === 'reject'"><Close /></el-icon>
          {{ reviewForm.reviewResult === 'approve' ? '确认通过' : '确认拒绝' }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 创建代理对话框 -->
    <el-dialog v-model="showCreateDialogVisible" title="添加代理" width="500px">
      <el-form :model="createForm" :rules="createRules" ref="createFormRef" label-width="100px">
        <el-form-item label="代理地址" prop="proxyUrl">
          <el-input v-model="createForm.proxyUrl" placeholder="https://xxx-xxxxxxx-xx.deno.dev/ 或 https://xxx-xxxxxxx-xx.deno.net/" />
        </el-form-item>
        <el-form-item label="备注说明" prop="description">
          <el-input v-model="createForm.description" type="textarea" :rows="3" placeholder="可选" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="createProxy" :loading="creating">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Check, Close, Plus } from '@element-plus/icons-vue'
import { adminProxyAPI } from '@/api'

// 响应式数据
const loading = ref(false)
const updating = ref(false)
const reviewing = ref(false)
const creating = ref(false)

const proxies = ref([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)

const filters = ref({
  status: '',
  userId: ''
})

// 对话框控制
const showEditDialogVisible = ref(false)
const showReviewDialogVisible = ref(false)
const showCreateDialogVisible = ref(false)

// 表单数据
const editForm = ref({
  id: null,
  proxyUrl: '',
  status: '',
  description: ''
})

const reviewForm = ref({
  id: null,
  proxy_url: '',
  user: null,
  created_at: '',
  reviewResult: '', // 'approve' 或 'reject'
  reason: '', // 拒绝原因
  approveNote: '' // 通过备注
})

const createForm = ref({
  proxyUrl: '',
  description: ''
})

// 表单引用
const editFormRef = ref()
const createFormRef = ref()

// 表单验证规则
const editRules = {
  proxyUrl: [
    { required: true, message: '请输入代理地址', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (!value) {
          callback()
          return
        }
        if (!value.startsWith('https://')) {
          callback(new Error('代理地址必须以https://开头'))
          return
        }

        // 验证域名限制
        try {
          const urlObj = new URL(value)
          const hostname = urlObj.hostname

          // 检查是否为允许的域名
          const allowedDomains = ['.deno.dev', '.deno.net', '.vercel.app', '.supabase.co']
          const isValidDomain = allowedDomains.some(domain => hostname.endsWith(domain))

          if (!isValidDomain) {
            callback(new Error('不支持的代理'))
            return
          }
        } catch (e) {
          callback(new Error('代理地址格式无效'))
          return
        }

        callback()
      },
      trigger: 'blur'
    }
  ],
  status: [
    { required: true, message: '请选择状态', trigger: 'change' }
  ]
}

// 创建代理验证规则
const createRules = {
  proxyUrl: [
    { required: true, message: '请输入代理地址', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (!value) {
          callback()
          return
        }
        if (!value.startsWith('https://')) {
          callback(new Error('代理地址必须以https://开头'))
          return
        }

        // 验证域名限制
        try {
          const urlObj = new URL(value)
          const hostname = urlObj.hostname

          // 检查是否为允许的域名
          const allowedDomains = ['.deno.dev', '.deno.net', '.vercel.app', '.supabase.co']
          const isValidDomain = allowedDomains.some(domain => hostname.endsWith(domain))

          if (!isValidDomain) {
            callback(new Error('不支持的代理'))
            return
          }
        } catch (e) {
          callback(new Error('代理地址格式无效'))
          return
        }

        callback()
      },
      trigger: 'blur'
    }
  ]
}

// 获取状态类型
const getStatusType = (status) => {
  switch (status) {
    case 'pending': return 'warning'
    case 'valid': return 'success'
    case 'invalid': return 'danger'
    default: return 'info'
  }
}

// 获取状态文本
const getStatusText = (status) => {
  switch (status) {
    case 'pending': return '待审核'
    case 'valid': return '有效'
    case 'invalid': return '无效'
    default: return '未知'
  }
}

// 格式化日期
const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString('zh-CN')
}

// 获取用户显示文本
const getUserDisplayText = (row) => {
  if (row.user) {
    return row.user.username || `ID: ${row.user_id}`
  } else if (row.user_id) {
    return `用户ID: ${row.user_id}`
  } else {
    return '管理员添加'
  }
}

// 加载代理列表
const loadProxies = async () => {
  try {
    loading.value = true

    const params = {
      page: currentPage.value,
      page_size: pageSize.value
    }

    if (filters.value.status) {
      params.status = filters.value.status
    }

    if (filters.value.userId) {
      params.user_id = filters.value.userId
    }

    const response = await adminProxyAPI.getProxies(params)
    proxies.value = response.list || []
    total.value = response.total || 0

  } catch (error) {
    console.error('加载代理列表失败:', error)
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      const message = error.response?.data?.msg || error.message || '加载代理列表失败'
      ElMessage.error(message)
    }
    // 发生错误时使用空数据
    proxies.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 重置筛选条件
const resetFilters = () => {
  filters.value = {
    status: '',
    userId: ''
  }
  loadProxies()
}

// 显示创建对话框
const showCreateDialogFunc = () => {
  createForm.value = {
    proxyUrl: '',
    description: ''
  }
  showCreateDialogVisible.value = true
}

// 创建代理
const createProxy = async () => {
  if (!createFormRef.value) return

  try {
    await createFormRef.value.validate()
    creating.value = true

    await adminProxyAPI.createProxy({
      proxy_url: createForm.value.proxyUrl,
      description: createForm.value.description
    })

    ElMessage.success('代理创建成功')
    showCreateDialogVisible.value = false

    await loadProxies()

  } catch (error) {
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      const message = error.response?.data?.msg || '创建代理失败'
      ElMessage.error(message)
    }
  } finally {
    creating.value = false
  }
}

// 显示编辑对话框
const showEditDialogFunc = (row) => {
  editForm.value = {
    id: row.id,
    proxyUrl: row.proxy_url,
    status: row.status,
    description: row.description || ''
  }
  showEditDialogVisible.value = true
}

// 更新代理
const updateProxy = async () => {
  if (!editFormRef.value) return

  try {
    await editFormRef.value.validate()
    updating.value = true

    await adminProxyAPI.updateProxyStatus(editForm.value.id, {
      status: editForm.value.status,
      description: editForm.value.description,
      proxy_url: editForm.value.proxyUrl
    })

    ElMessage.success('代理更新成功')
    showEditDialogVisible.value = false

    await loadProxies()

  } catch (error) {
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      const message = error.response?.data?.msg || '更新代理失败'
      ElMessage.error(message)
    }
  } finally {
    updating.value = false
  }
}


// 显示审核对话框
const showReviewDialog = (row) => {
  reviewForm.value = {
    id: row.id,
    proxy_url: row.proxy_url,
    user: row.user,
    created_at: row.created_at,
    reviewResult: '',
    reason: '',
    approveNote: ''
  }
  showReviewDialogVisible.value = true
}

// 提交审核结果
const submitReview = async () => {
  try {
    reviewing.value = true

    if (reviewForm.value.reviewResult === 'approve') {
      // 审核通过
      await adminProxyAPI.approveProxy(reviewForm.value.id)
      ElMessage.success('代理审核通过')
    } else if (reviewForm.value.reviewResult === 'reject') {
      // 审核拒绝
      await adminProxyAPI.rejectProxy(reviewForm.value.id, {
        reason: reviewForm.value.reason
      })
      ElMessage.success('代理已拒绝')
    }

    showReviewDialogVisible.value = false
    await loadProxies()

  } catch (error) {
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      const message = error.response?.data?.msg || '审核操作失败'
      ElMessage.error(message)
    }
  } finally {
    reviewing.value = false
  }
}

// 删除代理
const deleteProxy = async (row) => {
  try {
    await ElMessageBox.confirm('确定删除该代理吗？此操作不可恢复。', '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    await adminProxyAPI.deleteProxy(row.id)

    ElMessage.success('代理删除成功')
    await loadProxies()

  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      const message = error.response?.data?.msg || '删除代理失败'
      ElMessage.error(message)
    }
  }
}

// 组件挂载时加载数据
onMounted(() => {
  loadProxies()
})
</script>

<style scoped>
.proxy-management-page {
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

.proxy-url {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
  word-break: break-all;
}

.admin-added {
  color: #409eff;
  font-style: italic;
}

.user-id {
  color: #e6a23c;
  font-style: italic;
}

.no-description {
  color: var(--text-color-placeholder);
  font-style: italic;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

/* 审核对话框样式 */
.review-content {
  max-height: 60vh;
  overflow-y: auto;
}

.proxy-info {
  background: var(--bg-color-page);
  padding: 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}

.proxy-info h4 {
  margin: 0 0 12px 0;
  color: var(--text-color-primary);
  font-size: 16px;
  font-weight: 600;
}

.proxy-info p {
  margin: 8px 0;
  color: var(--text-color-secondary);
  font-size: 14px;
  line-height: 1.5;
}

.proxy-info strong {
  color: var(--text-color-primary);
  font-weight: 500;
}

.el-radio {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
  padding: 8px;
  border-radius: 6px;
  transition: background-color 0.2s;
}

.el-radio:hover {
  background-color: var(--bg-color-page);
}

.el-radio .el-icon {
  margin-right: 6px;
}


</style>
