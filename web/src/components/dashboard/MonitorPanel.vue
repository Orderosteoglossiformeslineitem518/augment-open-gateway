<template>
  <div class="panel monitor-panel">
    <div class="content-card">
      <div class="card-header">
        <h3>定时监测
          <el-tooltip content="创建监测任务后会定期发送随机程序问题测试模型可用性" placement="top" effect="light">
            <el-icon style="font-size: 14px; color: #94a3b8; cursor: help; vertical-align: middle; margin-left: 4px;"><QuestionFilled /></el-icon>
          </el-tooltip>
        </h3>
        <div class="header-actions">
          <el-tooltip
            :disabled="total < 10"
            content="服务器负载受限，仅可以添加10条监测记录！"
            placement="bottom"
            effect="light"
          >
            <el-button class="add-token-btn" :disabled="total >= 10" @click="openCreateDrawer">
              <el-icon><Plus /></el-icon>
              创建
            </el-button>
          </el-tooltip>
        </div>
      </div>
      <!-- 搜索区域 -->
      <div class="filter-section channel-filter">
        <div class="filter-inputs">
          <el-input
            v-model="searchName"
            placeholder="渠道名称"
            clearable
            style="width: 200px"
            @keyup.enter="handleQuery"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="searchType" placeholder="渠道类型" clearable style="width: 130px">
            <el-option label="公益" value="公益" />
            <el-option label="自建" value="自建" />
            <el-option label="商业" value="商业" />
          </el-select>
          <el-select v-model="searchStatus" placeholder="监测状态" clearable style="width: 130px">
            <el-option label="启用" value="enabled" />
            <el-option label="禁用" value="disabled" />
          </el-select>
        </div>
        <div class="filter-buttons">
          <el-button class="reset-btn" @click="handleReset">重置</el-button>
          <el-button class="query-btn" @click="handleQuery">查询</el-button>
        </div>
      </div>
      <!-- 表格 -->
      <div class="card-body">
        <div class="table-wrapper">
          <el-table
            :data="configList"
            v-loading="loading"
            style="width: 100%"
            height="100%"
            empty-text="暂无监测配置"
            table-layout="fixed"
            row-key="id"
            :expand-row-keys="expandedRows"
            @expand-change="handleExpandChange"
          >
            <el-table-column type="expand">
              <template #default="{ row }">
                <div class="expand-content" v-loading="row._detailLoading">
                  <div class="model-cards" v-if="row._detail?.models?.length">
                    <div class="model-card" v-for="model in row._detail.models" :key="model.id">
                      <!-- 卡片头部 -->
                      <div class="card-top">
                        <div class="card-top-left">
                          <img
                            v-if="row.provider_icon"
                            :src="getIconUrl(row.provider_icon)"
                            class="provider-icon"
                            @error="handleIconError"
                          />
                          <el-icon v-else class="provider-icon-default"><Connection /></el-icon>
                          <div class="card-title-area">
                            <span class="card-model-name">{{ model.model_name }}</span>
                            <span class="card-channel-sub">{{ row.channel_name }}</span>
                          </div>
                        </div>
                        <el-tag
                          :type="model.latest_status === 'normal' ? 'success' : model.latest_status === 'delayed' ? 'warning' : 'danger'"
                          size="small"
                          class="status-badge"
                        >
                          {{ statusLabel(model.latest_status) }}
                        </el-tag>
                      </div>
                      <!-- 延迟指标 -->
                      <div class="card-metrics">
                        <div class="metric-item">
                          <span class="metric-val">{{ msToSec(model.latest_latency) }}<span class="metric-unit">s</span></span>
                          <span class="metric-desc">对话延迟</span>
                        </div>
                        <div class="metric-divider"></div>
                        <div class="metric-item">
                          <span class="metric-val">{{ msToSec(model.avg_latency) }}<span class="metric-unit">s</span></span>
                          <span class="metric-desc">平均延迟</span>
                        </div>
                      </div>
                      <!-- 可用性 -->
                      <div class="card-status-section">
                        <div class="availability-row">
                          <span class="availability-label">可用性 (14天)</span>
                        </div>
                        <div class="availability-detail">
                          <span class="availability-count">{{ model.success_count || 0 }}/{{ model.total_checks || 0 }} 成功</span>
                          <span :class="['availability-pct', getAvailabilityClass(model.availability)]">
                            {{ (model.availability || 0).toFixed(2) }}%
                          </span>
                        </div>
                      </div>
                      <!-- 历史图表 -->
                      <div class="card-history">
                        <div class="history-header">
                          <span class="history-label">HISTORY ({{ model.daily_stats?.length || 0 }}PTS)</span>
                        </div>
                        <div class="history-bars">
                          <el-tooltip
                            v-for="(stat, idx) in alignDailyStats(model.daily_stats)"
                            :key="idx"
                            placement="top"
                            :show-after="200"
                            effect="light"
                            popper-class="history-tooltip"
                          >
                            <template #content>
                              <div v-if="!stat.total_checks" class="ht-empty">{{ formatStatDate(stat.date, stat.last_checked_at) }}<br/>暂无数据</div>
                              <div v-else class="ht-content">
                                <div class="ht-header">
                                  <span :class="['ht-badge', getDayStatus(stat)]">{{ dayStatusLabel(getDayStatus(stat)) }}</span>
                                  <span class="ht-date">{{ formatStatDate(stat.date, stat.last_checked_at) }}</span>
                                </div>
                                <div class="ht-metrics">
                                  <div class="ht-metric-row">
                                    <span class="ht-metric-label">Latency</span>
                                    <span class="ht-metric-val">{{ msToSec(stat.avg_latency) }} <span class="ht-unit">s</span></span>
                                  </div>
                                  <div class="ht-metric-row">
                                    <span class="ht-metric-label">检测次数</span>
                                    <span class="ht-metric-val">{{ stat.total_checks }} <span class="ht-unit">次</span></span>
                                  </div>
                                </div>
                                <div v-if="stat.last_error" class="ht-error">
                                  {{ stat.last_error }}
                                </div>
                                <div class="ht-summary">
                                  {{ stat.normal_count }}正常 · {{ stat.delayed_count }}延迟 · {{ stat.error_count }}错误
                                </div>
                              </div>
                            </template>
                            <div :class="['history-bar', getDayStatus(stat)]"></div>
                          </el-tooltip>
                        </div>
                        <div class="history-footer">
                          <span>PAST</span>
                          <span>NOW</span>
                        </div>
                      </div>
                    </div>
                  </div>
                  <el-empty v-else-if="!row._detailLoading" description="暂无监测数据" :image-size="60" />
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="channel_name" label="渠道名称" min-width="120" show-overflow-tooltip />
            <el-table-column prop="channel_type" label="渠道类型" width="90" align="center">
              <template #default="{ row }">
                <el-tag size="small" :type="typeTagType(row.channel_type)">{{ row.channel_type }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="监测模型" width="80" align="center">
              <template #default="{ row }">
                <span class="model-count-cell">{{ row.models?.length || 0 }} 个</span>
              </template>
            </el-table-column>
            <el-table-column label="模型状态" min-width="140">
              <template #default="{ row }">
                <div class="model-status-summary">
                  <span v-if="row.normal_count > 0" class="status-dot normal">{{ row.normal_count }} 正常</span>
                  <span v-if="row.delayed_count > 0" class="status-dot delayed">{{ row.delayed_count }} 延迟</span>
                  <span v-if="row.error_count > 0" class="status-dot error">{{ row.error_count }} 错误</span>
                  <span v-if="row.pending_count > 0" class="status-dot pending">{{ row.pending_count }} 待监测</span>
                  <span v-if="!row.normal_count && !row.delayed_count && !row.error_count && !row.pending_count" class="status-dot empty-status">暂无数据</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="监测频率" width="80" align="center">
              <template #default="{ row }">
                <span class="freq-cell">{{ row.check_interval || 1 }}天/次</span>
              </template>
            </el-table-column>
            <el-table-column prop="status" label="监测状态" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 'enabled' ? 'success' : 'info'" size="small">
                  {{ row.status === 'enabled' ? '启用' : '禁用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="created_at" label="创建时间" min-width="150" show-overflow-tooltip>
              <template #default="{ row }">
                {{ formatDate(row.created_at) }}
              </template>
            </el-table-column>
            <el-table-column label="最近监测" min-width="150" show-overflow-tooltip>
              <template #default="{ row }">
                <span v-if="row.last_check_time" class="last-check-cell">{{ formatDate(row.last_check_time) }}</span>
                <span v-else class="last-check-cell empty">暂未监测</span>
              </template>
            </el-table-column>
            <el-table-column label="下次监测" min-width="120" show-overflow-tooltip>
              <template #default="{ row }">
                <span v-if="row.status !== 'enabled'" class="next-check-cell disabled">已禁用</span>
                <span v-else-if="row.next_check_time" :class="['next-check-cell', isOverdue(row.next_check_time) ? 'overdue' : '']">
                  {{ formatRelativeTime(row.next_check_time) }}
                </span>
                <span v-else class="next-check-cell empty">待调度</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200" fixed="right" align="center" header-align="center">
              <template #default="{ row }">
                <div class="action-buttons">
                  <el-button link type="success" size="small" :loading="row._checking" @click="handleManualCheck(row)">
                    监测
                  </el-button>
                  <el-button link type="primary" size="small" @click="openEditDrawer(row)">编辑</el-button>
                  <el-button
                    link
                    :type="row.status === 'enabled' ? 'warning' : 'success'"
                    size="small"
                    @click="handleToggleStatus(row)"
                  >
                    {{ row.status === 'enabled' ? '禁用' : '启用' }}
                  </el-button>
                  <el-button link type="danger" size="small" @click="handleDelete(row)">删除</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div class="pagination-wrapper" v-if="total > 0">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :page-sizes="[10, 20, 50]"
            :total="total"
            layout="total, sizes, prev, pager, next"
            @current-change="fetchList"
            @size-change="fetchList"
          />
        </div>
      </div>
    </div>

    <!-- 创建/编辑抽屉 -->
    <el-drawer
      v-model="drawerVisible"
      :title="drawerMode === 'create' ? '创建监测配置' : '编辑监测配置'"
      direction="rtl"
      size="560px"
      class="monitor-drawer"
      :close-on-click-modal="true"
      @close="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-position="top" class="drawer-form">
        <!-- 渠道信息区 -->
        <div class="form-section">
          <div class="section-title">渠道信息</div>
          <el-form-item label="选择监测渠道" prop="channel_id">
            <el-select
              v-model="form.channel_id"
              placeholder="请选择已启用的渠道"
              filterable
              style="width: 100%"
              @change="handleChannelChange"
            >
              <el-option
                v-for="ch in enabledChannels"
                :key="ch.id"
                :label="ch.provider_name"
                :value="ch.id"
              >
                <div style="display: flex; align-items: center; gap: 8px;">
                  <img v-if="ch.icon" :src="getIconUrl(ch.icon)" style="width: 20px; height: 20px; border-radius: 4px;" @error="handleIconError" />
                  <span>{{ ch.provider_name }}</span>
                </div>
              </el-option>
            </el-select>
          </el-form-item>
          <!-- 选中渠道后显示渠道信息卡片 -->
          <div v-if="form.channel_id" class="channel-info-card">
            <img v-if="selectedChannelIcon" :src="getIconUrl(selectedChannelIcon)" class="channel-info-icon" @error="handleIconError" />
            <el-icon v-else class="channel-info-icon-default"><Connection /></el-icon>
            <div class="channel-info-text">
              <span class="channel-info-name">{{ selectedChannelName }}</span>
              <span class="channel-info-sub">{{ selectedChannelIcon || '未设置图标' }}</span>
            </div>
          </div>
          <el-form-item label="渠道类型" prop="channel_type">
            <el-select v-model="form.channel_type" placeholder="请选择" style="width: 100%">
              <el-option label="公益" value="公益" />
              <el-option label="自建" value="自建" />
              <el-option label="商业" value="商业" />
            </el-select>
          </el-form-item>
        </div>

        <!-- 监测配置区 -->
        <div class="form-section">
          <div class="section-title">监测配置</div>
          <el-form-item label="监测频率">
            <div class="frequency-cards">
              <div
                v-for="f in frequencyOptions"
                :key="f.value"
                :class="['frequency-card', { active: form.check_interval === f.value }]"
                @click="form.check_interval = f.value"
              >
                <div class="freq-top">
                  <span class="freq-num">{{ f.value }}</span>
                  <span class="freq-unit">天</span>
                </div>
                <span class="freq-label">{{ f.label }}</span>
              </div>
            </div>
          </el-form-item>
          <el-form-item label="ClaudeCode 模拟">
            <div class="cc-sim-switch">
              <el-switch
                v-model="ccSimSwitch"
                active-text="开启"
                inactive-text="关闭"
              />
              <span class="cc-sim-desc">开启后对于名称包含 Claude 的模型将模拟 ClaudeCode 客户端请求进行监测</span>
            </div>
          </el-form-item>
          <el-form-item label="监测模型" prop="model_names">
            <div class="model-select-area">
              <!-- 加载中 -->
              <div v-if="fetchingModels" class="model-fetch-loading">
                <div class="fetch-loading-bar"><div class="fetch-loading-progress"></div></div>
                <span class="fetch-loading-text">正在获取可用模型<span class="loading-dots"></span></span>
              </div>
              <!-- 错误提示 -->
              <div v-else-if="fetchModelError" class="model-fetch-error">
                <el-icon><WarningFilled /></el-icon>
                <span>{{ fetchModelError }}</span>
                <el-button link type="primary" size="small" @click="handleFetchModels">重试</el-button>
              </div>
              <el-select
                v-model="form.model_names"
                multiple
                filterable
                allow-create
                placeholder="选择渠道后自动获取，也可手动输入模型名"
                style="width: 100%"
                :multiple-limit="4"
                :loading="fetchingModels"
              >
                <el-option v-for="m in availableModels" :key="m" :label="m" :value="m" />
              </el-select>
              <div class="model-count-bar">
                <div class="model-count-fill" :style="{ width: (form.model_names.length / 4 * 100) + '%' }"></div>
              </div>
              <div class="model-count-tip">
                已选择 <strong>{{ form.model_names.length }}</strong> / 4 个模型
              </div>
            </div>
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button class="dialog-btn cancel-btn" @click="drawerVisible = false">取消</el-button>
          <el-button class="dialog-btn submit-btn" :loading="submitting" @click="handleSubmit">
            {{ drawerMode === 'create' ? '创建' : '保存' }}
          </el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search, Refresh, Connection, Loading, WarningFilled, QuestionFilled } from '@element-plus/icons-vue'
import { monitorAPI, externalChannelAPI } from '@/api/index.js'
import { getIconUrl } from '@/config/lobeIcons.js'

const props = defineProps({
  loading: Boolean
})

const emit = defineEmits(['fetch-data'])

// 监测频率选项
const frequencyOptions = [
  { value: 1, label: '每天检测' },
  { value: 3, label: '每3天检测' },
  { value: 7, label: '每周检测' }
]

// 搜索
const searchName = ref('')
const searchType = ref('')
const searchStatus = ref('')

// 列表
const configList = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)

// 展开行
const expandedRows = ref([])

// 抽屉
const drawerVisible = ref(false)
const drawerMode = ref('create')
const editingId = ref(null)
const formRef = ref(null)
const submitting = ref(false)
const form = ref({
  channel_id: null,
  channel_type: '',
  check_interval: 1,
  cc_sim_enabled: 'enabled',
  model_names: []
})

// CC模拟开关（boolean <-> string 映射）
const ccSimSwitch = computed({
  get: () => form.value.cc_sim_enabled !== 'disabled',
  set: (val) => { form.value.cc_sim_enabled = val ? 'enabled' : 'disabled' }
})

const formRules = {
  channel_id: [{ required: true, message: '请选择渠道', trigger: 'change' }],
  channel_type: [{ required: true, message: '请选择渠道类型', trigger: 'change' }],
  model_names: [
    { required: true, type: 'array', min: 1, message: '至少选择1个模型', trigger: 'change' },
    { type: 'array', max: 6, message: '最多选择6个模型', trigger: 'change' }
  ]
}

// 渠道列表
const enabledChannels = ref([])
const fetchingModels = ref(false)
const availableModels = ref([])
const fetchModelError = ref('')

// 选中渠道信息
const selectedChannelName = computed(() => {
  const ch = enabledChannels.value.find(c => c.id === form.value.channel_id)
  return ch?.provider_name || ''
})
const selectedChannelIcon = computed(() => {
  const ch = enabledChannels.value.find(c => c.id === form.value.channel_id)
  return ch?.icon || ''
})

// 初始化
onMounted(() => {
  fetchList()
  fetchEnabledChannels()
})

// 获取列表
const fetchList = async () => {
  loading.value = true
  try {
    const res = await monitorAPI.getConfigs({
      channel_name: searchName.value,
      channel_type: searchType.value,
      status: searchStatus.value,
      page: page.value,
      page_size: pageSize.value
    })
    const oldDetailMap = {}
    configList.value.forEach(old => { if (old._detail) oldDetailMap[old.id] = old._detail })
    configList.value = (res.list || []).map(item => ({
      ...item,
      _detail: oldDetailMap[item.id] || null,
      _detailLoading: false,
      _checking: false
    }))
    total.value = res.total || 0
  } catch (error) {
    if (!error.handled && !error.silent) {
      ElMessage.error('获取监测列表失败')
    }
  } finally {
    loading.value = false
  }
}

// 获取启用的外部渠道
const fetchEnabledChannels = async () => {
  try {
    const res = await externalChannelAPI.getList()
    enabledChannels.value = (res.list || []).filter(ch => ch.status === 'active')
  } catch {
    // 静默
  }
}

// 搜索
const handleQuery = () => {
  page.value = 1
  fetchList()
}
const handleReset = () => {
  searchName.value = ''
  searchType.value = ''
  searchStatus.value = ''
  page.value = 1
  fetchList()
}

// 展开行
const handleExpandChange = async (row, expandedRowsList) => {
  expandedRows.value = expandedRowsList.map(r => r.id)
  if (expandedRowsList.includes(row) && !row._detail) {
    await loadDetail(row)
  }
}

const handleExpand = async (row) => {
  const idx = expandedRows.value.indexOf(row.id)
  if (idx > -1) {
    expandedRows.value.splice(idx, 1)
  } else {
    expandedRows.value.push(row.id)
    if (!row._detail) {
      await loadDetail(row)
    }
  }
}

const loadDetail = async (row) => {
  row._detailLoading = true
  try {
    const res = await monitorAPI.getConfigDetail(row.id)
    row._detail = res
  } catch {
    ElMessage.error('获取详情失败')
  } finally {
    row._detailLoading = false
  }
}

// 启用/禁用
// 主动监测
const handleManualCheck = async (row) => {
  row._checking = true
  try {
    await monitorAPI.triggerCheck(row.id)
    ElMessage.success('监测已触发，请稍后展开查看结果')
    // 清除本地缓存的详情，下次展开时会重新从后端获取
    row._detail = null
  } catch (error) {
    if (!error.handled && !error.silent) {
      ElMessage.error(error?.response?.data?.msg || error?.msg || '触发监测失败')
    }
  } finally {
    row._checking = false
  }
}

// 删除
const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除渠道「${row.channel_name}」的监测配置吗？`,
      '确认删除',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning',
        customClass: 'token-action-dialog',
        confirmButtonClass: 'token-action-confirm-btn danger',
        cancelButtonClass: 'token-action-cancel-btn'
      }
    )
    await monitorAPI.deleteConfig(row.id)
    ElMessage.success('已删除')
    fetchList()
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error('删除失败')
    }
  }
}

const handleToggleStatus = async (row) => {
  const newStatus = row.status === 'enabled' ? 'disabled' : 'enabled'
  const action = newStatus === 'enabled' ? '启用' : '禁用'
  try {
    await ElMessageBox.confirm(
      `确定${action}该监测配置？`,
      '提示',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
        customClass: 'token-action-dialog',
        confirmButtonClass: 'token-action-confirm-btn',
        cancelButtonClass: 'token-action-cancel-btn'
      }
    )
    await monitorAPI.toggleConfigStatus(row.id, newStatus)
    ElMessage.success(`${action}成功`)
    fetchList()
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error(`${action}失败`)
    }
  }
}

// 抽屉操作
const openCreateDrawer = () => {
  drawerMode.value = 'create'
  editingId.value = null
  resetForm()
  drawerVisible.value = true
}

const openEditDrawer = async (row) => {
  drawerMode.value = 'edit'
  editingId.value = row.id
  form.value = {
    channel_id: row.channel_id,
    channel_type: row.channel_type,
    check_interval: row.check_interval || 1,
    cc_sim_enabled: row.cc_sim_enabled || 'enabled',
    model_names: row.models?.map(m => m.model_name) || []
  }
  drawerVisible.value = true
  // 自动获取模型列表（补充可选项，不清空已选）
  if (row.channel_id) {
    fetchingModels.value = true
    fetchModelError.value = ''
    try {
      const res = await monitorAPI.getChannelModels(row.channel_id)
      availableModels.value = res.models || []
    } catch {
      // 编辑模式静默失败，已有模型仍可用
    } finally {
      fetchingModels.value = false
    }
  }
}

const resetForm = () => {
  form.value = { channel_id: null, channel_type: '', check_interval: 1, cc_sim_enabled: 'enabled', model_names: [] }
  availableModels.value = []
  fetchModelError.value = ''
  formRef.value?.resetFields()
}

const handleChannelChange = () => {
  form.value.model_names = []
  availableModels.value = []
  fetchModelError.value = ''
  // 自动获取模型
  if (form.value.channel_id) {
    handleFetchModels()
  }
}

// 获取可用模型（选择渠道后自动调用）
const handleFetchModels = async () => {
  if (!form.value.channel_id) return
  fetchingModels.value = true
  fetchModelError.value = ''
  availableModels.value = []
  try {
    const res = await monitorAPI.getChannelModels(form.value.channel_id)
    const models = res.models || []
    if (models.length > 0) {
      availableModels.value = models
    } else {
      fetchModelError.value = '该渠道未返回可用模型，可手动输入模型名称'
    }
  } catch (error) {
    fetchModelError.value = '获取模型列表失败，可手动输入模型名称'
  } finally {
    fetchingModels.value = false
  }
}

// 提交
const handleSubmit = async () => {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    ElMessage.error('请检查表单填写')
    return
  }

  submitting.value = true
  try {
    const data = {
      channel_id: form.value.channel_id,
      channel_type: form.value.channel_type,
      check_interval: form.value.check_interval,
      cc_sim_enabled: form.value.cc_sim_enabled,
      model_names: form.value.model_names
    }
    if (drawerMode.value === 'create') {
      await monitorAPI.createConfig(data)
      ElMessage.success('创建成功')
    } else {
      await monitorAPI.updateConfig(editingId.value, data)
      ElMessage.success('更新成功')
    }
    drawerVisible.value = false
    fetchList()
  } catch (error) {
    if (!error.handled && !error.silent) {
      ElMessage.error(drawerMode.value === 'create' ? '创建失败' : '更新失败')
    }
  } finally {
    submitting.value = false
  }
}

// 工具方法
const msToSec = (ms) => ((ms || 0) / 1000).toFixed(2)

const statusLabel = (s) => {
  if (s === 'normal') return '正常'
  if (s === 'delayed') return '延迟'
  return '错误'
}

const typeTagType = (t) => {
  if (t === '公益') return 'success'
  if (t === '自建') return 'primary'
  return 'warning'
}

const getAvailabilityClass = (pct) => {
  if (pct >= 90) return 'high'
  if (pct >= 50) return 'medium'
  return 'low'
}

const getDayStatus = (stat) => {
  if (!stat || stat.total_checks === 0) return 'empty'
  if (stat.error_count > stat.normal_count) return 'error'
  if (stat.delayed_count > stat.normal_count) return 'delayed'
  return 'normal'
}

const dayStatusLabel = (status) => {
  if (status === 'normal') return '正常'
  if (status === 'delayed') return '延迟'
  if (status === 'error') return '异常'
  return '无数据'
}

const formatStatDate = (dateStr, lastCheckedAt) => {
  // 优先使用精确的最后检测时间（含时分秒）
  const src = lastCheckedAt || dateStr
  if (!src) return '-'
  const d = new Date(src)
  if (isNaN(d.getTime())) return src.split('T')[0] || src
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  // 如果有精确时间则显示时分秒
  if (lastCheckedAt) {
    const h = String(d.getHours()).padStart(2, '0')
    const min = String(d.getMinutes()).padStart(2, '0')
    const sec = String(d.getSeconds()).padStart(2, '0')
    return `${y}/${m}/${day} ${h}:${min}:${sec}`
  }
  return `${y}/${m}/${day}`
}

const formatDate = (dateStr) => {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}

const isOverdue = (dateStr) => {
  if (!dateStr) return false
  return new Date(dateStr).getTime() < Date.now()
}

const formatRelativeTime = (dateStr) => {
  if (!dateStr) return '-'
  const target = new Date(dateStr).getTime()
  const now = Date.now()
  const diff = target - now
  if (diff <= 0) {
    // 已过期，显示"即将执行"
    const past = -diff
    if (past < 60 * 1000) return '即将执行'
    if (past < 3600 * 1000) return `${Math.floor(past / 60000)}分钟前应执行`
    return formatDate(dateStr)
  }
  // 未来时间
  if (diff < 60 * 1000) return '不到1分钟'
  if (diff < 3600 * 1000) return `${Math.floor(diff / 60000)}分钟后`
  if (diff < 86400 * 1000) {
    const h = Math.floor(diff / 3600000)
    const m = Math.floor((diff % 3600000) / 60000)
    return m > 0 ? `${h}小时${m}分钟后` : `${h}小时后`
  }
  const d = Math.floor(diff / 86400000)
  const h = Math.floor((diff % 86400000) / 3600000)
  const m = Math.floor((diff % 3600000) / 60000)
  if (h > 0 && m > 0) return `${d}天${h}小时${m}分钟后`
  if (h > 0) return `${d}天${h}小时后`
  if (m > 0) return `${d}天${m}分钟后`
  return `${d}天后`
}

// 将 daily_stats 对齐到今天的14天窗口，确保最右侧(NOW)始终对应今天
const alignDailyStats = (dailyStats) => {
  const result = []
  const today = new Date()
  const statsMap = {}
  if (dailyStats) {
    for (const stat of dailyStats) {
      statsMap[stat.date] = stat
    }
  }
  // 从13天前到今天，共14天
  for (let i = 13; i >= 0; i--) {
    const d = new Date(today)
    d.setDate(d.getDate() - i)
    const y = d.getFullYear()
    const m = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    const dateStr = `${y}-${m}-${day}`
    if (statsMap[dateStr]) {
      result.push(statsMap[dateStr])
    } else {
      result.push({ date: dateStr, total_checks: 0 })
    }
  }
  return result
}

const handleIconError = (e) => {
  e.target.style.display = 'none'
}
</script>

<style scoped>
/* ===== 基础面板布局（与 ExternalChannelPanel 一致） ===== */
.panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.monitor-panel .content-card {
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
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}
.card-header h3 {
  font-size: 16px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  margin: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}
.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.filter-section {
  padding: 12px 20px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}
.channel-filter {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.channel-filter .filter-inputs {
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
  color: #fff !important;
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
.add-token-btn {
  background: var(--primary, oklch(0.21 0.006 285.885)) !important;
  border: none !important;
  border-radius: var(--radius-md, 8px) !important;
  color: #fff !important;
  font-weight: 500 !important;
  padding: 8px 16px !important;
  transition: all 0.2s ease !important;
}
.add-token-btn:hover {
  background: oklch(0.3 0.006 285.885) !important;
  transform: translateY(-1px);
}
.add-token-btn .el-icon {
  margin-right: 4px;
}

/* ===== 搜索区域 ===== */
.filter-inputs {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}

/* ===== 模型状态摘要 ===== */
.model-status-summary {
  display: grid;
  grid-template-columns: repeat(2, auto);
  gap: 4px 8px;
  justify-content: start;
  align-items: center;
}
.status-dot {
  font-size: 13px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
  white-space: nowrap;
}
.status-dot.normal { color: #22c55e; background: #f0fdf4; }
.status-dot.delayed { color: #f59e0b; background: #fffbeb; }
.status-dot.error { color: #ef4444; background: #fef2f2; }
.status-dot.pending { color: #6366f1; background: #eef2ff; }
.status-dot.empty-status { color: var(--muted-foreground, oklch(0.552 0.016 285.938)); }

.model-count-cell {
  font-size: 13px;
  color: #475569;
  font-weight: 500;
}
.freq-cell {
  font-size: 13px;
  color: #475569;
}
.last-check-cell {
  font-size: 13px;
  color: #475569;
}
.last-check-cell.empty {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}
.next-check-cell {
  font-size: 13px;
  color: #475569;
  font-weight: 500;
}
.next-check-cell.overdue {
  color: #f59e0b;
}
.next-check-cell.disabled {
  color: var(--ring, oklch(0.705 0.015 286.067));
}
.next-check-cell.empty {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

/* ===== 展开行卡片 ===== */
@keyframes expandFadeIn {
  from {
    opacity: 0;
    transform: translateY(-8px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
.expand-content {
  padding: 16px 20px;
  animation: expandFadeIn 0.25s ease-out;
}
.model-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}
@media (max-width: 1400px) {
  .model-cards { grid-template-columns: repeat(2, 1fr); }
}
@media (max-width: 900px) {
  .model-cards { grid-template-columns: 1fr; }
}

@keyframes cardSlideIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
.model-card {
  background: var(--card, oklch(1 0 0));
  border: 1px solid #e8ecf0;
  border-radius: var(--radius-md, 8px);
  padding: 12px;
  transition: box-shadow 0.2s;
  animation: cardSlideIn 0.3s ease-out both;
}
.model-card:nth-child(1) { animation-delay: 0s; }
.model-card:nth-child(2) { animation-delay: 0.05s; }
.model-card:nth-child(3) { animation-delay: 0.1s; }
.model-card:nth-child(4) { animation-delay: 0.15s; }
.model-card:hover {
  box-shadow: 0 4px 16px rgba(0,0,0,0.06);
}

/* 卡片顶部 */
.card-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 10px;
}
.card-top-left {
  display: flex;
  gap: 6px;
  align-items: center;
  flex: 1;
  min-width: 0;
}
.provider-icon {
  width: 26px;
  height: 26px;
  border-radius: 5px;
  object-fit: contain;
  flex-shrink: 0;
  background: #f8f9fa;
  padding: 2px;
}
.provider-icon-default {
  width: 26px;
  height: 26px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}
.card-title-area {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.card-model-name {
  font-size: 12px;
  font-weight: 700;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  letter-spacing: -0.2px;
}
.card-channel-sub {
  font-size: 10px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  margin-top: 1px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.status-badge {
  flex-shrink: 0;
  border-radius: var(--radius-sm, 6px) !important;
  font-weight: 500;
}

/* 延迟指标 */
.card-metrics {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}
.metric-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}
.metric-val {
  font-size: 16px;
  font-weight: 800;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  line-height: 1;
}
.metric-unit {
  font-size: 10px;
  font-weight: 500;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  margin-left: 1px;
}
.metric-desc {
  font-size: 10px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-weight: 500;
}
.metric-divider {
  width: 1px;
  height: 22px;
  background: #e8ecf0;
  flex-shrink: 0;
}

/* 状态和可用性 */
.card-status-section {
  margin-bottom: 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}
.status-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}
.status-label, .availability-label {
  font-size: 11px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}
.status-value {
  font-size: 11px;
  font-weight: 600;
}
.status-value.normal { color: #22c55e; }
.status-value.delayed { color: #f59e0b; }
.status-value.error { color: #ef4444; }

.availability-row {
  margin-bottom: 2px;
}
.availability-detail {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}
.availability-count {
  font-size: 10px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}
.availability-pct {
  font-size: 13px;
  font-weight: 700;
}
.availability-pct.high { color: #22c55e; }
.availability-pct.medium { color: #f59e0b; }
.availability-pct.low { color: #ef4444; }

/* 历史图表 */
.card-history {
  margin-top: 2px;
}
.history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}
.history-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  letter-spacing: 0.5px;
}
.history-bars {
  display: flex;
  gap: 1px;
  height: 8px;
}
.history-bars .el-tooltip__trigger {
  flex: 1;
  min-width: 3px;
  height: 100%;
}
.history-bar {
  width: 100%;
  height: 100%;
  border-radius: 2px;
  cursor: pointer;
  transition: opacity 0.15s;
}
.history-bar:hover { opacity: 0.7; }
.history-bar.normal { background: #22c55e; }
.history-bar.delayed { background: #f59e0b; }
.history-bar.error { background: #ef4444; }
.history-bar.empty { background: var(--border, oklch(0.92 0.004 286.32)); }
.history-footer {
  display: flex;
  justify-content: space-between;
  margin-top: 4px;
  font-size: 10px;
  color: var(--ring, oklch(0.705 0.015 286.067));
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* ===== 抽屉 ===== */
.icon-preview {
  display: flex;
  align-items: center;
  gap: 10px;
}
.preview-icon {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm, 6px);
  background: #f8f9fa;
  padding: 2px;
}
.placeholder-text {
  color: #c0c4cc;
  font-size: 13px;
}
/* ===== 监测频率卡片 ===== */
.frequency-cards {
  display: flex;
  gap: 12px;
  width: 100%;
}
.frequency-card {
  flex: 1;
  display: flex;
  align-items: baseline;
  justify-content: center;
  gap: 2px;
  padding: 12px 10px;
  border: 2px solid var(--border, oklch(0.92 0.004 286.32));
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  background: #fafbfc;
}
.frequency-card:hover {
  border-color: #a3bffa;
  background: #f0f5ff;
}
.frequency-card.active {
  border-color: var(--primary, oklch(0.21 0.006 285.885));
  background: #eff6ff;
  box-shadow: 0 0 0 3px rgba(0, 100, 250, 0.1);
}
.frequency-value {
  font-size: 22px;
  font-weight: 700;
  color: #475569;
}
.frequency-unit {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-weight: 500;
}
/* ===== 抽屉表单 ===== */
.drawer-form {
  padding: 0 4px;
}
.form-section {
  margin-bottom: 20px;
  padding: 16px;
  background: var(--secondary, oklch(0.967 0.001 286.375));
  border-radius: 10px;
  border: 1px solid #eef2f7;
}
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  margin-bottom: 14px;
  padding-bottom: 10px;
  border-bottom: 1px solid #e8ecf0;
  display: flex;
  align-items: center;
  gap: 6px;
}
.section-title::before {
  content: '';
  width: 3px;
  height: 14px;
  background: var(--primary, oklch(0.21 0.006 285.885));
  border-radius: 2px;
}

/* 选中渠道信息卡片 */
.channel-info-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: var(--card, oklch(1 0 0));
  border-radius: var(--radius-md, 8px);
  border: 1px solid #e8ecf0;
  margin-bottom: 16px;
}
.channel-info-icon {
  width: 36px;
  height: 36px;
  border-radius: var(--radius-md, 8px);
  object-fit: contain;
  background: var(--border, oklch(0.92 0.004 286.32));
  padding: 4px;
  flex-shrink: 0;
}
.channel-info-icon-default {
  width: 36px;
  height: 36px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  flex-shrink: 0;
}
.channel-info-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.channel-info-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}
.channel-info-sub {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

/* 监测频率卡片 */
.frequency-cards {
  display: flex;
  gap: 8px;
  width: 100%;
}
.frequency-card {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 8px 6px 6px;
  border: 1.5px solid var(--border, oklch(0.92 0.004 286.32));
  border-radius: var(--radius-md, 8px);
  cursor: pointer;
  transition: all 0.2s ease;
  background: var(--card, oklch(1 0 0));
}
.frequency-card:hover {
  border-color: #93b5f7;
  background: #f8fbff;
}
.frequency-card.active {
  border-color: var(--primary, oklch(0.21 0.006 285.885));
  background: #eff6ff;
  box-shadow: 0 0 0 2px rgba(0, 100, 250, 0.08);
}
.freq-top {
  display: flex;
  align-items: baseline;
  gap: 1px;
}
.freq-num {
  font-size: 18px;
  font-weight: 800;
  color: #475569;
  line-height: 1;
}
.freq-unit {
  font-size: 10px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-weight: 600;
}
.freq-label {
  font-size: 10px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}
.frequency-card.active .freq-num {
  color: var(--primary, oklch(0.21 0.006 285.885));
}
.frequency-card.active .freq-unit,
.frequency-card.active .freq-label {
  color: #60a5fa;
}

/* CC模拟开关 */
.cc-sim-switch {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.cc-sim-desc {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  line-height: 1.5;
}

/* 模型选择区域 */
.model-select-area {
  width: 100%;
}
.model-count-bar {
  margin-top: 8px;
  width: 100%;
  height: 4px;
  background: var(--border, oklch(0.92 0.004 286.32));
  border-radius: 4px;
  overflow: hidden;
}
.model-count-fill {
  height: 100%;
  background: linear-gradient(90deg, #60a5fa, #0064FA);
  border-radius: 4px;
  transition: width 0.3s ease;
}
.model-count-tip {
  margin-top: 6px;
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}
.model-count-tip strong {
  color: var(--primary, oklch(0.21 0.006 285.885));
}

/* 加载动画 */
.model-fetch-loading {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 10px;
  padding: 12px 14px;
  background: var(--card, oklch(1 0 0));
  border-radius: var(--radius-md, 8px);
  border: 1px solid #d4e5ff;
}
.fetch-loading-bar {
  width: 100%;
  height: 4px;
  background: #dbeafe;
  border-radius: 4px;
  overflow: hidden;
}
.fetch-loading-progress {
  width: 40%;
  height: 100%;
  background: linear-gradient(90deg, #60a5fa, #3b82f6, #60a5fa);
  background-size: 200% 100%;
  border-radius: 4px;
  animation: fetchSlide 1.5s ease-in-out infinite;
}
@keyframes fetchSlide {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(350%); }
}
.fetch-loading-text {
  font-size: 13px;
  color: #3b82f6;
  font-weight: 500;
}
.loading-dots::after {
  content: '';
  animation: loadingDots 1.4s steps(4, end) infinite;
}
@keyframes loadingDots {
  0% { content: ''; }
  25% { content: '.'; }
  50% { content: '..'; }
  75% { content: '...'; }
}
.model-fetch-error {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #f56c6c;
  margin-bottom: 8px;
  background: #fef0f0;
  padding: 8px 12px;
  border-radius: var(--radius-sm, 6px);
  border: 1px solid #fde2e2;
}

/* ===== 分页 ===== */
.pagination-wrapper {
  margin-top: 16px;
  display: flex;
  justify-content: center;
}

/* ===== 操作按钮 ===== */
.action-buttons {
  display: flex;
  gap: 8px;
  justify-content: center;
}

/* ===== 抽屉底部 ===== */
.dialog-footer {
  display: flex;
  justify-content: center;
  gap: 12px;
}
.dialog-btn {
  min-width: 100px;
  padding: 10px 24px !important;
  border-radius: var(--radius-md, 8px) !important;
  font-weight: 500 !important;
}
.cancel-btn {
  background: var(--card, oklch(1 0 0)) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32)) !important;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938)) !important;
}
.cancel-btn:hover { border-color: var(--ring, oklch(0.705 0.015 286.067)) !important; color: var(--foreground, oklch(0.141 0.005 285.823)) !important; background: var(--secondary, oklch(0.967 0.001 286.375)) !important; }
.submit-btn {
  background: var(--primary, oklch(0.21 0.006 285.885)) !important;
  border: none !important;
  color: #fff !important;
}
.submit-btn:hover:not(:disabled) { background: oklch(0.3 0.006 285.885) !important; }
</style>

<style>
/* 历史图表 Tooltip 样式（全局，el-tooltip 渲染在 body） */
.history-tooltip.el-popper {
  padding: 0 !important;
  border-radius: var(--radius-xl, 14px) !important;
  border: 1px solid #e8ecf0 !important;
  box-shadow: 0 8px 24px rgba(0,0,0,0.12) !important;
  min-width: 200px;
}
.ht-empty {
  padding: 10px 14px;
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  text-align: center;
}
.ht-content {
  padding: 14px 16px;
}
.ht-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.ht-badge {
  display: inline-block;
  padding: 2px 10px;
  border-radius: var(--radius-sm, 6px);
  font-size: 12px;
  font-weight: 600;
}
.ht-badge.normal { background: #f0fdf4; color: #22c55e; }
.ht-badge.delayed { background: #fffbeb; color: #f59e0b; }
.ht-badge.error { background: #fef2f2; color: #ef4444; }
.ht-date {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}
.ht-metrics {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 10px;
}
.ht-metric-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
}
.ht-metric-label {
  font-size: 13px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}
.ht-metric-val {
  font-size: 15px;
  font-weight: 700;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}
.ht-unit {
  font-size: 12px;
  font-weight: 500;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  margin-left: 2px;
}
.ht-error {
  font-size: 12px;
  color: #ef4444;
  background: #fef2f2;
  border-radius: var(--radius-sm, 6px);
  padding: 6px 10px;
  margin-bottom: 10px;
  line-height: 1.4;
  word-break: break-all;
}
.ht-summary {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  padding-top: 8px;
  border-top: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

/* 暗黑模式 - 历史 Tooltip */
:root.dark-mode .history-tooltip.el-popper {
  background: #1e1e22 !important;
  border-color: rgba(255,255,255,0.08) !important;
  box-shadow: 0 8px 24px rgba(0,0,0,0.4) !important;
}
:root.dark-mode .ht-empty { color: #86868b; }
:root.dark-mode .ht-badge.normal { background: rgba(34,197,94,0.15); color: #4ade80; }
:root.dark-mode .ht-badge.delayed { background: rgba(234,179,8,0.15); color: #fbbf24; }
:root.dark-mode .ht-badge.error { background: rgba(239,68,68,0.15); color: #f87171; }
:root.dark-mode .ht-date { color: #86868b; }
:root.dark-mode .ht-metric-label { color: #86868b; }
:root.dark-mode .ht-metric-val { color: #f5f5f7; }
:root.dark-mode .ht-unit { color: #86868b; }
:root.dark-mode .ht-error { color: #f87171; background: rgba(239,68,68,0.1); }
:root.dark-mode .ht-summary { color: #86868b; border-top-color: rgba(255,255,255,0.08); }
</style>
