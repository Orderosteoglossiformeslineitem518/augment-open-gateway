<template>
  <div class="panel data-panel">
    <!-- 统计概览卡片 -->
    <div class="stats-cards">
      <div class="stat-card">
        <div class="stat-icon requests">
          <el-icon><DataLine /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-value">{{ tokenAccountStats.total_count }}</span>
          <span class="stat-label">总账号数</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon success">
          <el-icon><CircleCheck /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-value success">{{ tokenAccountStats.available_count }}</span>
          <span class="stat-label">可用账号数</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon error">
          <el-icon><CircleClose /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-value error">{{ tokenAccountStats.disabled_count }}</span>
          <span class="stat-label">封禁账号数</span>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-icon credits">
          <el-icon><Coin /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-value credits">{{ tokenAccountStats.expired_count }}</span>
          <span class="stat-label">过期账号数</span>
        </div>
      </div>
    </div>

    <!-- 调用信息与使用教程卡片容器 -->
    <div class="cards-row">
      <!-- 调用信息卡片 -->
      <div class="content-card">
        <div class="card-header">
          <h3>调用信息</h3>
        </div>
        <div class="card-body">
          <!-- 有令牌时显示调用信息 -->
          <div v-if="userToken && userToken.status === 'active'" class="info-grid">
            <div class="info-row">
              <span class="info-label">提示信息</span>
              <span class="info-tip">以下地址和令牌可以在插件下载菜单中下载的定制插件使用！</span>
            </div>
            <div class="info-row">
              <span class="info-label">调用地址</span>
              <div class="copy-field">
                <code class="code-value">{{ apiUrl }}</code>
                <el-button @click="copyApiUrl" size="small" text class="copy-btn">
                  <el-icon><DocumentCopy /></el-icon>
                </el-button>
              </div>
            </div>
            <div class="info-row">
              <span class="info-label">调用令牌</span>
              <div class="copy-field">
                <code class="code-value token">{{ formatToken(userToken.token) }}</code>
                <el-button @click="copyToken" size="small" text class="copy-btn">
                  <el-icon><DocumentCopy /></el-icon>
                </el-button>
              </div>
            </div>
          </div>
          <!-- 令牌异常时显示异常信息 -->
          <div v-else class="info-alert">
            <el-alert
              :title="getTokenErrorTitle()"
              :description="getTokenErrorMessage()"
              :type="getTokenErrorType()"
              :closable="false"
              show-icon
            />
          </div>
        </div>
      </div>

      <!-- 使用教程卡片 -->
      <div class="content-card tutorial-card">
        <div class="card-header">
          <h3>使用教程</h3>
        </div>
        <div class="card-body tutorial-body">
          <p class="tutorial-tip">
            <span class="tip-item">• 请确保账号面板拥有一个可用AugmentCode账号，分配或自行提交的均可</span>
            <span class="tip-item">• 推荐您配置外部渠道使用，官方账号仅用于项目检索、提示增强、ACE检索</span>
          </p>
          <a href="" target="_blank" class="tutorial-link">
            <el-icon><ArrowRight /></el-icon>
            <span>点击跳转查看使用教程文档</span>
          </a>
        </div>
      </div>
    </div>

    <!-- 使用统计图表 -->
    <div class="content-card">
      <div class="card-header">
        <h3 class="card-title-with-icon">
          <el-icon class="title-icon"><TrendCharts /></el-icon>
          使用统计
          <el-tooltip content="数据缓存时间5分钟，非实时更新" placement="top">
            <el-icon class="help-icon"><QuestionFilled /></el-icon>
          </el-tooltip>
        </h3>
        <el-radio-group v-model="usageStatsDaysLocal" size="small" @change="handleUsageStatsDaysChange">
          <el-radio-button :value="7">最近7天</el-radio-button>
          <el-radio-button :value="15">最近15天</el-radio-button>
          <el-radio-button :value="30">最近30天</el-radio-button>
        </el-radio-group>
      </div>
      <div class="card-body">
        <div v-if="!usageStatsLoading && usageStats.length === 0" class="empty-data">
          <el-empty description="暂无数据" :image-size="80" />
        </div>
        <div v-else ref="chartRef" class="chart-container" v-loading="usageStatsLoading"></div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import {
  DataLine,
  CircleCheck,
  CircleClose,
  Coin,
  ArrowRight,
  DocumentCopy,
  Share,
  SwitchButton,
  TrendCharts,
  QuestionFilled
} from '@element-plus/icons-vue'
import * as echarts from 'echarts'

const props = defineProps({
  tokenAccountStats: {
    type: Object,
    default: () => ({ total_count: 0, available_count: 0, disabled_count: 0, expired_count: 0 })
  },
  userInfo: {
    type: Object,
    default: null
  },
  userToken: {
    type: Object,
    default: null
  },
  apiUrl: {
    type: String,
    default: ''
  },
  usageStats: {
    type: Array,
    default: () => []
  },
  usageStatsLoading: {
    type: Boolean,
    default: false
  },
  usageStatsDays: {
    type: Number,
    default: 7
  },
  isMaintenanceMode: {
    type: Boolean,
    default: false
  },
  maintenanceMessage: {
    type: String,
    default: ''
  },
  noTokenMessage: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['update:usageStatsDays', 'fetchUsageStats'])

const usageStatsDaysLocal = ref(props.usageStatsDays)
const chartRef = ref(null)
let chartInstance = null

// 监听 usageStatsDays 变化
watch(() => props.usageStatsDays, (newVal) => {
  usageStatsDaysLocal.value = newVal
})

// 处理天数变化
const handleUsageStatsDaysChange = (val) => {
  emit('update:usageStatsDays', val)
  emit('fetchUsageStats')
}

// 格式化令牌显示
const formatToken = (token) => {
  if (!token || token.length < 20) return token
  const start = token.substring(0, 10)
  const end = token.substring(token.length - 10)
  return `${start}****${end}`
}

// 获取令牌错误标题
const getTokenErrorTitle = () => {
  if (!props.userToken) return '暂无可用令牌'
  if (props.userToken.status !== 'active') return '令牌状态异常'
  return '令牌异常'
}

// 获取令牌错误消息
const getTokenErrorMessage = () => {
  if (!props.userToken) {
    if (props.isMaintenanceMode) {
      return props.maintenanceMessage || '系统正在维护中！'
    }
    return props.noTokenMessage || '您当前没有可用的令牌，请联系管理员处理！。'
  }
  if (props.userToken.status === 'inactive') return '您的令牌已被禁用，请联系管理员处理。'
  if (props.userToken.expires_at && new Date(props.userToken.expires_at) < new Date()) {
    return '您的令牌已过期，请联系管理员更新令牌。'
  }
  if (props.userToken.max_requests > 0 && props.userToken.used_requests >= props.userToken.max_requests) {
    return '您的令牌使用次数已达上限，请联系管理员处理！'
  }
  return '令牌状态异常，请联系管理员处理。'
}

// 获取令牌错误类型
const getTokenErrorType = () => {
  if (!props.userToken) return 'warning'
  if (props.userToken.status === 'inactive') return 'error'
  return 'warning'
}

// 复制API地址
const copyApiUrl = async () => {
  try {
    await navigator.clipboard.writeText(props.apiUrl)
    ElMessage.success('API地址已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

// 复制令牌
const copyToken = async () => {
  try {
    await navigator.clipboard.writeText(props.userToken.token)
    ElMessage.success('令牌已复制到剪贴板')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

// 初始化图表
const initChart = () => {
  if (!chartRef.value) return
  
  const rect = chartRef.value.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) {
    setTimeout(initChart, 100)
    return
  }
  
  if (chartInstance) {
    chartInstance.dispose()
    chartInstance = null
  }
  
  chartInstance = echarts.init(chartRef.value)
  
  const dates = props.usageStats.map(item => item.date)
  const requestCounts = props.usageStats.map(item => item.request_count)
  const officialCounts = props.usageStats.map(item => item.official_count)
  const externalCounts = props.usageStats.map(item => item.external_count)
  
  const option = {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'line' }
    },
    legend: {
      data: ['请求次数', '官方请求', '外部渠道'],
      bottom: 0
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '15%',
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
    series: [
      {
        name: '请求次数',
        type: 'line',
        data: requestCounts,
        smooth: true,
        itemStyle: { color: '#18181b' },
        lineStyle: { width: 2 },
        areaStyle: { color: 'rgba(24, 24, 27, 0.08)' }
      },
      {
        name: '官方请求',
        type: 'line',
        data: officialCounts,
        smooth: true,
        itemStyle: { color: '#10b981' },
        lineStyle: { width: 2 },
        areaStyle: { color: 'rgba(16, 185, 129, 0.08)' }
      },
      {
        name: '外部渠道',
        type: 'line',
        data: externalCounts,
        smooth: true,
        itemStyle: { color: '#3b82f6' },
        lineStyle: { width: 2 },
        areaStyle: { color: 'rgba(59, 130, 246, 0.08)' }
      }
    ]
  }
  
  chartInstance.setOption(option)
}

// 窗口大小变化时重新渲染图表
const handleResize = () => {
  if (chartInstance) {
    chartInstance.resize()
  }
}

// 监听数据变化重新渲染图表
watch(() => props.usageStats, () => {
  nextTick(() => {
    if (props.usageStats.length > 0) {
      initChart()
    }
  })
}, { deep: true })

// 暴露方法给父组件
defineExpose({
  initChart
})

onMounted(() => {
  if (props.usageStats.length > 0) {
    nextTick(() => initChart())
  }
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  if (chartInstance) {
    chartInstance.dispose()
    chartInstance = null
  }
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* 统计卡片 */
.stats-cards {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

.stat-card {
  background: var(--card, oklch(1 0 0));
  border-radius: var(--radius-xl, 14px);
  padding: 14px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  box-shadow: var(--shadow-sm);
}

.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-lg, 10px);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
}

.stat-icon.requests {
  background: oklch(0.21 0.006 285.885 / 10%);
  color: oklch(0.21 0.006 285.885);
}

.stat-icon.success {
  background: oklch(0.65 0.2 160 / 10%);
  color: var(--color-success, #10b981);
}

.stat-icon.error {
  background: oklch(0.577 0.245 27.325 / 10%);
  color: var(--color-error, #ef4444);
}

.stat-icon.credits {
  background: oklch(0.7 0.15 80 / 10%);
  color: var(--color-warning, #f59e0b);
}

.stat-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-value {
  font-size: 20px;
  font-weight: 700;
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.stat-value.success { color: var(--color-success, #10b981); }
.stat-value.error { color: var(--color-error, #ef4444); }
.stat-value.credits { color: var(--color-warning, #f59e0b); }

.stat-label {
  font-size: 12px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

/* 卡片行容器 */
.cards-row {
  display: flex;
  gap: 20px;
}

.cards-row .content-card {
  flex: 1;
}

/* 内容卡片 */
.content-card {
  background: var(--card, oklch(1 0 0));
  border-radius: var(--radius-xl, 14px);
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
  box-shadow: var(--shadow-sm);
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
  color: var(--card-foreground, oklch(0.141 0.005 285.823));
  margin: 0;
}

.card-title-with-icon {
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-title-with-icon .title-icon {
  font-size: 18px;
  color: var(--primary, oklch(0.21 0.006 285.885));
}

.card-title-with-icon .help-icon {
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  cursor: help;
  margin-left: 4px;
}

.card-body {
  padding: 16px 20px;
}

/* 徽章样式 */
.badge {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.success-badge {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
}

.info-badge {
  background: linear-gradient(135deg, #909399 0%, #b3b6bb 100%);
  color: white;
}

/* 信息网格 */
.info-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.info-tip {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 14px;
  margin: 0;
  line-height: 1.6;
}

.info-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.info-label {
  width: 80px;
  font-size: 14px;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  flex-shrink: 0;
}

.copy-field {
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 400px;
}

.code-value {
  font-family: var(--font-mono, 'Monaco', 'Menlo', 'Consolas', monospace);
  font-size: 12px;
  color: var(--foreground, oklch(0.141 0.005 285.823));
  background: var(--muted, oklch(0.967 0.001 286.375));
  padding: 6px 10px;
  border-radius: var(--radius-sm, 6px);
  border: 1px solid var(--border, oklch(0.92 0.004 286.32));
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 320px;
}

.code-value.token {
  max-width: 260px;
}

.copy-btn {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
}

.copy-btn:hover {
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

.info-alert {
  padding: 8px 0;
}

/* 使用教程卡片 */
.tutorial-body {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
}

.tutorial-tip {
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  font-size: 14px;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.tip-item {
  display: block;
  line-height: 1.6;
}

.tutorial-link {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--primary, oklch(0.21 0.006 285.885));
  font-size: 14px;
  text-decoration: none;
  padding: 8px 12px;
  margin-left: -12px;
  border-radius: var(--radius-md, 8px);
  transition: all 0.2s;
}

.tutorial-link:hover {
  background: var(--accent, oklch(0.967 0.001 286.375));
}

.tutorial-link .el-icon {
  font-size: 16px;
}

/* 时间选择器 —— 匹配设计系统 Tabs 风格 */
:deep(.el-radio-group) {
  background: var(--muted, oklch(0.967 0.001 286.375));
  border-radius: var(--radius-md, 8px);
  padding: 3px;
  gap: 0;
  border: none;
  box-shadow: none;
}

:deep(.el-radio-button__inner) {
  background: transparent;
  border: none !important;
  border-radius: var(--radius-sm, 6px) !important;
  padding: 5px 14px;
  font-size: 13px;
  font-weight: 500;
  color: var(--muted-foreground, oklch(0.552 0.016 285.938));
  box-shadow: none !important;
  transition: background-color 0.2s ease, color 0.2s ease, box-shadow 0.2s ease;
  line-height: 1.4;
}

:deep(.el-radio-button__inner:hover) {
  color: var(--foreground, oklch(0.141 0.005 285.823));
}

:deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  background: var(--card, oklch(1 0 0));
  color: var(--foreground, oklch(0.141 0.005 285.823));
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px -1px rgba(0, 0, 0, 0.1) !important;
  font-weight: 600;
}

:deep(.el-radio-button:first-child .el-radio-button__inner),
:deep(.el-radio-button:last-child .el-radio-button__inner) {
  border-radius: var(--radius-sm, 6px) !important;
}

:deep(.el-radio-button) {
  --el-radio-button-checked-bg-color: transparent;
  --el-radio-button-checked-text-color: var(--foreground, oklch(0.141 0.005 285.823));
  --el-radio-button-checked-border-color: transparent;
}

/* 图表容器 */
.chart-container {
  height: 320px;
  width: 100%;
}

/* 空数据提示 */
.empty-data {
  height: 320px;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* 响应式设计 */
@media (max-width: 1024px) {
  .stats-cards {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .stats-cards {
    grid-template-columns: repeat(2, 1fr);
  }
  
  .cards-row {
    flex-direction: column;
  }
}

@media (max-width: 480px) {
  .stats-cards {
    grid-template-columns: 1fr;
  }
  
  .info-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
  
  .info-label {
    width: auto;
  }
  
  .copy-field {
    width: 100%;
  }
  
  .code-value {
    font-size: 12px;
  }
}
</style>
