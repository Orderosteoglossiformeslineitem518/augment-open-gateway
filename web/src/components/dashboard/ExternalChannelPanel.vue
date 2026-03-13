<template>
  <div class="panel channels-panel">
    <div class="content-card">
      <div class="card-header">
        <h3>
          外部渠道列表
          <el-tooltip content="外部渠道仅支持标准claude接口格式，不支持openai格式" placement="top">
            <el-icon class="help-icon"><QuestionFilled /></el-icon>
          </el-tooltip>
        </h3>
        <div class="header-actions">
          <el-button class="add-token-btn" @click="openCreateChannelDialog">
            <el-icon><Plus /></el-icon>
            添加渠道
          </el-button>
        </div>
      </div>
      <div class="filter-section channel-filter">
        <div class="filter-inputs">
          <el-input 
            v-model="searchKeywordLocal" 
            placeholder="搜索供应商名称或API地址" 
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
          <el-tooltip :content="apiAddressVisible ? '隐藏API地址' : '显示API地址'" placement="top">
            <el-button class="visibility-btn" @click="toggleApiAddressVisibility">
              <el-icon><View v-if="apiAddressVisible" /><Hide v-else /></el-icon>
            </el-button>
          </el-tooltip>
          <el-button class="reset-btn" @click="handleQueryReset">重置</el-button>
          <el-button class="query-btn" @click="handleQuery">查询</el-button>
        </div>
      </div>
      <div class="card-body">
        <div class="table-wrapper">
        <el-table :data="filteredChannels" v-loading="loading" style="width: 100%" height="100%" empty-text="暂无外部渠道" table-layout="fixed">
          <el-table-column label="图标" width="70" align="center">
            <template #default="{ row }">
              <div class="channel-icon-cell">
                <img v-if="row.icon" :src="getIconUrl(row.icon)" :alt="row.icon" class="channel-icon" @error="handleTableIconError" />
                <el-icon v-else class="default-icon"><Connection /></el-icon>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="provider_name" label="供应商名称" width="140" show-overflow-tooltip />
          <el-table-column label="备注" width="120">
            <template #default="{ row }">
              <el-tooltip v-if="row.remark && row.remark.length > 15" :content="row.remark" placement="top">
                <span class="remark-text">{{ row.remark.substring(0, 15) }}...</span>
              </el-tooltip>
              <span v-else-if="row.remark" class="remark-text">{{ row.remark }}</span>
              <span v-else class="no-remark">-</span>
            </template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="80" align="center">
            <template #default="{ row }">
              <el-tooltip
                v-if="row.status === 'active' && row.is_bound"
                content="该渠道已被绑定，无法禁用"
                placement="top"
              >
                <el-tag type="success" size="small" class="status-tag-disabled">
                  启用
                </el-tag>
              </el-tooltip>
              <el-tag
                v-else
                :type="row.status === 'active' ? 'success' : 'danger'"
                size="small"
                class="status-tag-clickable"
                @click="openStatusDialog(row)"
              >
                {{ row.status === 'active' ? '启用' : '禁用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="api_endpoint" label="API地址" width="250" show-overflow-tooltip>
            <template #default="{ row }">
              <span :class="['api-endpoint-text', { 'api-address-blur': !apiAddressVisible }]">{{ row.api_endpoint }}</span>
            </template>
          </el-table-column>
          <el-table-column label="模型映射" min-width="380">
            <template #default="{ row }">
              <div class="model-mapping-tags" v-if="row.models?.length > 0">
                <el-tag v-for="m in row.models" :key="m.id" size="small" type="info" class="model-tag">
                  {{ m.internal_model }} → {{ m.external_model }}
                </el-tag>
              </div>
              <span v-else class="no-mapping">未配置</span>
            </template>
          </el-table-column>
          <el-table-column label="测试延迟" width="100" align="center">
            <template #default="{ row }">
              <span v-if="row.last_test_latency !== null && row.last_test_latency !== undefined" class="latency-text">
                {{ (row.last_test_latency / 1000).toFixed(2) }}s
              </span>
              <span v-else class="no-latency">-</span>
            </template>
          </el-table-column>
          <el-table-column label="模拟思考" width="90" align="center">
            <template #default="{ row }">
              <el-tag
                :type="row.thinking_signature_enabled !== 'disabled' ? 'success' : 'info'"
                size="small"
              >
                {{ row.thinking_signature_enabled !== 'disabled' ? '开启' : '关闭' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="使用记录" width="100" align="center">
            <template #default="{ row }">
              <el-button link type="primary" @click="openUsageStatsDialog(row)" class="usage-stats-btn">
                <el-icon :size="18"><TrendCharts /></el-icon>
              </el-button>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="150" show-overflow-tooltip />
          <el-table-column label="操作" width="160" fixed="right" align="center">
            <template #default="{ row }">
              <div class="action-buttons">
                <el-button link type="success" size="small" @click="handleTestChannel(row)" :loading="testingChannelId === row.id">
                  测试
                </el-button>
                <el-button link type="primary" size="small" @click="openEditChannelDialog(row)">
                  编辑
                </el-button>
                <el-button link type="danger" size="small" @click="handleDeleteChannel(row)">
                  删除
                </el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        </div>
        <div class="pagination-wrapper" v-if="total > 0">
          <el-pagination
            v-model:current-page="pageLocal"
            v-model:page-size="pageSizeLocal"
            :page-sizes="[10, 20, 50, 100]"
            :total="total"
            layout="total, sizes, prev, pager, next"
          />
        </div>
      </div>
    </div>

    <!-- 外部渠道编辑抽屉 -->
    <el-drawer
      v-model="channelDrawerVisible"
      :title="channelDialogMode === 'create' ? '添加外部渠道' : '编辑外部渠道'"
      direction="rtl"
      size="640px"
      class="channel-drawer"
      :close-on-click-modal="true"
      @close="resetChannelForm"
    >
      <el-form ref="channelFormRef" :model="channelForm" :rules="channelFormRules" label-position="top" class="drawer-form">
        <!-- 基本信息 -->
        <div class="form-section">
          <div class="section-title">基本信息</div>
          <el-form-item label="供应商名称" prop="provider_name">
            <el-input v-model="channelForm.provider_name" placeholder="请输入供应商名称" maxlength="100" />
          </el-form-item>
          <el-form-item label="渠道图标" prop="icon">
            <IconSelector v-model="channelForm.icon" />
          </el-form-item>
          <el-form-item label="备注" prop="remark">
            <el-input v-model="channelForm.remark" type="textarea" :rows="2" placeholder="可选，输入备注信息" maxlength="500" />
          </el-form-item>
        </div>

        <!-- 连接配置 -->
        <div class="form-section">
          <div class="section-title">连接配置</div>
          <el-form-item label="API地址" prop="api_endpoint">
            <el-input v-model="channelForm.api_endpoint" placeholder="必填，如：https://api.anthropic.com/v1/messages">
              <template #suffix>
                <el-tooltip content="填充 /v1/messages 后缀" placement="top">
                  <el-icon class="fill-suffix-icon" @click="handleFillApiSuffix"><MagicStick /></el-icon>
                </el-tooltip>
              </template>
            </el-input>
            <div class="api-warning-tip">Tips:如需使用gpt系列模型，请保证渠道同时支持v1/messages与/v1/responses端点！</div>
          </el-form-item>
          <el-form-item label="API Key" prop="api_key">
            <el-input
              v-model="channelForm.api_key"
              type="password"
              show-password
              autocomplete="new-password"
              :placeholder="channelDialogMode === 'edit' ? '留空则不修改' : '必填，输入API Key'"
            />
            <div class="fetch-models-section">
              <el-button
                type="success"
                link
                :loading="fetchingModels"
                @click="handleFetchAvailableModels"
              >
                <el-icon v-if="!fetchingModels"><Menu /></el-icon>
                {{ fetchingModels ? '获取中...' : '获取可用模型' }}
              </el-button>
              <div v-if="availableModels.length > 0" class="available-models-list">
                <div class="models-tags">
                  <el-tag
                    v-for="model in displayedModels"
                    :key="model"
                    class="model-tag"
                    type="success"
                    effect="plain"
                    @click="copyModelName(model)"
                  >
                    {{ model }}
                  </el-tag>
                </div>
                <el-button
                  v-if="availableModels.length > 5 && !showAllModels"
                  type="success"
                  link
                  size="small"
                  @click="showAllModels = true"
                >
                  查看全部 ({{ availableModels.length }})
                </el-button>
                <el-button
                  v-if="showAllModels && availableModels.length > 5"
                  type="info"
                  link
                  size="small"
                  @click="showAllModels = false"
                >
                  收起
                </el-button>
              </div>
            </div>
          </el-form-item>
        </div>

        <!-- 对话模型映射配置 -->
        <div class="form-section">
          <div class="section-title">
            对话模型映射
            <el-tooltip content="未映射外部渠道时，请留意账户积分消耗" placement="top">
              <el-icon class="help-icon"><QuestionFilled /></el-icon>
            </el-tooltip>
          </div>
          <div class="model-mapping-section">
            <div v-for="(mapping, index) in channelForm.models" :key="index" class="model-mapping-row">
              <el-select v-model="mapping.internal_model" placeholder="选择Augment模型" class="model-select-internal">
                <el-option
                  v-for="model in internalModels"
                  :key="model"
                  :label="model"
                  :value="model"
                  :disabled="isInternalModelUsed(model, index)"
                />
              </el-select>
              <span class="mapping-arrow">→</span>
              <el-input v-model="mapping.external_model" placeholder="外部模型ID" class="model-input-external" @input="handleExternalModelInput(index)" />
              <el-select
                v-if="isGPTModel(mapping.internal_model)"
                v-model="mapping.reasoning_effort"
                placeholder="思考强度"
                class="reasoning-effort-select"
              >
                <el-option label="none" value="none" />
                <el-option label="low" value="low" />
                <el-option label="medium" value="medium" />
                <el-option label="high" value="high" />
                <el-option label="xhigh" value="xhigh" />
              </el-select>
              <el-button link type="danger" @click="removeModelMapping(index)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
            <el-button
              type="primary"
              link
              @click="addModelMapping"
              class="add-mapping-btn"
              :disabled="channelForm.models.length >= maxModelMappings"
            >
              <el-icon><Plus /></el-icon>
              添加模型映射 ({{ channelForm.models.length }}/{{ maxModelMappings }})
            </el-button>
          </div>
        </div>

        <!-- 底层模型映射配置 -->
        <div class="form-section">
          <div class="section-title">
            底层模型映射
            <el-tooltip content="用于标题生成和对话总结场景，非必填。配置后将优先使用指定模型处理对应请求" placement="top">
              <el-icon class="help-icon"><QuestionFilled /></el-icon>
            </el-tooltip>
          </div>
          <el-form-item label="标题生成模型">
            <el-input
              v-model="channelForm.title_generation_model_mapping"
              placeholder="选填，如：claude-3-5-haiku-20241022"
            />
            <div class="form-tip">用于底层生成对话标题，留空则使用首个对话模型映射，建议使用低消耗快速模型</div>
          </el-form-item>
          <el-form-item label="对话总结模型">
            <el-input
              v-model="channelForm.summary_model_mapping"
              placeholder="选填，如：claude-3-5-haiku-20241022"
            />
            <div class="form-tip">用于底层生成对话总结，留空则使用首个对话模型映射，建议使用低消耗快速模型</div>
          </el-form-item>
        </div>

        <!-- 高级配置（可折叠） -->
        <div class="form-section form-section-collapsible">
          <div class="section-title section-title-clickable" @click="advancedConfigExpanded = !advancedConfigExpanded">
            <el-icon class="collapse-arrow" :class="{ expanded: advancedConfigExpanded }"><ArrowRight /></el-icon>
            高级配置
          </div>
          <Transition name="collapse">
            <div v-show="advancedConfigExpanded" class="section-body">
              <el-form-item label="模拟思考" prop="thinking_signature_enabled">
                <div class="thinking-signature-switch">
                  <el-switch
                    v-model="channelForm.thinking_signature_enabled"
                    active-value="enabled"
                    inactive-value="disabled"
                    active-text="开启"
                    inactive-text="关闭"
                  />
                  <div class="form-tip">开启后插件侧未传输思考块时将使用预设签名避免报错 (2API渠道建议保持开启，官方渠道建议关闭)</div>
                </div>
              </el-form-item>
              <el-form-item label="CC模拟" prop="claude_code_simulation_enabled">
                <div class="thinking-signature-switch">
                  <el-switch
                    v-model="channelForm.claude_code_simulation_enabled"
                    active-value="enabled"
                    inactive-value="disabled"
                    active-text="开启"
                    inactive-text="关闭"
                  />
                  <div class="form-tip">开启后，调用 Claude 渠道时将使用 ClaudeCode 请求头进行客户端模拟（推荐保持开启以提高兼容性）</div>
                </div>
              </el-form-item>
            </div>
          </Transition>
        </div>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button class="dialog-btn cancel-btn" @click="channelDrawerVisible = false">取消</el-button>
          <el-button class="dialog-btn submit-btn" :loading="channelSubmitting" @click="handleChannelSubmit">
            {{ channelDialogMode === 'create' ? '添加' : '保存' }}
          </el-button>
        </div>
      </template>
    </el-drawer>

    <!-- 测试渠道弹窗 -->
    <el-dialog
      v-model="testDialogVisible"
      title="测试渠道"
      width="450px"
      class="test-dialog"
      :close-on-click-modal="false"
      @close="closeTestDialog"
    >
      <div class="test-dialog-content">
        <p class="test-tip">选择要测试的模型：</p>
        <el-select v-model="selectedTestModel" placeholder="请选择模型" style="width: 100%;" :disabled="testExecuting">
          <el-option
            v-for="m in currentTestChannel?.models || []"
            :key="m.id"
            :label="m.external_model"
            :value="m.external_model"
          >
            <span>{{ m.internal_model }} → {{ m.external_model }}</span>
          </el-option>
        </el-select>
        
        <!-- 测试结果显示区域 -->
        <div v-if="testResult" class="test-result-section">
          <div :class="['test-result-card', testResult.success ? 'success' : 'error']">
            <div class="result-header">
              <el-icon v-if="testResult.success" class="result-icon success"><CircleCheck /></el-icon>
              <el-icon v-else class="result-icon error"><CircleClose /></el-icon>
              <span class="result-title">{{ testResult.success ? '测试成功' : '测试失败' }}</span>
            </div>
            <div class="result-body">
              <div class="result-item">
                <span class="result-label">响应延迟：</span>
                <span class="result-value">{{ testResult.latency }}s</span>
              </div>
              <div class="result-item">
                <span class="result-label">返回信息：</span>
                <span class="result-value">{{ testResult.message }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button class="dialog-btn cancel-btn" @click="closeTestDialog">关闭</el-button>
          <el-button class="dialog-btn submit-btn" :loading="testExecuting" @click="executeTest">
            {{ testExecuting ? '测试中...' : (testResult ? '重新测试' : '开始测试') }}
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 使用统计弹窗 -->
    <el-dialog
      v-model="usageStatsDialogVisible"
      :title="`${currentStatsChannel?.provider_name || ''} - 使用统计`"
      width="560px"
      class="usage-stats-dialog"
      :close-on-click-modal="false"
      @close="closeUsageStatsDialog"
    >
      <div class="usage-stats-content">
        <div class="time-range-selector">
          <el-radio-group v-model="usageStatsDays" @change="fetchUsageStats">
            <el-radio-button :value="7">近 7 天</el-radio-button>
            <el-radio-button :value="15">近 15 天</el-radio-button>
            <el-radio-button :value="30">近 30 天</el-radio-button>
          </el-radio-group>
        </div>
        <div v-if="!usageStatsLoading && usageStatsData.length === 0" class="empty-data">
          <el-empty description="暂无使用记录" :image-size="60" />
        </div>
        <div v-else ref="usageChartRef" class="chart-container" v-loading="usageStatsLoading"></div>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button class="dialog-btn cancel-btn" @click="closeUsageStatsDialog">关闭</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 状态切换弹窗 -->
    <el-dialog
      v-model="statusDialogVisible"
      title="切换渠道状态"
      width="400px"
      class="status-dialog"
      :close-on-click-modal="false"
      @close="closeStatusDialog"
    >
      <div class="status-dialog-content">
        <p class="status-tip">
          渠道 <span class="channel-name">{{ currentStatusChannel?.provider_name }}</span> 当前状态为
          <el-tag :type="currentStatusChannel?.status === 'active' ? 'success' : 'danger'" size="small">
            {{ currentStatusChannel?.status === 'active' ? '启用' : '禁用' }}
          </el-tag>
        </p>
        <p class="status-question">确定要将状态切换为 
          <el-tag :type="currentStatusChannel?.status === 'active' ? 'danger' : 'success'" size="small">
            {{ currentStatusChannel?.status === 'active' ? '禁用' : '启用' }}
          </el-tag> 吗？
        </p>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button class="dialog-btn cancel-btn" @click="closeStatusDialog">取消</el-button>
          <el-button class="dialog-btn submit-btn" :loading="statusSubmitting" @click="handleStatusToggle">
            确认切换
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search, Edit, Delete, Connection, CircleCheck, CircleClose, QuestionFilled, TrendCharts, Menu, View, Hide, ArrowRight } from '@element-plus/icons-vue'
import { externalChannelAPI } from '@/api'
import IconSelector from '@/components/common/IconSelector.vue'
import { getIconUrl } from '@/config/lobeIcons'
import * as echarts from 'echarts'

const props = defineProps({
  externalChannels: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  page: { type: Number, default: 1 },
  pageSize: { type: Number, default: 10 },
  searchKeyword: { type: String, default: '' },
  internalModels: { type: Array, default: () => [] }
})

const emit = defineEmits(['update:page', 'update:pageSize', 'update:searchKeyword', 'fetchExternalChannels'])

// 本地状态
const pageLocal = ref(props.page)
const pageSizeLocal = ref(props.pageSize)
const searchKeywordLocal = ref(props.searchKeyword)

// API地址显示/隐藏状态（从 localStorage 初始化，默认为显示）
const apiAddressVisible = ref(localStorage.getItem('api_address_visible') !== 'false')

// 切换 API 地址显示/隐藏
const toggleApiAddressVisibility = () => {
  apiAddressVisible.value = !apiAddressVisible.value
  localStorage.setItem('api_address_visible', String(apiAddressVisible.value))
}

// 渠道抽屉
const channelDrawerVisible = ref(false)
const channelDialogMode = ref('create')
const channelFormRef = ref(null)
const channelSubmitting = ref(false)
const currentEditChannelId = ref(null)
const advancedConfigExpanded = ref(false)

// 测试渠道状态
const testingChannelId = ref(null)
const testDialogVisible = ref(false)
const currentTestChannel = ref(null)
const selectedTestModel = ref('')
const testExecuting = ref(false)
const testResult = ref(null)

// 使用统计状态
const usageStatsDialogVisible = ref(false)
const currentStatsChannel = ref(null)
const usageStatsDays = ref(7)
const usageStatsLoading = ref(false)
const usageStatsData = ref([])
const usageChartRef = ref(null)
let usageChartInstance = null

// 状态切换弹窗状态
const statusDialogVisible = ref(false)
const currentStatusChannel = ref(null)
const statusSubmitting = ref(false)

// 获取可用模型状态
const fetchingModels = ref(false)
const availableModels = ref([])
const showAllModels = ref(false)

const channelForm = ref({
  provider_name: '',
  remark: '',
  api_endpoint: '',
  api_key: '',
  icon: '',
  status: 'active',
  thinking_signature_enabled: 'enabled', // 思考签名开关，默认开启
  claude_code_simulation_enabled: 'enabled', // ClaudeCode客户端模拟开关，默认开启
  title_generation_model_mapping: '', // 标题生成模型映射
  summary_model_mapping: '', // 对话总结模型映射
  models: []
})

// 监听 props 变化
watch(() => props.page, (val) => { pageLocal.value = val })
watch(() => props.pageSize, (val) => { pageSizeLocal.value = val })
watch(() => props.searchKeyword, (val) => { searchKeywordLocal.value = val })

// 计算过滤后的渠道列表
const filteredChannels = computed(() => {
  let list = props.externalChannels
  if (searchKeywordLocal.value) {
    const keyword = searchKeywordLocal.value.toLowerCase()
    list = list.filter(c => 
      c.provider_name?.toLowerCase().includes(keyword) || 
      c.api_endpoint?.toLowerCase().includes(keyword)
    )
  }
  const start = (pageLocal.value - 1) * pageSizeLocal.value
  return list.slice(start, start + pageSizeLocal.value)
})

const total = computed(() => {
  let list = props.externalChannels
  if (searchKeywordLocal.value) {
    const keyword = searchKeywordLocal.value.toLowerCase()
    list = list.filter(c => 
      c.provider_name?.toLowerCase().includes(keyword) || 
      c.api_endpoint?.toLowerCase().includes(keyword)
    )
  }
  return list.length
})

// 最大模型映射数量，根据内部模型数量动态计算
const maxModelMappings = computed(() => props.internalModels.length || 0)

// 渠道表单验证规则
const channelFormRules = {
  provider_name: [{ required: true, message: '请输入供应商名称', trigger: 'blur' }],
  api_endpoint: [
    { required: true, message: '请输入API请求地址', trigger: 'blur' },
    {
      validator: (_, value, callback) => {
        if (value && !value.startsWith('http://') && !value.startsWith('https://')) {
          callback(new Error('API地址必须以http://或https://开头'))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ],
  api_key: [{
    validator: (_, value, callback) => {
      if (channelDialogMode.value === 'create' && !value) {
        callback(new Error('请输入API Key'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }]
}

// 查询相关
const handleQuery = () => {
  emit('update:searchKeyword', searchKeywordLocal.value)
  emit('update:page', 1)
  emit('fetchExternalChannels')
}

const handleQueryReset = () => {
  searchKeywordLocal.value = ''
  emit('update:searchKeyword', '')
  emit('update:page', 1)
  emit('fetchExternalChannels')
}

// 渠道抽屉操作
const openCreateChannelDialog = () => {
  channelDialogMode.value = 'create'
  currentEditChannelId.value = null
  resetChannelForm()
  channelDrawerVisible.value = true
}

const openEditChannelDialog = (channel) => {
  channelDialogMode.value = 'edit'
  currentEditChannelId.value = channel.id
  channelForm.value = {
    provider_name: channel.provider_name || '',
    remark: channel.remark || '',
    api_endpoint: channel.api_endpoint || '',
    api_key: '',
    icon: channel.icon || '',
    status: channel.status || 'active',
    thinking_signature_enabled: channel.thinking_signature_enabled || 'enabled', // 加载渠道的思考签名设置
    claude_code_simulation_enabled: channel.claude_code_simulation_enabled || 'enabled', // 加载ClaudeCode客户端模拟设置
    title_generation_model_mapping: channel.title_generation_model_mapping || '', // 标题生成模型映射
    summary_model_mapping: channel.summary_model_mapping || '', // 对话总结模型映射
    models: channel.models?.map(m => ({ internal_model: m.internal_model, external_model: m.external_model, reasoning_effort: m.reasoning_effort || 'medium' })) || []
  }
  channelDrawerVisible.value = true
}

const resetChannelForm = () => {
  channelForm.value = {
    provider_name: '',
    remark: '',
    api_endpoint: '',
    api_key: '',
    icon: '',
    status: 'active',
    thinking_signature_enabled: 'enabled', // 思考签名开关，默认开启
    claude_code_simulation_enabled: 'enabled', // ClaudeCode客户端模拟开关，默认开启
    title_generation_model_mapping: '', // 标题生成模型映射
    summary_model_mapping: '', // 对话总结模型映射
    models: []
  }
  // 重置获取模型列表相关状态
  availableModels.value = []
  showAllModels.value = false
  fetchingModels.value = false
  if (channelFormRef.value) {
    channelFormRef.value.clearValidate()
  }
}

// 模型映射操作
const addModelMapping = () => {
  if (channelForm.value.models.length >= maxModelMappings.value) {
    ElMessage.warning(`最多只能添加${maxModelMappings.value}个模型映射`)
    return
  }
  channelForm.value.models.push({ internal_model: '', external_model: '', reasoning_effort: 'medium' })
}

const removeModelMapping = (index) => {
  channelForm.value.models.splice(index, 1)
}

const isInternalModelUsed = (model, currentIndex) => {
  return channelForm.value.models.some((m, idx) => idx !== currentIndex && m.internal_model === model)
}

// 判断是否为GPT系列模型
const isGPTModel = (model) => {
  return model && model.startsWith('gpt-')
}

// 处理外部模型ID输入，自动移除空格
const handleExternalModelInput = (index) => {
  const value = channelForm.value.models[index].external_model
  if (value && value.includes(' ')) {
    channelForm.value.models[index].external_model = value.replace(/\s/g, '')
  }
}

// 一键填充 API 地址后缀
const handleFillApiSuffix = () => {
  const endpoint = channelForm.value.api_endpoint?.trim() || ''
  const suffix = '/v1/messages'

  // 如果已经包含后缀，不触发填充
  if (endpoint.endsWith(suffix)) {
    ElMessage.info('API 地址已包含 /v1/messages 后缀')
    return
  }

  // 移除末尾斜杠后添加后缀
  const cleanEndpoint = endpoint.replace(/\/+$/, '')
  channelForm.value.api_endpoint = cleanEndpoint + suffix
  ElMessage.success('已填充 /v1/messages 后缀')
}

// 获取可用模型列表
const handleFetchAvailableModels = async () => {
  const apiEndpoint = channelForm.value.api_endpoint?.trim()
  const apiKey = channelForm.value.api_key?.trim()
  const isEditMode = channelDialogMode.value === 'edit'

  if (!apiEndpoint) {
    ElMessage.warning('请先填写 API 地址')
    return
  }

  // 创建模式必须填写 API Key
  if (!isEditMode && !apiKey) {
    ElMessage.warning('请先填写 API Key')
    return
  }

  fetchingModels.value = true
  availableModels.value = []
  showAllModels.value = false

  try {
    let res
    if (isEditMode) {
      // 编辑模式：传递渠道 ID，让后端查询 API Key
      // 如果用户填写了新的 API Key，则使用新的
      if (apiKey) {
        res = await externalChannelAPI.fetchAvailableModels(apiEndpoint, apiKey, null)
      } else {
        res = await externalChannelAPI.fetchAvailableModels(apiEndpoint, null, currentEditChannelId.value)
      }
    } else {
      // 创建模式：传递 API Key
      res = await externalChannelAPI.fetchAvailableModels(apiEndpoint, apiKey, null)
    }

    const models = res.models
    if (models && models.length > 0) {
      availableModels.value = models
      ElMessage.success(`成功获取 ${models.length} 个模型`)
    } else {
      ElMessage.warning('未获取到可用模型')
    }
  } catch (error) {
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      const errorMsg = error.response?.data?.error || error.message || '获取模型列表失败'
      ElMessage.error(errorMsg)
    }
  } finally {
    fetchingModels.value = false
  }
}

// 复制模型名称到剪贴板
const copyModelName = async (modelName) => {
  try {
    await navigator.clipboard.writeText(modelName)
    ElMessage.success(`已复制: ${modelName}`)
  } catch {
    ElMessage.error('复制失败')
  }
}

// 显示的模型列表（前5个或全部）
const displayedModels = computed(() => {
  if (showAllModels.value || availableModels.value.length <= 5) {
    return availableModels.value
  }
  return availableModels.value.slice(0, 5)
})

// 提交渠道表单
const handleChannelSubmit = async () => {
  if (!channelFormRef.value) return

  try {
    await channelFormRef.value.validate()
  } catch {
    ElMessage.error('请检查表单填写是否正确')
    return
  }

  channelSubmitting.value = true
  try {
    const submitData = {
      provider_name: channelForm.value.provider_name.trim(),
      remark: channelForm.value.remark.trim(),
      api_endpoint: channelForm.value.api_endpoint.trim(),
      api_key: channelForm.value.api_key.trim(),
      icon: channelForm.value.icon.trim(),
      status: channelForm.value.status,
      thinking_signature_enabled: channelForm.value.thinking_signature_enabled, // 添加思考签名设置
      claude_code_simulation_enabled: channelForm.value.claude_code_simulation_enabled, // 添加ClaudeCode客户端模拟设置
      title_generation_model_mapping: channelForm.value.title_generation_model_mapping?.trim() || '', // 标题生成模型映射
      summary_model_mapping: channelForm.value.summary_model_mapping?.trim() || '', // 对话总结模型映射
      models: channelForm.value.models.filter(m => m.internal_model && m.external_model).map(m => ({
        internal_model: m.internal_model,
        external_model: m.external_model,
        reasoning_effort: isGPTModel(m.internal_model) ? (m.reasoning_effort || 'medium') : ''
      }))
    }

    if (channelDialogMode.value === 'create') {
      await externalChannelAPI.create(submitData)
      ElMessage.success('外部渠道创建成功，请测试任意模型以启用渠道')
    } else {
      await externalChannelAPI.update(currentEditChannelId.value, submitData)
      ElMessage.success('外部渠道更新成功')
    }

    channelDrawerVisible.value = false
    emit('fetchExternalChannels')
  } catch (error) {
    // API拦截器已显示错误
  } finally {
    channelSubmitting.value = false
  }
}

// 删除渠道
const handleDeleteChannel = async (channel) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除渠道 "${channel.provider_name}" 吗？删除后无法恢复。`,
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
    await externalChannelAPI.delete(channel.id)
    ElMessage.success('渠道删除成功')
    emit('fetchExternalChannels')
  } catch (error) {
    if (error !== 'cancel') {
      // API拦截器已显示错误
    }
  }
}

// 打开测试弹窗
const handleTestChannel = (channel) => {
  if (!channel.models || channel.models.length === 0) {
    ElMessage.warning('该渠道未配置模型映射，请先添加模型映射')
    return
  }
  currentTestChannel.value = channel
  selectedTestModel.value = channel.models[0].external_model // 默认选择第一个
  testDialogVisible.value = true
}

// 执行测试
const executeTest = async () => {
  if (!currentTestChannel.value || !selectedTestModel.value) return
  
  testExecuting.value = true
  testResult.value = null
  const startTime = Date.now()
  const wasDisabled = currentTestChannel.value.status === 'disabled'
  
  try {
    const result = await externalChannelAPI.test(currentTestChannel.value.id, selectedTestModel.value)
    const latency = ((Date.now() - startTime) / 1000).toFixed(2)
    
    // 如果测试成功且渠道之前是禁用状态，提示已自动启用
    let message = result.message
    if (result.success && wasDisabled) {
      message += '，渠道已自动启用'
    }
    
    // 测试成功后刷新列表以更新延迟值和状态
    if (result.success) {
      emit('fetchExternalChannels')
    }
    
    testResult.value = {
      success: result.success,
      message: message,
      latency: latency
    }
  } catch (error) {
    const latency = ((Date.now() - startTime) / 1000).toFixed(2)
    testResult.value = {
      success: false,
      message: error.message || '测试请求失败',
      latency: latency
    }
  } finally {
    testExecuting.value = false
  }
}

// 关闭测试弹窗
const closeTestDialog = () => {
  testDialogVisible.value = false
  currentTestChannel.value = null
  selectedTestModel.value = ''
  testResult.value = null
  testExecuting.value = false
}

// 处理表格图标加载错误
const handleTableIconError = (e) => {
  e.target.style.display = 'none'
}

// 初始化使用统计图表
const initUsageChart = () => {
  if (!usageChartRef.value) return
  
  const rect = usageChartRef.value.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) {
    setTimeout(initUsageChart, 100)
    return
  }
  
  if (usageChartInstance) {
    usageChartInstance.dispose()
    usageChartInstance = null
  }
  
  usageChartInstance = echarts.init(usageChartRef.value)
  
  const dates = usageStatsData.value.map(item => item.date)
  const requestCounts = usageStatsData.value.map(item => item.request_count)
  
  const option = {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'line' }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '12%',
      top: '10%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: dates,
      axisLabel: {
        rotate: 45,
        formatter: (value) => value.substring(5)
      }
    },
    yAxis: {
      type: 'value',
      minInterval: 1
    },
    series: [{
      name: '请求次数',
      type: 'line',
      data: requestCounts,
      smooth: true,
      itemStyle: { color: '#667eea' },
      lineStyle: { width: 2 },
      areaStyle: { color: 'rgba(102, 126, 234, 0.1)' }
    }]
  }
  
  usageChartInstance.setOption(option)
}

// 打开使用统计弹窗
const openUsageStatsDialog = (channel) => {
  currentStatsChannel.value = channel
  usageStatsDays.value = 7
  usageStatsData.value = []
  usageStatsDialogVisible.value = true
  fetchUsageStats()
}

// 获取使用统计数据
const fetchUsageStats = async () => {
  if (!currentStatsChannel.value) return
  
  usageStatsLoading.value = true
  try {
    const result = await externalChannelAPI.getUsageStats(currentStatsChannel.value.id, usageStatsDays.value)
    usageStatsData.value = result?.daily_stats || []
    nextTick(() => {
      if (usageStatsData.value.length > 0) {
        initUsageChart()
      }
    })
  } catch (error) {
    usageStatsData.value = []
  } finally {
    usageStatsLoading.value = false
  }
}

// 关闭使用统计弹窗
const closeUsageStatsDialog = () => {
  if (usageChartInstance) {
    usageChartInstance.dispose()
    usageChartInstance = null
  }
  usageStatsDialogVisible.value = false
  currentStatsChannel.value = null
  usageStatsData.value = []
}

// 打开状态切换弹窗
const openStatusDialog = (channel) => {
  currentStatusChannel.value = channel
  statusDialogVisible.value = true
}

// 关闭状态切换弹窗
const closeStatusDialog = () => {
  statusDialogVisible.value = false
  currentStatusChannel.value = null
  statusSubmitting.value = false
}

// 切换渠道状态
const handleStatusToggle = async () => {
  if (!currentStatusChannel.value) return
  
  statusSubmitting.value = true
  try {
    const newStatus = currentStatusChannel.value.status === 'active' ? 'disabled' : 'active'
    await externalChannelAPI.update(currentStatusChannel.value.id, { status: newStatus })
    ElMessage.success(`渠道已${newStatus === 'active' ? '启用' : '禁用'}`)
    closeStatusDialog()
    emit('fetchExternalChannels')
  } catch (error) {
    // API拦截器已显示错误
  } finally {
    statusSubmitting.value = false
  }
}

// 组件卸载时清理图表
onUnmounted(() => {
  if (usageChartInstance) {
    usageChartInstance.dispose()
    usageChartInstance = null
  }
})
</script>


<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.channels-panel .content-card {
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
}

.help-icon {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  cursor: help;
}

.help-icon:hover {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

/* ===== 抽屉表单 ===== */
.drawer-form {
  padding: 0 4px;
}
.form-section {
  margin-bottom: 10px;
  padding: 10px 12px;
  background: var(--secondary, oklch(0.967 0.001 286.375));
  border-radius: 10px;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
}
.section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  margin-bottom: 8px;
  padding-bottom: 6px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
  display: flex;
  align-items: center;
  gap: 6px;
}
.section-title::before {
  content: '';
  width: 3px;
  height: 13px;
  background: var(--primary, oklch(0.21 0.006 285.885));
  border-radius: 2px;
}
.form-section .el-form-item:last-child {
  margin-bottom: 0;
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

/* 眼睛图标按钮样式 */
.filter-buttons .visibility-btn {
  background: var(--card, oklch(1 0 0)) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32)) !important;
  border-radius: var(--radius-md, 8px) !important;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938)) !important;
  padding: 8px 12px !important;
  transition: all 0.2s ease !important;
}

.filter-buttons .visibility-btn:hover {
  background: var(--secondary, oklch(0.967 0.001 286.375)) !important;
  border-color: var(--ring, oklch(0.705 0.015 286.067)) !important;
  color: var(--foreground, oklch(0.141 0.005 285.823)) !important;
}

.filter-buttons .visibility-btn .el-icon {
  font-size: 16px;
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

/* 渠道图标单元格 */
.channel-icon-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 32px;
}

.channel-icon {
  width: 28px;
  height: 28px;
  object-fit: contain;
  border-radius: 4px;
}

.default-icon {
  font-size: 24px;
  color: #c0c4cc;
}

.api-endpoint-text {
  font-size: 13px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  word-break: break-all;
  transition: filter 0.2s ease;
}

/* API地址模糊效果 */
.api-address-blur {
  filter: blur(4px);
  user-select: none;
}

.model-mapping-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.model-tag {
  font-size: 12px;
}

.no-mapping {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 13px;
}

.latency-text {
  font-size: 13px;
  color: #22c55e;
  font-weight: 500;
}

.no-latency {
  color: var(--ring, oklch(0.705 0.015 286.067));
  font-size: 13px;
}

.remark-text {
  font-size: 13px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.no-remark {
  color: var(--ring, oklch(0.705 0.015 286.067));
}

.action-buttons {
  display: flex;
  gap: 8px;
}

.pagination-wrapper {
  margin-top: 16px;
  display: flex;
  justify-content: center;
}

/* 添加按钮样式 */
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

/* 模型映射区域 */
.model-mapping-section {
  padding: 0;
}

.model-mapping-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.model-select-internal {
  width: 160px;
  flex-shrink: 0;
}

.model-input-external {
  flex: 1;
  min-width: 0;
}

.reasoning-effort-select {
  width: 110px;
  flex-shrink: 0;
}

.mapping-arrow {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 16px;
}

.add-mapping-btn {
  margin-top: 8px;
}

/* API地址填充后缀图标 */
.fill-suffix-icon {
  cursor: pointer;
  color: #67c23a;
  font-size: 16px;
  transition: color 0.2s;
}

.fill-suffix-icon:hover {
  color: #85ce61;
}

.form-tip {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  margin-top: 4px;
}

/* 可折叠 section */
.form-section-collapsible .section-title-clickable {
  cursor: pointer;
  user-select: none;
  transition: color 0.2s ease;
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: none;
}
.form-section-collapsible .section-title-clickable::before {
  display: none;
}
.form-section-collapsible .section-title-clickable:hover {
  color: var(--primary, oklch(0.21 0.006 285.885));
}
.collapse-arrow {
  font-size: 13px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  transition: transform 0.3s ease;
}
.collapse-arrow.expanded {
  transform: rotate(90deg);
}
.section-body {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

/* 折叠动画 */
.collapse-enter-active,
.collapse-leave-active {
  transition: all 0.3s ease;
  overflow: hidden;
}
.collapse-enter-from,
.collapse-leave-to {
  opacity: 0;
  max-height: 0;
  margin-top: 0;
  padding-top: 0;
}
.collapse-enter-to,
.collapse-leave-from {
  opacity: 1;
  max-height: 300px;
}

/* 思考签名开关区域 */
.thinking-signature-switch {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.api-warning-tip {
  font-size: 12px;
  color: #ef4444;
  margin-top: 4px;
}

/* 获取可用模型区域 */
.fetch-models-section {
  margin-top: 8px;
}

.available-models-list {
  margin-top: 8px;
  padding: 12px;
  background: var(--card, oklch(1 0 0));
  border-radius: var(--radius-md, 8px);
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.models-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}

.model-tag {
  cursor: pointer;
  transition: all 0.2s ease;
}

.model-tag:hover {
  background: var(--color-success, #10b981) !important;
  color: #fff !important;
  border-color: var(--color-success, #10b981) !important;
}

/* 对话框底部按钮区域 */
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
  font-size: 14px !important;
  transition: all 0.2s ease !important;
}

.cancel-btn {
  background: var(--card, oklch(1 0 0)) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32)) !important;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938)) !important;
}

.cancel-btn:hover {
  border-color: var(--ring, oklch(0.705 0.015 286.067)) !important;
  color: var(--foreground, oklch(0.141 0.005 285.823)) !important;
  background: var(--secondary, oklch(0.967 0.001 286.375)) !important;
}

.submit-btn {
  background: var(--primary, oklch(0.21 0.006 285.885)) !important;
  border: none !important;
  color: #fff !important;
}

.submit-btn:hover:not(:disabled) {
  background: oklch(0.3 0.006 285.885) !important;
}

/* 测试弹窗内容样式 */
.test-dialog-content {
  padding: 10px 0;
}

.test-tip {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  margin-bottom: 12px;
}

/* 测试结果区域样式 */
.test-result-section {
  margin-top: 20px;
}

.test-result-card {
  border-radius: var(--radius-md, 8px);
  padding: 16px;
  border: 1px solid;
}

.test-result-card.success {
  background: #f0fdf4;
  border-color: #86efac;
}

.test-result-card.error {
  background: #fef2f2;
  border-color: #fca5a5;
}

.result-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.result-icon {
  font-size: 20px;
}

.result-icon.success {
  color: #22c55e;
}

.result-icon.error {
  color: #ef4444;
}

.result-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.result-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.result-item {
  display: flex;
  align-items: flex-start;
  font-size: 13px;
}

.result-label {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  min-width: 80px;
}

.result-value {
  color: var(--foreground, oklch(0.141 0.005 285.823));
  word-break: break-all;
  margin-bottom: 12px;
}

/* 使用统计按钮样式 */
.usage-stats-btn {
  padding: 4px !important;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938)) !important;
}

.usage-stats-btn:hover {
  color: var(--primary, oklch(0.21 0.006 285.885)) !important;
}

.usage-stats-btn .el-icon {
  font-size: 18px;
}

/* 使用统计弹窗内容样式 */
.usage-stats-content {
  padding: 0;
}

.time-range-selector {
  display: flex;
  justify-content: center;
  margin-bottom: 16px;
}

.chart-container {
  height: 280px;
  width: 100%;
}

.empty-data {
  height: 280px;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 状态标签可点击样式 */
.status-tag-clickable {
  cursor: pointer;
  transition: all 0.2s ease;
}

.status-tag-clickable:hover {
  transform: scale(1.05);
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.15);
}

/* 状态标签禁用样式（已绑定的渠道） */
.status-tag-disabled {
  cursor: not-allowed;
  opacity: 0.8;
}

/* 状态弹窗内容样式 */
.status-dialog-content {
  padding: 10px 0;
}

.status-tip {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  margin-bottom: 16px;
  line-height: 1.8;
}

.status-tip .channel-name {
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.status-question {
  font-size: 14px;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  line-height: 1.8;
}
</style>

<style>
/* 外部渠道抽屉样式 */
.channel-drawer.el-drawer {
  border-left: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.channel-drawer .el-drawer__header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
  margin-bottom: 0;
}

.channel-drawer .el-drawer__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.channel-drawer .el-drawer__body {
  padding: 14px 16px 10px 16px;
  overflow-y: auto;
}

.channel-drawer .el-drawer__footer {
  padding: 12px 20px;
  border-top: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.channel-drawer .el-form-item__label {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-weight: 500;
}

.channel-drawer .el-form-item {
  margin-bottom: 10px;
}

.channel-drawer .el-form-item__label {
  padding-bottom: 4px;
}

.channel-drawer .el-input__wrapper,
.channel-drawer .el-textarea__inner {
  border-radius: var(--radius-md, 8px) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32)) !important;
  box-shadow: none !important;
}

.channel-drawer .el-input__wrapper:hover,
.channel-drawer .el-textarea__inner:hover {
  border-color: var(--ring, oklch(0.705 0.015 286.067)) !important;
}

.channel-drawer .el-input__wrapper.is-focus,
.channel-drawer .el-textarea__inner:focus {
  border-color: var(--primary, oklch(0.21 0.006 285.885)) !important;
}

/* 测试弹窗样式 */
.test-dialog.el-dialog {
  border-radius: var(--radius-lg, 10px) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
}

.test-dialog .el-dialog__header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.test-dialog .el-dialog__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.test-dialog .el-dialog__body {
  padding: 20px;
}

.test-dialog .el-dialog__footer {
  padding: 12px 20px;
}

/* 使用统计弹窗样式 */
.usage-stats-dialog.el-dialog {
  border-radius: var(--radius-lg, 10px) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
}

.usage-stats-dialog .el-dialog__header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.usage-stats-dialog .el-dialog__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.usage-stats-dialog .el-dialog__body {
  padding: 20px;
}

.usage-stats-dialog .el-dialog__footer {
  padding: 12px 20px;
}

/* 状态切换弹窗样式 */
.status-dialog.el-dialog {
  border-radius: var(--radius-lg, 10px) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
}

.status-dialog .el-dialog__header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.status-dialog .el-dialog__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.status-dialog .el-dialog__body {
  padding: 20px;
}

.status-dialog .el-dialog__footer {
  padding: 12px 20px;
}
</style>
