<template>
  <div class="panel plugins-panel">
    <div class="content-card">
      <div class="card-header">
        <h3>
          插件下载
          <el-tooltip content="插件仅注入了第三方登录方式，未进行其他修改" placement="top">
            <el-icon class="help-icon"><QuestionFilled /></el-icon>
          </el-tooltip>
        </h3>
      </div>
      <div class="filter-section plugin-filter">
        <div class="filter-inputs">
          <el-input 
            v-model="versionKeywordLocal" 
            placeholder="搜索版本号" 
            clearable 
            style="width: 280px"
            @keyup.enter="handleQuery"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </div>
        <div class="filter-buttons">
          <el-button class="reset-btn" @click="handleQueryReset">重置</el-button>
          <el-button class="query-btn" @click="handleQuery">查询</el-button>
        </div>
      </div>
      <div class="card-body">
        <div class="table-wrapper">
        <el-table :data="props.plugins" v-loading="props.loading" style="width: 100%" height="100%" empty-text="暂无插件" table-layout="fixed">
          <el-table-column label="插件图标" width="80" align="center">
            <template #default="{ row }">
              <div class="plugin-icon-wrapper">
                <img v-if="row.plugin_icon" :src="row.plugin_icon" alt="插件图标" class="plugin-icon" />
                <el-icon v-else class="default-icon"><Box /></el-icon>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="plugin_name" label="插件名称" width="160" show-overflow-tooltip />
          <el-table-column prop="plugin_version" label="版本号" width="120" show-overflow-tooltip>
            <template #default="{ row }">
              <el-tag type="primary" size="small">{{ row.plugin_version }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="备注" width="320">
            <template #default="{ row }">
              <el-tooltip v-if="row.remark && row.remark.length > 40" :content="row.remark" placement="top">
                <span class="remark-text">{{ row.remark.substring(0, 40) }}...</span>
              </el-tooltip>
              <span v-else-if="row.remark" class="remark-text">{{ row.remark }}</span>
              <span v-else class="no-remark">-</span>
            </template>
          </el-table-column>
          <el-table-column label="更新内容" min-width="200">
            <template #default="{ row }">
              <el-tooltip v-if="row.update_content && row.update_content.length > 50" placement="top">
                <template #content>
                  <div class="update-content-tooltip">{{ row.update_content }}</div>
                </template>
                <span class="update-content-text">{{ row.update_content.substring(0, 50) }}...</span>
              </el-tooltip>
              <span v-else-if="row.update_content" class="update-content-text">{{ row.update_content }}</span>
              <span v-else class="no-content">-</span>
            </template>
          </el-table-column>
          <el-table-column label="发布时间" width="160" show-overflow-tooltip>
            <template #default="{ row }">
              <span v-if="row.publish_time" class="time-text">{{ formatTime(row.publish_time) }}</span>
              <span v-else class="no-time">-</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120" fixed="right" align="center">
            <template #default="{ row }">
              <el-button 
                type="primary" 
                size="small" 
                class="download-btn"
                @click="handleDownload(row)"
              >
                <el-icon><Download /></el-icon>
                跳转下载
              </el-button>
            </template>
          </el-table-column>
        </el-table>
        </div>
        <div class="pagination-wrapper" v-if="props.total > 0">
          <el-pagination
            v-model:current-page="pageLocal"
            v-model:page-size="pageSizeLocal"
            :page-sizes="[10, 20, 50, 100]"
            :total="props.total"
            layout="total, sizes, prev, pager, next"
            @current-change="handlePageChange"
            @size-change="handlePageSizeChange"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Download, Box, QuestionFilled } from '@element-plus/icons-vue'

const props = defineProps({
  plugins: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  page: { type: Number, default: 1 },
  pageSize: { type: Number, default: 10 },
  total: { type: Number, default: 0 },
  versionKeyword: { type: String, default: '' }
})

const emit = defineEmits([
  'update:page', 'update:pageSize', 'update:versionKeyword', 'fetchPlugins'
])

// 本地状态（用于双向绑定）
const pageLocal = ref(props.page)
const pageSizeLocal = ref(props.pageSize)
const versionKeywordLocal = ref(props.versionKeyword)

// 监听 props 变化
watch(() => props.page, (val) => { pageLocal.value = val })
watch(() => props.pageSize, (val) => { pageSizeLocal.value = val })
watch(() => props.versionKeyword, (val) => { versionKeywordLocal.value = val })

// 格式化时间
const formatTime = (time) => {
  if (!time) return '-'
  const date = new Date(time)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

// 查询
const handleQuery = () => {
  emit('update:versionKeyword', versionKeywordLocal.value)
  emit('update:page', 1)
  emit('fetchPlugins')
}

// 重置查询
const handleQueryReset = () => {
  versionKeywordLocal.value = ''
  emit('update:versionKeyword', '')
  emit('update:page', 1)
  emit('fetchPlugins')
}

// 分页变化
const handlePageChange = (page) => {
  emit('update:page', page)
  emit('fetchPlugins')
}

const handlePageSizeChange = (size) => {
  emit('update:pageSize', size)
  emit('update:page', 1)
  emit('fetchPlugins')
}

// 跳转下载
const handleDownload = (plugin) => {
  if (plugin.plugin_url) {
    window.open(plugin.plugin_url, '_blank')
  } else {
    ElMessage.error('下载地址不存在')
  }
}
</script>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.plugins-panel .content-card {
  height: calc(100vh - 140px);
  min-height: 400px;
}

.content-card {
  background: var(--card, oklch(1 0 0));
  border-radius: var(--radius-xl, 14px);
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 20px;
}

.card-header h3 {
  font-size: 16px;
  font-weight: 600;
  margin: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.help-icon {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  cursor: help;
}

.help-icon:hover {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.filter-section {
  padding: 12px 20px;
}

.plugin-filter {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.plugin-filter .filter-inputs {
  flex: 1;
}

.filter-buttons {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.filter-buttons .reset-btn {
  background: var(--card, oklch(1 0 0)) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32)) !important;
  border-radius: var(--radius-md, 8px) !important;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938)) !important;
  font-weight: 500 !important;
  padding: 8px 16px !important;
  transition: all 0.2s ease !important;
}

.filter-buttons .reset-btn:hover {
  background: var(--secondary, oklch(0.967 0.001 286.375)) !important;
  border-color: var(--ring, oklch(0.705 0.015 286.067)) !important;
  color: var(--foreground, oklch(0.141 0.005 285.823)) !important;
}

.filter-buttons .query-btn {
  background: var(--primary, oklch(0.21 0.006 285.885)) !important;
  border: none !important;
  border-radius: var(--radius-md, 8px) !important;
  color: var(--card, oklch(1 0 0)) !important;
  font-weight: 500 !important;
  padding: 8px 16px !important;
  transition: all 0.2s ease !important;
}

.filter-buttons .query-btn:hover {
  background: oklch(0.3 0.006 285.885) !important;
  transform: translateY(-1px);
}

.card-body {
  padding: 16px 20px;
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
}

.table-wrapper {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.plugin-icon-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  margin: 0 auto;
}

.plugin-icon {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md, 8px);
  object-fit: cover;
}

.default-icon {
  font-size: 28px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.remark-text {
  font-size: 13px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.no-remark {
  color: var(--ring, oklch(0.705 0.015 286.067));
}

.update-content-text {
  font-size: 13px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  line-height: 1.5;
}

.update-content-tooltip {
  max-width: 400px;
  white-space: pre-wrap;
  word-break: break-all;
}

.no-content {
  color: var(--ring, oklch(0.705 0.015 286.067));
}

.time-text {
  font-size: 13px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.no-time {
  color: var(--ring, oklch(0.705 0.015 286.067));
}

.download-btn {
  background: var(--primary, oklch(0.21 0.006 285.885)) !important;
  border: none !important;
  border-radius: var(--radius-sm, 6px) !important;
  color: var(--card, oklch(1 0 0)) !important;
  font-weight: 500 !important;
  font-size: 13px !important;
  padding: 6px 12px !important;
  transition: all 0.2s ease !important;
}

.download-btn:hover {
  background: oklch(0.3 0.006 285.885) !important;
  transform: translateY(-1px);
}

.download-btn .el-icon {
  margin-right: 4px;
}

.pagination-wrapper {
  margin-top: 16px;
  display: flex;
  justify-content: center;
}
</style>
