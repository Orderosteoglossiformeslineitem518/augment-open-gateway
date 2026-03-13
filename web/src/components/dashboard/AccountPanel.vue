<template>
  <div class="panel accounts-panel">
    <div class="content-card">
      <div class="card-header">
        <h3>TOKEN账号列表</h3>
        <div class="header-notice">
          <el-icon class="notice-icon"><WarningFilled /></el-icon>
          <span class="notice-text">由于 AugmentCode 官方收费政策变动，将不再为新用户提供账号，已分配账号用户待积分用尽后请自行新增账号或使用其他服务</span>
        </div>
        <div class="header-actions">
          <el-button class="add-token-btn" @click="openAddTokenDialog">
            <el-icon><Plus /></el-icon>
            添加账号
          </el-button>
        </div>
      </div>
      <!-- 搜索和筛选区域 -->
      <div class="filter-section">
        <el-row :gutter="12" align="middle">
          <el-col :span="6">
            <el-input
              v-model="searchQueryLocal"
              placeholder="搜索邮箱或TOKEN"
              clearable
              @keyup.enter="handleQuery"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
          </el-col>
          <el-col :span="2">
            <el-select v-model="tokenTypeFilterLocal" placeholder="类型筛选" clearable @change="handleQuery" style="width: 100%;">
              <el-option label="全部类型" value="" />
              <el-option label="共享" value="shared" />
              <el-option label="自有" value="own" />
            </el-select>
          </el-col>
          <el-col :span="2">
            <el-select v-model="statusFilterLocal" placeholder="状态筛选" clearable @change="handleQuery" style="width: 100%;">
              <el-option label="全部状态" value="" />
              <el-option label="正常" value="active" />
              <el-option label="已封禁" value="disabled" />
              <el-option label="已过期" value="expired" />
            </el-select>
          </el-col>
          <el-col :span="14">
            <div class="filter-buttons">
              <el-button class="reset-btn" @click="handleQueryReset">重置</el-button>
              <el-button class="query-btn" @click="handleQuery">查询</el-button>
            </div>
          </el-col>
        </el-row>
      </div>
      <div class="card-body">
        <div class="table-wrapper">
        <el-table :data="tokenAllocations" v-loading="tokenListLoading" style="width: 100%" height="100%" empty-text="暂无可用账号" :row-class-name="getRowClassName">
          <!-- TOKEN -->
          <el-table-column label="TOKEN" min-width="100">
            <template #default="{ row }">
              <div class="token-cell">
                <el-tooltip v-if="row.is_current_using" content="正在使用" placement="top" :show-after="300">
                  <span :class="['token-id', 'current-using']">
                    {{ row.is_shared_token ? row.token_value : formatTokenId(row.token_value) }}
                  </span>
                </el-tooltip>
                <span v-else class="token-id">
                  {{ row.is_shared_token ? row.token_value : formatTokenId(row.token_value) }}
                </span>
                <el-button v-if="!row.is_shared_token" link size="small" @click="copyText(row.token_value, 'TOKEN')">
                  <el-icon><DocumentCopy /></el-icon>
                </el-button>
              </div>
            </template>
          </el-table-column>
          <!-- 租户地址 -->
          <el-table-column label="租户地址" min-width="120">
            <template #default="{ row }">
              <div class="tenant-cell" v-if="row.tenant_address">
                <el-tooltip :content="row.tenant_address" placement="top" :show-after="300">
                  <span class="tenant-address">{{ formatTenantAddress(row.tenant_address) }}</span>
                </el-tooltip>
                <el-button link size="small" @click="copyText(row.tenant_address, '租户地址')">
                  <el-icon><DocumentCopy /></el-icon>
                </el-button>
              </div>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <!-- 邮箱 -->
          <el-table-column prop="token_email" label="邮箱" min-width="140">
            <template #default="{ row }">
              <div class="email-cell" v-if="row.token_email">
                <el-tooltip :content="row.token_email" placement="top" :show-after="300">
                  <span class="email-text">{{ formatEmail(row.token_email) }}</span>
                </el-tooltip>
                <el-button link size="small" @click="copyText(row.token_email, '邮箱')">
                  <el-icon><DocumentCopy /></el-icon>
                </el-button>
              </div>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <!-- 订阅地址 -->
          <el-table-column label="订阅" width="60" align="center">
            <template #default="{ row }">
              <el-tooltip 
                :content="row.portal_url ? '点击查看订阅详情' : '该账号未配置Portal地址'" 
                placement="top" 
                :show-after="300"
              >
                <el-button 
                  link 
                  size="small" 
                  :disabled="!row.portal_url"
                  :class="['portal-btn', { 'portal-btn-disabled': !row.portal_url }]"
                  @click="openPortalUrl(row.portal_url)"
                >
                  <el-icon :size="16"><Link /></el-icon>
                </el-button>
              </el-tooltip>
            </template>
          </el-table-column>
          <!-- 账号类型 -->
          <el-table-column label="类型" width="80">
            <template #default="{ row }">
              <el-tag :type="row.is_shared_token ? 'warning' : 'primary'" size="small">
                {{ row.is_shared_token ? '共享' : '自有' }}
              </el-tag>
            </template>
          </el-table-column>
          <!-- 状态 -->
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <el-tag :type="getTokenStatusType(row)" size="small">
                {{ getTokenStatusText(row) }}
              </el-tag>
            </template>
          </el-table-column>
          <!-- 封禁原因 -->
          <el-table-column label="封禁原因" width="100">
            <template #default="{ row }">
              <el-button 
                v-if="row.ban_reason" 
                link 
                type="danger" 
                size="small"
                @click="showBanReasonDialog(row)"
              >
                查看
              </el-button>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <!-- 使用情况 -->
          <el-table-column width="180">
            <template #header>
              <span class="column-header-with-help">
                使用情况
                <el-tooltip content="此处展示为官方积分用量，非实时统计" placement="top">
                  <el-icon class="help-icon"><QuestionFilled /></el-icon>
                </el-tooltip>
              </span>
            </template>
            <template #default="{ row }">
              <div class="usage-cell">
                <el-progress 
                  :percentage="getUsagePercentage(row)" 
                  :status="getUsageStatus(row)"
                  :stroke-width="8"
                  :show-text="false"
                  class="usage-progress"
                />
                <span class="usage-text">{{ row.used_requests }} / {{ row.max_requests === -1 ? '∞' : row.max_requests }}</span>
              </div>
            </template>
          </el-table-column>
          <!-- 增强状态 -->
          <el-table-column label="增强状态" width="100">
            <template #default="{ row }">
              <el-tooltip 
                v-if="row.enhanced_enabled && row.enhanced_channel_name" 
                :content="`绑定渠道: ${row.enhanced_channel_name}`" 
                placement="top"
              >
                <el-tag type="success" size="small" class="enhanced-tag">已增强</el-tag>
              </el-tooltip>
              <el-tag v-else :type="row.enhanced_enabled ? 'success' : 'info'" size="small">
                {{ row.enhanced_enabled ? '已增强' : '未增强' }}
              </el-tag>
            </template>
          </el-table-column>
          <!-- 创建时间 -->
          <el-table-column prop="token_created_at" label="创建时间" width="130">
            <template #default="{ row }">
              {{ formatDateTime(row.token_created_at) }}
            </template>
          </el-table-column>
          <!-- 过期时间 -->
          <el-table-column label="过期时间" width="130">
            <template #default="{ row }">
              {{ row.token_expires_at ? formatDateTime(row.token_expires_at) : '永久' }}
            </template>
          </el-table-column>
          <!-- 操作 -->
          <el-table-column label="操作" width="180" fixed="right">
            <template #default="{ row }">
              <div class="action-buttons">
                <el-button
                  link
                  type="warning"
                  size="small"
                  :disabled="row.token_status === 'disabled'"
                  @click="handleEnhanceToken(row)"
                >
                  增强
                </el-button>
                <el-button
                  link
                  type="primary"
                  size="small"
                  :disabled="row.is_current_using || row.token_status !== 'active' || tokenAllocations.length <= 1"
                  @click="handleSwitchToken(row)"
                >
                  切换
                </el-button>
                <el-button
                  link
                  type="danger"
                  size="small"
                  :disabled="row.token_status === 'disabled'"
                  @click="handleDisableToken(row)"
                >
                  禁用
                </el-button>
                <el-button
                  v-if="!row.is_shared_token && row.token_status === 'disabled'"
                  link
                  type="danger"
                  size="small"
                  @click="handleDeleteToken(row)"
                >
                  删除
                </el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        </div>
        <div class="pagination-wrapper">
          <el-pagination
            v-model:current-page="pageLocal"
            v-model:page-size="pageSizeLocal"
            :page-sizes="[10, 20, 50, 100]"
            :total="total"
            layout="total, sizes, prev, pager, next"
            @current-change="handlePageChange"
            @size-change="handlePageSizeChange"
          />
        </div>
      </div>
    </div>

    <!-- 封禁原因弹窗 -->
    <el-dialog v-model="showBanReasonDialogVisible" title="封禁原因" width="400px" class="ban-reason-dialog">
      <div class="ban-reason-content">
        <p>{{ currentBanReason }}</p>
      </div>
      <template #footer>
        <el-button @click="showBanReasonDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 增强TOKEN对话框 -->
    <el-dialog
      v-model="enhanceDialogVisible"
      title="增强TOKEN"
      width="480px"
      class="enhance-dialog"
      :close-on-click-modal="false"
      @close="resetEnhanceForm"
    >
      <div class="enhance-dialog-content" v-loading="channelListLoading" element-loading-text="加载渠道列表...">
        <p class="enhance-tip">选择一个外部渠道来增强此TOKEN的能力：</p>
        <p v-if="currentEnhanceToken && !currentEnhanceToken.is_current_using" class="enhance-warning">
          注意，当前增强TOKEN非正在使用账号，渠道无法即时生效！
        </p>
        <el-form ref="enhanceFormRef" :model="enhanceForm" label-width="80px">
          <el-form-item label="外部渠道" prop="channel_id">
            <el-select
              v-model="enhanceForm.channel_id"
              placeholder="请选择外部渠道"
              style="width: 100%;"
              :disabled="channelListLoading"
              clearable
            >
              <el-option
                v-for="channel in availableChannels"
                :key="channel.id"
                :label="channel.provider_name"
                :value="channel.id"
              >
                <span>{{ channel.provider_name }}</span>
                <span v-if="channel.remark" class="channel-remark">（{{ channel.remark }}）</span>
              </el-option>
            </el-select>
          </el-form-item>
        </el-form>
        <div v-if="currentEnhanceToken?.enhanced_channel_id" class="current-binding-info">
          <el-alert type="info" :closable="false">
            当前已绑定渠道：<strong>{{ currentEnhanceToken.enhanced_channel_name }}</strong>
          </el-alert>
        </div>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button class="dialog-btn cancel-btn" @click="enhanceDialogVisible = false">取消</el-button>
          <el-button
            class="dialog-btn submit-btn"
            :loading="enhanceSubmitting"
            :disabled="channelListLoading"
            @click="handleEnhanceSubmit"
          >
            {{ getEnhanceButtonText }}
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 添加TOKEN账号抽屉 -->
    <el-drawer
      v-model="addTokenDrawerVisible"
      title="添加TOKEN账号"
      direction="rtl"
      size="560px"
      class="add-token-drawer"
      :close-on-click-modal="true"
      @close="resetAddTokenForm"
    >
      <el-form ref="addTokenFormRef" :model="addTokenForm" :rules="addTokenRules" label-position="top" class="drawer-form">
        <!-- 认证信息 -->
        <div class="form-section">
          <div class="section-title">认证信息</div>
          <el-alert type="info" :closable="false" class="section-alert">
            <template #default>
              <span style="font-size: 13px;">
                提交方式：<strong>AuthSession</strong> 或 <strong>TOKEN + 租户地址</strong>（二选一）
              </span>
            </template>
          </el-alert>
          <el-form-item label="AuthSession" prop="auth_session">
            <el-input
              v-model="addTokenForm.auth_session"
              type="textarea"
              :rows="3"
              placeholder="输入AuthSession，系统将自动获取TOKEN、租户地址，必须以.eJ或.ey开头"
            />
          </el-form-item>
          <el-form-item label="TOKEN" prop="token">
            <el-input
              v-model="addTokenForm.token"
              placeholder="输入Augment账号TOKEN（64位）"
              maxlength="64"
              show-word-limit
            />
          </el-form-item>
          <el-form-item label="租户地址" prop="tenant_address">
            <el-input v-model="addTokenForm.tenant_address" placeholder="输入租户地址，必须以https://开头" />
          </el-form-item>
        </div>

        <!-- 账号配置 -->
        <div class="form-section">
          <div class="section-title">账号配置</div>
          <el-form-item label="PortalUrl" prop="portal_url">
            <el-input v-model="addTokenForm.portal_url" placeholder="订阅地址（可选），使用AuthSession时会自动获取" />
          </el-form-item>
          <el-form-item label="代理地址" prop="proxy_address">
            <el-input v-model="addTokenForm.proxy_address" placeholder="选填：输入一个可用的deno或supabase代理地址" />
            <div class="proxy-help-tip">
              <a href="" target="_blank" class="proxy-help-link">
                <el-icon><QuestionFilled /></el-icon>
                <span>查看代理搭建教程</span>
              </a>
            </div>
          </el-form-item>
          <el-form-item label="积分额度" prop="account_type" required>
            <el-radio-group v-model="addTokenForm.account_type">
              <el-radio label="34000_credits">34000</el-radio>
              <el-radio label="30000_credits">30000</el-radio>
              <el-radio label="24000_credits">24000</el-radio>
              <el-radio label="4000_credits">4000</el-radio>
              <el-radio label="0_credits">0</el-radio>
            </el-radio-group>
          </el-form-item>
        </div>

        <!-- 人机验证 -->
        <div class="form-section">
          <div class="section-title">人机验证</div>
          <div class="turnstile-section">
            <div v-if="turnstileLoading" class="turnstile-loading">
              <el-icon class="loading-icon"><Loading /></el-icon>
              <span>人机验证加载中...</span>
            </div>
            <div v-show="!turnstileLoading" id="add-token-turnstile-widget" class="turnstile-widget"></div>
          </div>
        </div>
      </el-form>

      <template #footer>
        <div class="dialog-footer">
          <el-button class="dialog-btn cancel-btn" @click="addTokenDrawerVisible = false">取消</el-button>
          <el-button
            class="dialog-btn submit-btn"
            :loading="addTokenSubmitting"
            :disabled="!turnstileToken"
            @click="handleAddTokenSubmit"
          >
            {{ turnstileToken ? '添加' : '请先完成人机验证' }}
          </el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Search, DocumentCopy, Link, QuestionFilled, Loading, WarningFilled } from '@element-plus/icons-vue'
import { userTokenAPI, externalChannelAPI } from '@/api'

const props = defineProps({
  tokenAllocations: { type: Array, default: () => [] },
  tokenListLoading: { type: Boolean, default: false },
  page: { type: Number, default: 1 },
  pageSize: { type: Number, default: 10 },
  total: { type: Number, default: 0 },
  searchQuery: { type: String, default: '' },
  statusFilter: { type: String, default: '' },
  tokenTypeFilter: { type: String, default: 'shared' }
})

const emit = defineEmits([
  'update:page', 'update:pageSize', 'update:searchQuery', 'update:statusFilter', 'update:tokenTypeFilter',
  'fetchTokenAllocations', 'fetchTokenAccountStats'
])

// 本地状态
const pageLocal = ref(props.page)
const pageSizeLocal = ref(props.pageSize)
const searchQueryLocal = ref(props.searchQuery)
const statusFilterLocal = ref(props.statusFilter)
const tokenTypeFilterLocal = ref(props.tokenTypeFilter)

// 封禁原因弹窗
const showBanReasonDialogVisible = ref(false)
const currentBanReason = ref('')

// 添加TOKEN抽屉
const addTokenDrawerVisible = ref(false)
const addTokenSubmitting = ref(false)
const addTokenFormRef = ref(null)
const turnstileToken = ref('')
const turnstileWidgetId = ref(null)
const turnstileLoading = ref(false)
const addTokenForm = ref({
  auth_session: '',
  token: '',
  tenant_address: '',
  portal_url: '',
  proxy_address: '',
  account_type: '30000_credits'
})

// 增强TOKEN对话框
const enhanceDialogVisible = ref(false)
const enhanceSubmitting = ref(false)
const enhanceFormRef = ref(null)
const currentEnhanceToken = ref(null)
const availableChannels = ref([])
const channelListLoading = ref(false)
const enhanceForm = ref({
  channel_id: null
})

// 监听 props 变化
watch(() => props.page, (val) => { pageLocal.value = val })
watch(() => props.pageSize, (val) => { pageSizeLocal.value = val })
watch(() => props.searchQuery, (val) => { searchQueryLocal.value = val })
watch(() => props.statusFilter, (val) => { statusFilterLocal.value = val })
watch(() => props.tokenTypeFilter, (val) => { tokenTypeFilterLocal.value = val })

// 添加TOKEN表单验证规则
const addTokenRules = {
  auth_session: [{
    validator: (_, value, callback) => {
      if (!value) { callback(); return }
      const trimmedValue = value.trim()
      if (!trimmedValue.startsWith('.eJ') && !trimmedValue.startsWith('.ey')) {
        callback(new Error('AuthSession必须以.eJ或.ey开头'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }],
  token: [{
    validator: (_, value, callback) => {
      if (value && value.length !== 64) {
        callback(new Error('TOKEN必须为64位字符串'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }],
  tenant_address: [{
    validator: (_, value, callback) => {
      if (!value) { callback(); return }
      let normalizedValue = value.trim()
      if (!normalizedValue.endsWith('/')) {
        normalizedValue = normalizedValue + '/'
        addTokenForm.value.tenant_address = normalizedValue
      }
      if (!normalizedValue.startsWith('https://')) {
        callback(new Error('租户地址必须以https://开头'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }],
  proxy_address: [
    {
      validator: (_, value, callback) => {
        if (!value || !value.trim()) {
          callback()
          return
        }
        let normalizedValue = value.trim()
        if (!normalizedValue.endsWith('/')) {
          normalizedValue = normalizedValue + '/'
          addTokenForm.value.proxy_address = normalizedValue
        }
        if (!normalizedValue.startsWith('https://')) {
          callback(new Error('代理地址必须以https://开头'))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ]
}

// 格式化方法
const formatTokenId = (tokenId) => {
  if (!tokenId || tokenId.length < 12) return tokenId
  return `${tokenId.substring(0, 6)}...${tokenId.substring(tokenId.length - 4)}`
}

const formatTenantAddress = (address) => {
  if (!address || address.length < 30) return address
  return `${address.substring(0, 20)}...${address.substring(address.length - 8)}`
}

const formatEmail = (email) => {
  if (!email || email.length < 8) return email
  const prefix = email.substring(0, 4)
  const suffix = email.substring(email.length - 4)
  return `${prefix}********${suffix}`
}

const formatDateTime = (dateStr) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit'
  })
}

const copyText = async (text, label) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`${label}已复制到剪贴板`)
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

// 打开Portal URL查看订阅详情
const openPortalUrl = (portalUrl) => {
  if (portalUrl) {
    window.open(portalUrl, '_blank')
  }
}

// 状态相关方法
const getTokenStatusType = (row) => {
  if (row.token_status === 'active') return 'success'
  if (row.token_status === 'disabled') return 'danger'
  if (row.token_status === 'expired') return 'warning'
  return 'info'
}

const getTokenStatusText = (row) => {
  if (row.token_status === 'active') return '正常'
  if (row.token_status === 'disabled') return '已禁用'
  if (row.token_status === 'expired') return '已过期'
  return row.status_display || '未知'
}

const getUsagePercentage = (row) => {
  if (row.max_requests === -1 || row.max_requests === 0) return 0
  return Math.min(100, Math.round((row.used_requests / row.max_requests) * 100))
}

const getUsageStatus = (row) => {
  const percentage = getUsagePercentage(row)
  if (percentage >= 90) return 'exception'
  if (percentage >= 70) return 'warning'
  return ''
}

const getRowClassName = ({ row }) => {
  if (row.is_current_using) return 'current-using-row'
  if (row.token_status === 'disabled') return 'disabled-row'
  return ''
}

// 封禁原因弹窗
const showBanReasonDialog = (row) => {
  currentBanReason.value = row.ban_reason || '未知原因'
  showBanReasonDialogVisible.value = true
}

// 查询相关
const handleQuery = () => {
  emit('update:searchQuery', searchQueryLocal.value)
  emit('update:statusFilter', statusFilterLocal.value)
  emit('update:tokenTypeFilter', tokenTypeFilterLocal.value)
  emit('update:page', 1)
  emit('fetchTokenAllocations')
}

const handleQueryReset = () => {
  searchQueryLocal.value = ''
  statusFilterLocal.value = ''
  tokenTypeFilterLocal.value = ''
  emit('update:searchQuery', '')
  emit('update:statusFilter', '')
  emit('update:tokenTypeFilter', '')
  emit('update:page', 1)
  emit('fetchTokenAllocations')
}

const handlePageChange = (page) => {
  emit('update:page', page)
  emit('fetchTokenAllocations')
}

const handlePageSizeChange = (size) => {
  emit('update:pageSize', size)
  emit('update:page', 1)
  emit('fetchTokenAllocations')
}

// TOKEN操作
const handleSwitchToken = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要切换到账号 ${row.token_email || row.token_id} 吗？`,
      '确认切换',
      {
        confirmButtonText: '确定切换',
        cancelButtonText: '取消',
        type: 'info',
        customClass: 'token-action-dialog',
        confirmButtonClass: 'token-action-confirm-btn primary',
        cancelButtonClass: 'token-action-cancel-btn'
      }
    )
    await userTokenAPI.switchToken(row.token_id)
    ElMessage.success('账号切换成功')
    emit('fetchTokenAllocations')
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error(error.message || '切换失败')
    }
  }
}

const handleDisableToken = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要禁用账号 ${row.token_email || row.token_id} 吗？禁用后将无法使用该账号。`,
      '确认禁用',
      {
        confirmButtonText: '确定禁用',
        cancelButtonText: '取消',
        type: 'warning',
        customClass: 'token-action-dialog',
        confirmButtonClass: 'token-action-confirm-btn danger',
        cancelButtonClass: 'token-action-cancel-btn'
      }
    )
    await userTokenAPI.disableToken(row.token_id)
    ElMessage.success('账号已禁用')
    emit('fetchTokenAllocations')
    emit('fetchTokenAccountStats')
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error(error.message || '禁用失败')
    }
  }
}

const handleDeleteToken = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除账号 ${row.token_email || row.token_id} 吗？删除后该账号将从您的账号列表中移除，此操作不可恢复。`,
      '确认删除',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'error',
        customClass: 'token-action-dialog',
        confirmButtonClass: 'token-action-confirm-btn danger',
        cancelButtonClass: 'token-action-cancel-btn'
      }
    )
    await userTokenAPI.deleteToken(row.token_id)
    ElMessage.success('账号已删除')
    emit('fetchTokenAllocations')
    emit('fetchTokenAccountStats')
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error(error.message || '删除失败')
    }
  }
}

// 增强TOKEN相关
const handleEnhanceToken = async (row) => {
  currentEnhanceToken.value = row
  enhanceForm.value.channel_id = row.enhanced_channel_id || null
  enhanceDialogVisible.value = true
  await fetchAvailableChannels()
}

const fetchAvailableChannels = async () => {
  channelListLoading.value = true
  try {
    const res = await externalChannelAPI.getList()
    // 只显示已启用的渠道（status为active）
    const allChannels = res.data?.list || res.list || []
    availableChannels.value = allChannels.filter(channel => channel.status === 'active')
  } catch (error) {
    if (!error.handled && !error.silent) {
      ElMessage.error('获取外部渠道列表失败')
    }
    availableChannels.value = []
  } finally {
    channelListLoading.value = false
  }
}

const resetEnhanceForm = () => {
  enhanceForm.value = { channel_id: null }
  currentEnhanceToken.value = null
  if (enhanceFormRef.value) {
    enhanceFormRef.value.clearValidate()
  }
}

// 计算按钮文本
const getEnhanceButtonText = computed(() => {
  const hasCurrentBinding = currentEnhanceToken.value?.enhanced_channel_id
  const hasNewSelection = enhanceForm.value.channel_id

  if (!hasCurrentBinding && hasNewSelection) {
    return '绑定'
  } else if (hasCurrentBinding && !hasNewSelection) {
    return '解除绑定'
  } else if (hasCurrentBinding && hasNewSelection) {
    return '更新绑定'
  }
  return '确定'
})

const handleEnhanceSubmit = async () => {
  const hasCurrentBinding = currentEnhanceToken.value?.enhanced_channel_id
  const hasNewSelection = enhanceForm.value.channel_id

  // 无变化时提示
  if (!hasCurrentBinding && !hasNewSelection) {
    ElMessage.warning('请选择外部渠道')
    return
  }

  enhanceSubmitting.value = true
  try {
    if (hasNewSelection) {
      // 绑定或更新
      await userTokenAPI.enhanceToken(currentEnhanceToken.value.token_id, enhanceForm.value.channel_id)
      ElMessage.success(hasCurrentBinding ? '增强绑定已更新' : '增强绑定成功')
    } else {
      // 解除绑定（清空了选择）
      await userTokenAPI.removeTokenEnhance(currentEnhanceToken.value.token_id)
      ElMessage.success('已解除增强绑定')
    }
    enhanceDialogVisible.value = false
    emit('fetchTokenAllocations')
  } catch (error) {
    // 如果错误已在拦截器中处理，不重复显示
    if (!error.handled && !error.silent) {
      ElMessage.error(error.message || '操作失败')
    }
  } finally {
    enhanceSubmitting.value = false
  }
}

// 添加TOKEN相关
const openAddTokenDialog = () => {
  addTokenDrawerVisible.value = true
  nextTick(() => {
    setTimeout(() => initTurnstile(), 100)
  })
}

const resetAddTokenForm = () => {
  addTokenForm.value = {
    auth_session: '',
    token: '',
    tenant_address: '',
    portal_url: '',
    proxy_address: '',
    account_type: '30000_credits'
  }
  turnstileToken.value = ''
  if (addTokenFormRef.value) {
    addTokenFormRef.value.clearValidate()
  }
  if (window.turnstile && turnstileWidgetId.value) {
    window.turnstile.remove(turnstileWidgetId.value)
    turnstileWidgetId.value = null
  }
}

const initTurnstile = () => {
  turnstileLoading.value = true
  if (!window.turnstile) {
    const script = document.createElement('script')
    script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js'
    script.async = true
    script.defer = true
    script.onload = () => setTimeout(() => renderTurnstile(), 100)
    document.head.appendChild(script)
  } else {
    renderTurnstile()
  }
}

const renderTurnstile = () => {
  const container = document.getElementById('add-token-turnstile-widget')
  if (!window.turnstile || !container) {
    turnstileLoading.value = false
    return
  }

  // 先清理旧 widget
  if (turnstileWidgetId.value) {
    try { window.turnstile.remove(turnstileWidgetId.value) } catch (e) {}
    turnstileWidgetId.value = null
  }
  container.innerHTML = ''

  const isLocalhost = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
  const siteKey = isLocalhost ? '1x00000000000000000000AA' : '0x4AAAAAACH-xo8bXfQMEWZ1'

  try {
    turnstileWidgetId.value = window.turnstile.render('#add-token-turnstile-widget', {
      sitekey: siteKey,
      theme: 'light',
      size: 'normal',
      callback: (token) => { turnstileToken.value = token },
      'error-callback': () => { turnstileToken.value = '' },
      'expired-callback': () => { turnstileToken.value = '' },
      'timeout-callback': () => { turnstileToken.value = '' }
    })
    // 渲染启动后立即结束 loading，让用户能看到验证框
    turnstileLoading.value = false
  } catch (e) {
    turnstileLoading.value = false
    console.error('Turnstile 渲染失败:', e)
  }
}

const handleAddTokenSubmit = async () => {
  if (!addTokenFormRef.value) return

  const hasAuthSession = addTokenForm.value.auth_session.trim().length > 0
  const hasToken = addTokenForm.value.token.trim().length > 0
  const hasTenantAddress = addTokenForm.value.tenant_address.trim().length > 0

  if (!hasAuthSession && (!hasToken || !hasTenantAddress)) {
    ElMessage.error('请填写AuthSession或(TOKEN+租户地址)')
    return
  }

  try {
    await addTokenFormRef.value.validate()
  } catch {
    ElMessage.error('请检查表单填写是否正确')
    return
  }

  if (!turnstileToken.value) {
    ElMessage.error('请完成人机验证')
    return
  }

  addTokenSubmitting.value = true
  try {
    const submitData = {
      auth_session: addTokenForm.value.auth_session.trim(),
      token: addTokenForm.value.token.trim(),
      tenant_address: addTokenForm.value.tenant_address.trim(),
      portal_url: addTokenForm.value.portal_url.trim(),
      proxy_address: addTokenForm.value.proxy_address.trim(),
      account_type: addTokenForm.value.account_type,
      turnstile_token: turnstileToken.value
    }
    await userTokenAPI.submitToken(submitData)
    ElMessage.success('TOKEN账号添加成功')
    addTokenDrawerVisible.value = false
    emit('fetchTokenAllocations')
    emit('fetchTokenAccountStats')
  } catch (error) {
    // 如果错误已在拦截器中处理，不重复显示
    if (!error.handled && !error.silent) {
      ElMessage.error(error.message || '添加失败，请重试')
    }
  } finally {
    addTokenSubmitting.value = false
  }
}
</script>


<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.content-card {
  background: var(--card, oklch(1 0 0));
  border-radius: var(--radius-xl, 14px);
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: calc(100vh - 140px);
  min-height: 400px;
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
  border-color: #cbd5e1 !important;
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

/* 行内公告 */
.header-notice {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 4px 12px;
  max-width: 70%;
  background: var(--secondary, oklch(0.967 0.001 286.375));
  border-radius: var(--radius-md, 8px);
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
}

.notice-icon {
  font-size: 14px;
  color: #f59e0b;
  flex-shrink: 0;
  animation: noticePulse 2s ease-in-out infinite;
}

@keyframes noticePulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.1); }
}

.notice-text {
  font-size: 12px;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  font-weight: 500;
  line-height: 1.5;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* TOKEN账号列表样式 */
.token-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.token-id {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.token-id.current-using {
  color: var(--color-info, #3b82f6);
  font-weight: 600;
}

.current-tag {
  margin-left: 4px;
}

.tenant-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tenant-address {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.usage-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.usage-progress {
  width: 100px !important;
}

.usage-text {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.email-cell {
  display: flex;
  align-items: center;
  gap: 4px;
}

.email-text {
  font-size: 12px;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 订阅地址按钮样式 */
.portal-btn {
  color: var(--primary, oklch(0.21 0.006 285.885)) !important;
  transition: all 0.2s ease;
}

.portal-btn:hover:not(:disabled) {
  color: oklch(0.3 0.006 285.885) !important;
  transform: scale(1.1);
}

.portal-btn-disabled {
  color: #c0c4cc !important;
  cursor: not-allowed;
}

.action-buttons {
  display: flex;
  gap: 2px;
}

/* 列标题带帮助图标 */
.column-header-with-help {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.help-icon {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  cursor: pointer;
}

.help-icon:hover {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

/* 表格行状态样式 */
:deep(.current-using-row > td.el-table__cell) {
  background-color: #eff6ff !important;
  transition: background-color 0.3s ease;
}

:deep(.current-using-row:hover > td.el-table__cell) {
  background-color: #dbeafe !important;
}

:deep(.disabled-row) {
  opacity: 0.8;
}

/* 封禁原因弹窗内容 */
.ban-reason-content {
  padding: 16px;
  background: #fef2f2;
  border-radius: var(--radius-md, 8px);
  border-left: 4px solid var(--destructive, oklch(0.577 0.245 27.325));
}

.ban-reason-content p {
  margin: 0;
  color: #991b1b;
  line-height: 1.6;
}

/* 添加TOKEN按钮样式 */
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
.section-alert {
  margin-bottom: 12px;
  border-radius: var(--radius-md, 8px);
}

/* 代理地址帮助提示 */
.proxy-help-tip {
  margin-top: 6px;
}

.proxy-help-link {
  color: var(--primary, oklch(0.21 0.006 285.885));
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  font-size: 12px;
  gap: 4px;
}

.proxy-help-link:hover {
  color: var(--primary, oklch(0.21 0.006 285.885));
  text-decoration: underline;
}

.proxy-help-link .el-icon {
  font-size: 14px;
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
  border-color: #cbd5e1 !important;
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

.submit-btn:disabled {
  background: #99c4fc !important;
  color: #fff !important;
}

.turnstile-section {
  text-align: center;
  padding: 12px;
  background: var(--card, oklch(1 0 0));
  border-radius: var(--radius-md, 8px);
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  min-height: 65px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.turnstile-widget {
  display: flex;
  justify-content: center;
}

.turnstile-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 14px;
}

.turnstile-loading .loading-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* 分页 */
.pagination-wrapper {
  margin-top: 16px;
  display: flex;
  justify-content: center;
}

/* 增强标签样式 */
.enhanced-tag {
  cursor: pointer;
}

/* 增强对话框内容 */
.enhance-dialog-content {
  padding: 0 8px;
}

.enhance-tip {
  margin: 0 0 8px 0;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 14px;
}

.enhance-warning {
  margin: 0 0 16px 0;
  color: var(--destructive, oklch(0.577 0.245 27.325));
  font-size: 13px;
  font-weight: 500;
  padding: 8px 12px;
  background: #fef2f2;
  border-radius: var(--radius-sm, 6px);
  border-left: 3px solid var(--destructive, oklch(0.577 0.245 27.325));
}

.current-binding-info {
  margin-top: 16px;
}

.channel-remark {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 12px;
  margin-left: 4px;
}

</style>

<style>
/* 添加TOKEN抽屉全局样式 */
.add-token-drawer.el-drawer {
  border-left: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.add-token-drawer .el-drawer__header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
  margin-bottom: 0;
}

.add-token-drawer .el-drawer__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.add-token-drawer .el-drawer__body {
  padding: 14px 16px 10px 16px;
  overflow-y: auto;
}

.add-token-drawer .el-drawer__footer {
  padding: 12px 20px;
  border-top: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.add-token-drawer .el-form-item__label {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-weight: 500;
}

.add-token-drawer .el-form-item {
  margin-bottom: 10px;
}

.add-token-drawer .el-form-item__label {
  padding-bottom: 4px;
}

.add-token-drawer .el-input__wrapper,
.add-token-drawer .el-textarea__inner {
  border-radius: 8px !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32)) !important;
  box-shadow: none !important;
}

.add-token-drawer .el-input__wrapper:hover,
.add-token-drawer .el-textarea__inner:hover {
  border-color: var(--ring, oklch(0.705 0.015 286.067)) !important;
}

.add-token-drawer .el-input__wrapper.is-focus,
.add-token-drawer .el-textarea__inner:focus {
  border-color: var(--primary, oklch(0.21 0.006 285.885)) !important;
}

.add-token-drawer .el-radio__label {
  font-size: 14px;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.add-token-drawer .el-radio__input.is-checked .el-radio__inner {
  background-color: var(--primary, oklch(0.21 0.006 285.885));
  border-color: var(--primary, oklch(0.21 0.006 285.885));
}

.add-token-drawer .el-radio__input.is-checked + .el-radio__label {
  color: var(--primary, oklch(0.21 0.006 285.885));
}

.add-token-drawer .el-radio {
  margin-right: 12px;
}

.add-token-drawer .el-radio:last-child {
  margin-right: 0;
}

.add-token-drawer .el-alert {
  border-radius: 8px;
}

/* TOKEN操作弹窗样式 */
.token-action-dialog {
  border-radius: var(--radius-lg, 10px) !important;
  overflow: hidden;
}

.token-action-dialog .el-message-box__header {
  padding: 20px 24px 12px 24px;
  text-align: center;
}

.token-action-dialog .el-message-box__title {
  font-size: 17px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.token-action-dialog .el-message-box__headerbtn {
  top: 16px;
  right: 16px;
}

.token-action-dialog .el-message-box__content {
  padding: 8px 24px 20px 24px;
  text-align: center;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 14px;
  line-height: 1.6;
}

.token-action-dialog .el-message-box__status {
  display: none;
}

.token-action-dialog .el-message-box__btns {
  padding: 0 24px 20px 24px;
  display: flex;
  justify-content: center;
  gap: 12px;
}

.token-action-cancel-btn {
  background: #fff !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32)) !important;
  border-radius: 8px !important;
  padding: 10px 24px !important;
  font-weight: 500 !important;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938)) !important;
  min-width: 88px !important;
  transition: all 0.2s ease !important;
}

.token-action-cancel-btn:hover {
  background: var(--secondary, oklch(0.967 0.001 286.375)) !important;
  border-color: var(--ring, oklch(0.705 0.015 286.067)) !important;
  color: var(--foreground, oklch(0.141 0.005 285.823)) !important;
}

.token-action-confirm-btn {
  border-radius: 8px !important;
  padding: 10px 24px !important;
  font-weight: 500 !important;
  min-width: 88px !important;
  border: none !important;
  transition: all 0.2s ease !important;
}

.token-action-confirm-btn.primary {
  background: #0064FA !important;
  color: #fff !important;
}

.token-action-confirm-btn.primary:hover {
  background: #0052cc !important;
}

.token-action-confirm-btn.danger {
  background: #ef4444 !important;
  color: #fff !important;
}

.token-action-confirm-btn.danger:hover {
  background: #dc2626 !important;
}

/* 增强TOKEN对话框全局样式 */
.enhance-dialog.el-dialog {
  border-radius: var(--radius-lg, 10px) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
}

.el-overlay .enhance-dialog.el-dialog {
  border-radius: var(--radius-lg, 10px) !important;
}

.enhance-dialog .el-dialog__header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
}

.enhance-dialog .el-dialog__title {
  font-size: 16px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.enhance-dialog .el-dialog__body {
  padding: 20px;
}

.enhance-dialog .el-dialog__footer {
  padding: 12px 20px;
}

.enhance-dialog .el-form-item__label {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-weight: 500;
}

.enhance-dialog .el-select .el-input__wrapper {
  border-radius: 8px !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32)) !important;
  box-shadow: none !important;
}

.enhance-dialog .el-select .el-input__wrapper:hover {
  border-color: var(--ring, oklch(0.705 0.015 286.067)) !important;
}

.enhance-dialog .el-select .el-input.is-focus .el-input__wrapper {
  border-color: var(--primary, oklch(0.21 0.006 285.885)) !important;
}

.enhance-dialog .el-alert {
  border-radius: 8px;
}

/* 封禁原因对话框全局样式 */
.ban-reason-dialog.el-dialog {
  border-radius: var(--radius-lg, 10px) !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
}

.el-overlay .ban-reason-dialog.el-dialog {
  border-radius: var(--radius-lg, 10px) !important;
}

.ban-reason-dialog .el-dialog__header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border, oklch(0.92 0.004 286.32));
  text-align: center;
}

.ban-reason-dialog .el-dialog__title {
  font-size: 17px;
  font-weight: 600;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.ban-reason-dialog .el-dialog__body {
  padding: 20px;
}

.ban-reason-dialog .el-dialog__footer {
  padding: 12px 20px;
  display: flex;
  justify-content: center;
}

.ban-reason-dialog .el-dialog__footer .el-button {
  min-width: 100px;
  padding: 10px 24px !important;
  border-radius: 8px !important;
  font-weight: 500 !important;
  font-size: 14px !important;
  background: #fff !important;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32)) !important;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938)) !important;
  transition: all 0.2s ease !important;
}

.ban-reason-dialog .el-dialog__footer .el-button:hover {
  border-color: var(--ring, oklch(0.705 0.015 286.067)) !important;
  color: var(--foreground, oklch(0.141 0.005 285.823)) !important;
  background: var(--secondary, oklch(0.967 0.001 286.375)) !important;
}
</style>
