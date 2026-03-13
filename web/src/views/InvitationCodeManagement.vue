<template>
  <div class="invitation-code-management">
    <!-- 筛选表单 -->
    <el-card class="filter-card">
      <el-form :model="filters" inline>
        <el-form-item label="搜索">
          <el-input
            v-model="searchKeyword"
            placeholder="搜索邀请码"
            clearable
            @keyup.enter="fetchList"
            @clear="fetchList"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select v-model="filterStatus" placeholder="全部状态" clearable @change="fetchList" style="width: 140px;">
            <el-option label="全部" value="" />
            <el-option label="未使用" value="unused" />
            <el-option label="已使用" value="used" />
          </el-select>
        </el-form-item>
        <el-form-item label="使用用户">
          <el-input
            v-model="searchUsedBy"
            placeholder="搜索使用用户"
            clearable
            @keyup.enter="fetchList"
            @clear="fetchList"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="fetchList">搜索</el-button>
          <el-button @click="resetFilters">重置</el-button>
          <el-button type="primary" @click="handleGenerate" :loading="generateLoading">
            <el-icon><Plus /></el-icon>
            生成邀请码
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 邀请码列表 -->
    <el-card class="table-card">
      <el-table
        :data="codeList"
        v-loading="loading"
        style="width: 100%"
      >
        <el-table-column prop="code" label="邀请码" min-width="150">
          <template #default="{ row }">
            <div class="code-cell">
              <span class="code-text">{{ row.code }}</span>
              <el-button link type="primary" @click="copyCode(row.code)">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'unused' ? 'success' : 'info'">
              {{ row.status === 'unused' ? '未使用' : '已使用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="creator_name" label="创建者" width="120" />
        <el-table-column prop="used_by_name" label="使用用户" width="120">
          <template #default="{ row }">
            {{ row.used_by_name || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="生成时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column prop="used_at" label="使用时间" width="180">
          <template #default="{ row }">
            {{ row.used_at ? formatDate(row.used_at) : '-' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="fetchList"
          @current-change="fetchList"
        />
      </div>
    </el-card>

    <!-- 生成数量输入对话框 -->
    <el-dialog
      v-model="generateInputDialogVisible"
      title="生成邀请码"
      width="400px"
    >
      <el-form label-width="80px">
        <el-form-item label="生成数量">
          <el-input-number
            v-model="generateCount"
            :min="1"
            :max="100"
            controls-position="right"
            style="width: 100%"
          />
        </el-form-item>
        <p style="color: #909399; font-size: 12px; margin: 0;">请输入1-100之间的数量</p>
      </el-form>
      <template #footer>
        <el-button @click="generateInputDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmGenerate" :loading="generateLoading">确定生成</el-button>
      </template>
    </el-dialog>

    <!-- 生成结果对话框 -->
    <el-dialog
      v-model="generateDialogVisible"
      title="生成结果"
      width="500px"
    >
      <div class="generate-result">
        <p>成功生成 {{ generatedCodes.length }} 个邀请码：</p>
        <div class="code-list">
          <div v-for="code in generatedCodes" :key="code" class="code-item">
            <span>{{ code }}</span>
            <el-button link type="primary" @click="copyCode(code)">复制</el-button>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="copyAllCodes">复制全部</el-button>
        <el-button type="primary" @click="generateDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, CopyDocument, Plus } from '@element-plus/icons-vue'
import { invitationCodeAPI } from '@/api'

// 筛选条件
const filters = reactive({
  status: '',
  keyword: ''
})

// 重置筛选条件
const resetFilters = () => {
  filterStatus.value = ''
  searchKeyword.value = ''
  searchUsedBy.value = ''
  fetchList()
}

// 列表数据
const codeList = ref([])
const loading = ref(false)

// 分页
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

// 筛选
const filterStatus = ref('')
const searchKeyword = ref('')
const searchUsedBy = ref('')

// 生成邀请码
const generateCount = ref(10)
const generateLoading = ref(false)
const generateInputDialogVisible = ref(false)
const generateDialogVisible = ref(false)
const generatedCodes = ref([])

// 格式化日期
const formatDate = (dateStr) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// 获取邀请码列表
const fetchList = async () => {
  loading.value = true
  try {
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize
    }
    if (filterStatus.value) {
      params.status = filterStatus.value
    }
    if (searchKeyword.value) {
      params.keyword = searchKeyword.value
    }
    if (searchUsedBy.value) {
      params.used_by_name = searchUsedBy.value
    }
    const data = await invitationCodeAPI.list(params)
    codeList.value = data?.list || []
    pagination.total = data?.total || 0
  } catch (error) {
    console.error('获取列表失败:', error)
    codeList.value = []
  } finally {
    loading.value = false
  }
}

// 生成邀请码 - 打开输入对话框
const handleGenerate = () => {
  generateCount.value = 10
  generateInputDialogVisible.value = true
}

// 确认生成邀请码
const confirmGenerate = async () => {
  const count = parseInt(generateCount.value)
  if (!count || count < 1 || count > 100) {
    ElMessage.warning('请输入1-100之间的数量')
    return
  }

  generateLoading.value = true
  try {
    const data = await invitationCodeAPI.generate({ count })
    generatedCodes.value = data?.codes || []
    generateInputDialogVisible.value = false
    generateDialogVisible.value = true
    ElMessage.success(`成功生成 ${data?.count || 0} 个邀请码`)
    fetchList()
  } catch (error) {
    console.error('生成邀请码失败:', error)
  } finally {
    generateLoading.value = false
  }
}

// 复制邀请码
const copyCode = async (code) => {
  try {
    await navigator.clipboard.writeText(code)
    ElMessage.success('已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

// 复制全部邀请码
const copyAllCodes = async () => {
  try {
    await navigator.clipboard.writeText(generatedCodes.value.join('\n'))
    ElMessage.success('已复制全部邀请码')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

// 删除邀请码
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除邀请码 "${row.code}" 吗？`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await invitationCodeAPI.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除失败:', error)
    }
  }
}

// 初始化
onMounted(() => {
  fetchList()
})
</script>

<style scoped>
.invitation-code-management {
  width: 100%;
  min-height: calc(100vh - 140px);
}

/* 筛选卡片 */
.filter-card {
  margin-bottom: 24px;
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.filter-card :deep(.el-form) {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
}

/* 表格卡片 */
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

.code-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.code-text {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-weight: 600;
  color: var(--text-color-primary);
}

/* 分页 */
.pagination-wrapper {
  margin-top: 20px;
  display: flex;
  justify-content: center;
}

/* 生成结果对话框 */
.generate-result {
  max-height: 400px;
  overflow-y: auto;
}

.generate-result p {
  margin-bottom: 15px;
  color: #606266;
}

.code-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.code-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
}

/* 响应式 */
@media (max-width: 768px) {
  .filter-card :deep(.el-form) {
    flex-direction: column;
    align-items: stretch;
  }

  .filter-card :deep(.el-form-item) {
    margin-right: 0;
    margin-bottom: 12px;
  }
}
</style>
