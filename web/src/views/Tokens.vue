<template>
  <div class="tokens-page">
    <!-- 搜索和过滤 -->
    <el-card class="filter-card">
      <el-row :gutter="20" style="margin-bottom: 16px;">
        <el-col :span="6">
          <el-input
            v-model="searchQuery"
            placeholder="搜索Token名称或描述"
            clearable
            @input="handleSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
        </el-col>
        <el-col :span="6">
          <el-input
            v-model="submitterUsernameQuery"
            placeholder="搜索提交用户名"
            clearable
            @input="handleSearch"
          >
            <template #prefix>
              <el-icon><User /></el-icon>
            </template>
          </el-input>
        </el-col>
        <el-col :span="2">
          <el-select v-model="statusFilter" placeholder="状态筛选" clearable @change="handleFilter" style="width: 100%;">
            <el-option label="全部" value="" />
            <el-option label="正常" value="active" />
            <el-option label="封禁" value="disabled" />
            <el-option label="过期" value="expired" />
          </el-select>
        </el-col>
        <el-col :span="2">
          <el-select v-model="isSharedFilter" placeholder="共享状态" clearable @change="handleFilter" style="width: 100%;">
            <el-option label="全部" value="" />
            <el-option label="已共享" value="true" />
            <el-option label="未共享" value="false" />
          </el-select>
        </el-col>
        <el-col :span="8" style="text-align: right;">
          <el-button @click="handleQuery">
            <el-icon><Search /></el-icon>
            查询
          </el-button>
          <el-button 
            type="warning" 
            @click="handleBatchRefreshAuthSession"
            :disabled="selectedTokens.length === 0"
            :loading="batchRefreshing"
          >
            <el-icon><Refresh /></el-icon>
            批量刷新Session
          </el-button>
          <el-button type="primary" @click="createToken">
            <el-icon><Plus /></el-icon>
            创建Token
          </el-button>
        </el-col>
      </el-row>
    </el-card>

    <!-- Token列表 -->
    <el-card class="table-card">
        <el-table
          :data="tokenStore.tokens || []"
          :loading="tokenStore.loading"
          stripe
          @selection-change="handleSelectionChange"
          v-loading="tokenStore.loading"
          element-loading-text="加载中..."
        >
        <el-table-column type="selection" width="55" />

        <el-table-column prop="token" label="TOKEN" width="180">
          <template #default="{ row }">
            <div class="token-value" v-if="row && row.token">
              <code>{{ maskToken(row.token) }}</code>
              <el-button
                type="text"
                size="small"
                @click="copyToken(row.token)"
                title="复制完整TOKEN"
              >
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </div>
            <span v-else class="text-muted">无Token</span>
          </template>
        </el-table-column>

        <el-table-column prop="tenant_address" label="租户地址" width="160">
          <template #default="{ row }">
            <div v-if="row && row.tenant_address" class="tenant-address-cell">
              <el-link :href="row.tenant_address" target="_blank" type="primary" :title="row.tenant_address">
                {{ formatTenantAddress(row.tenant_address) }}
              </el-link>
            </div>
            <span v-else class="text-muted">未设置</span>
          </template>
        </el-table-column>

        <el-table-column prop="email" label="邮箱" width="140">
          <template #default="{ row }">
            <span v-if="row && row.email" class="text-primary">{{ row.email }}</span>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>

        <el-table-column prop="submitter_username" label="提交用户" width="110">
          <template #default="{ row }">
            <span v-if="row && row.submitter_username" class="submitter-info">
              {{ row.submitter_username }}
            </span>
            <el-tag v-else type="info" size="small">系统添加</el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="is_shared" label="共享状态" width="90">
          <template #default="{ row }">
            <el-tag v-if="row" :type="row.is_shared ? 'success' : 'warning'" size="small">
              {{ row.is_shared ? '共享' : '私有' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row?.status)" v-if="row">
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="enhanced_enabled" label="增强状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row?.enhanced_enabled ? 'success' : 'info'" v-if="row">
              {{ row.enhanced_enabled ? '开启' : '关闭' }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column prop="ban_reason" label="封禁原因" width="130">
          <template #default="{ row }">
            <span v-if="row && row.ban_reason" class="ban-reason-text" :title="row.ban_reason">
              {{ truncateText(row.ban_reason, 15) }}
            </span>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>

        <el-table-column prop="used_requests" label="使用情况" width="110">
          <template #default="{ row }">
            <div class="usage-info" v-if="row">
              <div>{{ row.used_requests || 0 }} / {{ row.max_requests || 0 }}</div>
              <el-progress
                :percentage="getUsagePercentage(row)"
                :stroke-width="4"
                :show-text="false"
                :status="getUsageStatus(row)"
              />
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="current_users_count" label="当前使用人数" width="120">
          <template #default="{ row }">
            <div v-if="row" class="users-count-cell">
              <el-button
                type="text"
                :class="{ 'users-count-btn': true, 'has-users': (row.current_users_count || 0) > 0 }"
                @click="viewTokenUsers(row)"
                :disabled="(row.current_users_count || 0) === 0"
              >
                <el-icon><User /></el-icon>
                {{ row.current_users_count || 0 }}
              </el-button>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="created_at" label="创建时间" width="140">
          <template #default="{ row }">
            <span v-if="row && row.created_at">
              {{ formatDate(row.created_at) }}
            </span>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>

        <el-table-column prop="expires_at" label="过期时间" width="140">
          <template #default="{ row }">
            <span v-if="row && row.expires_at">
              {{ formatDate(row.expires_at) }}
            </span>
            <span v-else class="text-muted">永不过期</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button-group v-if="row">
              <el-button size="small" @click="editToken(row)">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button size="small" @click="viewStats(row)">
                <el-icon><TrendCharts /></el-icon>
              </el-button>
              <el-dropdown @command="(command) => handleAction(command, row)">
                <el-button size="small">
                  <el-icon><More /></el-icon>
                </el-button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item command="portal" :disabled="!row.portal_url">
                      查看用量
                    </el-dropdown-item>
                    <el-dropdown-item
                      command="banReason"
                      :disabled="!row.auth_session || !row.ban_reason || !row.ban_reason.includes('HTTP 403')"
                    >
                      封禁原因
                    </el-dropdown-item>
                    <el-dropdown-item command="toggle" divided>
                      {{ row.status === 'active' ? '禁用' : '启用' }}
                    </el-dropdown-item>
                    <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="tokenStore.pagination.page"
          v-model:page-size="tokenStore.pagination.pageSize"
          :total="tokenStore.pagination.total || 0"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>

    <!-- 创建/编辑Token对话框 -->
    <TokenDialog
      v-model="showCreateDialog"
      :token="currentToken"
      @success="handleDialogSuccess"
    />

    <!-- Token统计对话框 -->
    <TokenStatsDialog
      v-model="showStatsDialog"
      :token="currentToken"
    />

    <!-- TOKEN使用用户对话框 -->
    <TokenUsersDialog
      v-model="showUsersDialog"
      :token="currentToken"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Search, Plus, Edit, More, TrendCharts, CopyDocument, User, Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, ElLoading } from 'element-plus'
import { useTokenStore } from '@/store'
import TokenDialog from '@/components/TokenDialog.vue'
import TokenStatsDialog from '@/components/TokenStatsDialog.vue'
import TokenUsersDialog from '@/components/TokenUsersDialog.vue'
import { formatDate, copyToClipboard } from '@/utils'
import { tokenAPI } from '@/api'

const tokenStore = useTokenStore()

// 响应式数据
const searchQuery = ref('')
const submitterUsernameQuery = ref('')
const statusFilter = ref('')
const isSharedFilter = ref('')
const selectedTokens = ref([])
const showCreateDialog = ref(false)
const showStatsDialog = ref(false)
const showUsersDialog = ref(false)
const currentToken = ref(null)
const batchRefreshing = ref(false)

// 方法
const handleSearch = () => {
  // 实现搜索逻辑
  refreshData()
}

const handleFilter = () => {
  refreshData()
}

const handleQuery = () => {
  refreshData()
}

const refreshData = () => {
  const params = {}
  if (statusFilter.value) {
    params.status = statusFilter.value
  }
  if (searchQuery.value) {
    params.search = searchQuery.value
  }
  if (submitterUsernameQuery.value) {
    params.submitter_username = submitterUsernameQuery.value
  }
  if (isSharedFilter.value) {
    params.is_shared = isSharedFilter.value
  }
  tokenStore.fetchTokens(params)
}

const handleSelectionChange = (selection) => {
  selectedTokens.value = selection
}

const handlePageChange = (page) => {
  tokenStore.pagination.page = page
  refreshData()
}

const handleSizeChange = (size) => {
  tokenStore.pagination.pageSize = size
  tokenStore.pagination.page = 1
  refreshData()
}

const maskToken = (token) => {
  if (!token) return ''
  if (token.length <= 12) return token // 如果TOKEN太短，直接显示
  const prefix = token.substring(0, 6)
  const suffix = token.substring(token.length - 6)
  return `${prefix}****${suffix}`
}

const copyToken = async (token) => {
  try {
    await copyToClipboard(token)
    ElMessage.success('Token已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

const truncateText = (text, maxLength) => {
  if (!text) return ''
  if (text.length <= maxLength) return text
  return text.substring(0, maxLength) + '...'
}

// 格式化租户地址显示
const formatTenantAddress = (address) => {
  if (!address) return ''

  try {
    const url = new URL(address)
    const hostname = url.hostname

    // 处理 *.api.augmentcode.com 格式
    if (hostname.endsWith('.api.augmentcode.com')) {
      const subdomain = hostname.split('.')[0]
      return `https://${subdomain}.****.com`
    }

    // 处理其他域名，显示前部分和后缀
    if (hostname.length > 20) {
      const parts = hostname.split('.')
      if (parts.length >= 2) {
        const firstPart = parts[0]
        const lastPart = parts[parts.length - 1]
        const secondLastPart = parts[parts.length - 2]
        return `https://${firstPart}.****.${secondLastPart}.${lastPart}`
      }
    }

    // 短域名直接显示
    return address.length > 25 ? truncateText(address, 25) : address
  } catch (e) {
    // URL解析失败，使用截断显示
    return truncateText(address, 25)
  }
}



const getStatusType = (status) => {
  const types = {
    active: 'success',
    expired: 'warning',
    disabled: 'info'
  }
  return types[status] || 'info'
}

const getStatusText = (status) => {
  const texts = {
    active: '正常',
    expired: '已过期',
    disabled: '已禁用'
  }
  return texts[status] || status
}

const getUsagePercentage = (token) => {
  if (!token || !token.max_requests || token.max_requests <= 0) return 0
  return Math.round(((token.used_requests || 0) / token.max_requests) * 100)
}

const getUsageStatus = (token) => {
  if (!token) return 'success'
  const percentage = getUsagePercentage(token)
  if (percentage >= 90) return 'exception'
  if (percentage >= 70) return 'warning'
  return 'success'
}

const createToken = () => {
  currentToken.value = null
  showCreateDialog.value = true
}

const editToken = (token) => {
  currentToken.value = { ...token }
  showCreateDialog.value = true
}

const viewStats = (token) => {
  currentToken.value = token
  showStatsDialog.value = true
}

const viewTokenUsers = (token) => {
  currentToken.value = token
  showUsersDialog.value = true
}

const handleAction = async (command, token) => {
  switch (command) {
    case 'portal':
      openPortalUrl(token.portal_url)
      break
    case 'banReason':
      await fetchBanReason(token)
      break
    case 'toggle':
      await toggleTokenStatus(token)
      break
    case 'delete':
      await deleteToken(token)
      break
  }
}

// 打开Portal URL查看用量
const openPortalUrl = (portalUrl) => {
  if (portalUrl) {
    window.open(portalUrl, '_blank')
  }
}

// 获取并展示TOKEN封禁原因
const fetchBanReason = async (token) => {
  const loading = ElLoading.service({
    lock: true,
    text: '正在查询封禁原因...',
    background: 'rgba(0, 0, 0, 0.7)'
  })

  try {
    const result = await tokenAPI.getBanReason(token.id)
    loading.close()

    // 使用弹窗展示封禁原因
    ElMessageBox.alert(result.ban_reason || '未获取到封禁原因', '封禁原因详情', {
      confirmButtonText: '确定',
      dangerouslyUseHTMLString: false,
      customClass: 'ban-reason-dialog'
    })
  } catch (error) {
    loading.close()
    // 避免重复提示：如果拦截器已处理过错误，则不再显示
    if (!error.handled && !error.silent) {
      ElMessage.error(error.message || '获取封禁原因失败')
    }
  }
}



const toggleTokenStatus = async (token) => {
  const newStatus = token.status === 'active' ? 'disabled' : 'active'
  const action = newStatus === 'active' ? '启用' : '禁用'

  try {
    await ElMessageBox.confirm(`确定要${action}此Token吗？`, '确认操作')
    await tokenStore.updateToken(token.id, { status: newStatus })
    ElMessage.success(`Token已${action}`)
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error(`${action}失败`)
    }
  }
}

const deleteToken = async (token) => {
  try {
    await ElMessageBox.confirm('确定要删除此Token吗？此操作不可恢复。', '确认删除', {
      type: 'warning'
    })
    await tokenStore.deleteToken(token.id)
    ElMessage.success('Token已删除')
  } catch (error) {
    if (error !== 'cancel' && !error.handled && !error.silent) {
      ElMessage.error('删除失败')
    }
  }
}

const handleDialogSuccess = () => {
  showCreateDialog.value = false
  currentToken.value = null
  refreshData()
}

// 批量刷新AuthSession
const handleBatchRefreshAuthSession = async () => {
  if (selectedTokens.value.length === 0) {
    ElMessage.warning('请先选择要刷新的TOKEN')
    return
  }

  try {
    await ElMessageBox.confirm(
      `确定刷新选中的 ${selectedTokens.value.length} 个TOKEN？`,
      '批量刷新',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    batchRefreshing.value = true
    const tokenIds = selectedTokens.value.map(t => t.id)
    const result = await tokenAPI.batchRefreshAuthSession(tokenIds)

    // 显示刷新结果
    const successCount = result.success_count || 0
    const failedCount = result.failed_count || 0
    const skippedCount = result.skipped_count || 0

    if (successCount > 0 && failedCount === 0) {
      ElMessage.success(`刷新完成：成功 ${successCount} 个，跳过 ${skippedCount} 个`)
    } else if (successCount > 0) {
      ElMessage.warning(`刷新完成：成功 ${successCount} 个，失败 ${failedCount} 个，跳过 ${skippedCount} 个`)
    } else if (skippedCount > 0 && failedCount === 0) {
      ElMessage.info(`所有选中的TOKEN均无AuthSession，已跳过 ${skippedCount} 个`)
    } else {
      ElMessage.error(`刷新失败：失败 ${failedCount} 个，跳过 ${skippedCount} 个`)
    }

    // 刷新列表
    refreshData()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('批量刷新AuthSession失败:', error)
    }
  } finally {
    batchRefreshing.value = false
  }
}

// 生命周期
onMounted(() => {
  refreshData()
})
</script>

<style scoped>
.tokens-page {
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







.token-name strong {
  display: block;
  color: var(--text-color-primary);
}

.token-desc {
  font-size: 12px;
  color: var(--text-color-secondary);
  margin-top: 4px;
}

.token-value {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0; /* 允许flex子元素收缩 */
}

.token-value code {
  background: var(--bg-color-page);
  padding: 4px 8px;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
  flex: 1;
  min-width: 0; /* 允许收缩 */
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.token-value .el-button {
  flex-shrink: 0; /* 防止按钮被压缩 */
  margin-left: auto; /* 确保按钮在右侧 */
}

.usage-info {
  font-size: 12px;
}

.usage-info > div:first-child {
  margin-bottom: 4px;
}

.text-muted {
  color: var(--text-color-placeholder);
}

.ban-reason-text {
  color: var(--text-color-primary);
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tenant-address-cell {
  width: 100%;
  overflow: hidden;
}

.tenant-address-cell .el-link {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

/* 使用人数列样式 */
.users-count-cell {
  display: flex;
  justify-content: center;
}

.users-count-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 6px;
  transition: all 0.3s ease;
  font-weight: 500;
}

.users-count-btn:not(:disabled) {
  color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  border: 1px solid var(--el-color-primary-light-7);
}

.users-count-btn:not(:disabled):hover {
  background: var(--el-color-primary-light-8);
  border-color: var(--el-color-primary-light-5);
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(64, 158, 255, 0.2);
}

.users-count-btn:disabled {
  color: var(--el-text-color-placeholder);
  background: var(--el-fill-color-light);
  border: 1px solid var(--el-border-color-lighter);
  cursor: not-allowed;
}

.users-count-btn.has-users {
  font-weight: 600;
}


</style>
