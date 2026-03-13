<template>
  <div class="request-records-page">
    <!-- 搜索和过滤 -->
    <el-card class="filter-card">
      <el-row :gutter="20">
        <el-col :span="6">
          <el-input
            v-model="searchForm.pathSearch"
            placeholder="搜索请求路径"
            clearable
            @input="handleSearch"
            @keyup.enter="handleSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </el-col>
        <el-col :span="18" style="text-align: right;">
          <el-button @click="handleSearch" style="margin-right: 8px;">
            <el-icon><Search /></el-icon>
            搜索
          </el-button>
          <el-button @click="resetSearch">
            <el-icon><Refresh /></el-icon>
            重置
          </el-button>
        </el-col>
      </el-row>
    </el-card>

    <!-- 请求记录列表 -->
    <el-card class="table-card">
      <el-table
        :data="records || []"
        :loading="loading"
        stripe
        v-loading="loading"
        element-loading-text="加载中..."
      >
        <el-table-column prop="id" label="ID" width="80" />

        <el-table-column prop="path" label="请求路径" min-width="200">
          <template #default="{ row }">
            <div class="path-cell" :title="row.path">
              {{ truncateText(row.path, 50) }}
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="method" label="请求方式" width="100">
          <template #default="{ row }">
            <el-tag :type="getMethodTagType(row.method)" size="small">
              {{ row.method }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="user_agent" label="User-Agent" min-width="150">
          <template #default="{ row }">
            <div class="user-agent-cell" :title="row.user_agent">
              {{ truncateText(row.user_agent, 30) }}
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="client_ip" label="客户端IP" width="120" />

        <el-table-column prop="created_at" label="创建时间" width="160">
          <template #default="{ row }">
            {{ formatTime(row.created_at) }}
          </template>
        </el-table-column>

        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button type="primary" size="small" @click="viewDetails(row)">
              详情
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="pagination.total || 0"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>

    <!-- 详情弹窗 -->
    <el-dialog
      v-model="showDetailModal"
      title="请求详情"
      width="600px"
      :before-close="closeDetailModal"
    >
      <div v-if="selectedRecord">
        <el-descriptions :column="1" border>
          <el-descriptions-item label="请求路径">
            {{ selectedRecord.path }}
          </el-descriptions-item>
          <el-descriptions-item label="请求方式">
            <el-tag :type="getMethodTagType(selectedRecord.method)" size="small">
              {{ selectedRecord.method }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="客户端IP">
            {{ selectedRecord.client_ip }}
          </el-descriptions-item>
          <el-descriptions-item label="User-Agent">
            {{ selectedRecord.user_agent }}
          </el-descriptions-item>
          <el-descriptions-item label="创建时间">
            {{ formatTime(selectedRecord.created_at) }}
          </el-descriptions-item>
        </el-descriptions>

        <div v-if="selectedRecord.request_params" style="margin-top: 20px;">
          <h4>请求参数</h4>
          <el-input
            type="textarea"
            :rows="6"
            :value="formatJSON(selectedRecord.request_params)"
            readonly
          />
        </div>

        <div v-if="selectedRecord.request_headers" style="margin-top: 20px;">
          <h4>请求头</h4>
          <el-input
            type="textarea"
            :rows="6"
            :value="formatJSON(selectedRecord.request_headers)"
            readonly
          />
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import api from '../api'

const records = ref([])
const loading = ref(false)
const showDetailModal = ref(false)
const selectedRecord = ref(null)

const searchForm = reactive({
  pathSearch: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0,
  totalPages: 0
})

// 获取请求记录列表
const fetchRecords = async () => {
  try {
    loading.value = true
    const params = {
      page: pagination.page,
      page_size: pagination.pageSize
    }

    if (searchForm.pathSearch) {
      params.path_search = searchForm.pathSearch
    }

    const response = await api.get('/request-records', { params })

    // API拦截器已经处理了统一响应格式，直接使用返回的数据
    records.value = response.data || []
    const paginationData = response.pagination
    pagination.total = paginationData.total
    pagination.totalPages = paginationData.total_pages
  } catch (error) {
    console.error('获取请求记录失败:', error)
  } finally {
    loading.value = false
  }
}

// 搜索
const handleSearch = () => {
  pagination.page = 1
  fetchRecords()
}

// 重置搜索
const resetSearch = () => {
  searchForm.pathSearch = ''
  pagination.page = 1
  fetchRecords()
}

// 分页处理
const handlePageChange = (page) => {
  pagination.page = page
  fetchRecords()
}

const handleSizeChange = (size) => {
  pagination.pageSize = size
  pagination.page = 1
  fetchRecords()
}

// 查看详情
const viewDetails = (record) => {
  selectedRecord.value = record
  showDetailModal.value = true
}

// 关闭详情弹窗
const closeDetailModal = () => {
  showDetailModal.value = false
  selectedRecord.value = null
}

// 工具函数
const truncateText = (text, maxLength) => {
  if (!text) return ''
  return text.length > maxLength ? text.substring(0, maxLength) + '...' : text
}

const getMethodTagType = (method) => {
  const methodTypes = {
    'GET': 'success',
    'POST': 'primary',
    'PUT': 'warning',
    'DELETE': 'danger',
    'PATCH': 'info'
  }
  return methodTypes[method] || ''
}

const formatTime = (timeStr) => {
  if (!timeStr) return ''
  return new Date(timeStr).toLocaleString('zh-CN')
}

const formatJSON = (jsonStr) => {
  if (!jsonStr) return ''
  try {
    return JSON.stringify(JSON.parse(jsonStr), null, 2)
  } catch {
    return jsonStr
  }
}

onMounted(() => {
  fetchRecords()
})
</script>

<style scoped>
.request-records-page {
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

.path-cell, .user-agent-cell {
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 24px;
  padding-top: 24px;
  border-top: 1px solid var(--border-color-light);
}



.path-cell, .user-agent-cell {
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}


</style>
