<template>
  <div class="dashboard">
    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-cards">
      <el-col :xs="24" :sm="12" :md="8" :lg="4" :xl="4" v-for="card in statsCards" :key="card.key" class="stats-col">
        <el-card class="stat-card">
          <div class="stat-content">
            <div class="stat-icon" :class="card.iconClass">
              <el-icon><component :is="card.icon" /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">
                <span v-if="card.key === 'success_rate'">{{ (card?.value || 0) }}%</span>
                <span v-else>{{ formatNumber(card?.value || 0) }}</span>
              </div>
              <div class="stat-label">{{ card?.label || '' }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 图表区域 -->
    <el-row :gutter="20" class="charts-section">
      <!-- 请求趋势图 -->
      <el-col :xs="24" :lg="16">
        <el-card class="chart-card">
          <template #header>
            <div class="card-header">
              <span>请求趋势</span>
              <el-select v-model="trendPeriod" size="small" style="width: 120px" @change="onTrendPeriodChange">
                <el-option label="最近7天" value="7d" />
                <el-option label="最近30天" value="30d" />
                <el-option label="最近90天" value="90d" />
              </el-select>
            </div>
          </template>
          <div class="chart-container">
            <v-chart :option="trendChartOption" :loading="loading" />
          </div>
        </el-card>
      </el-col>

      <!-- Token状态分布 -->
      <el-col :xs="24" :lg="8">
        <el-card class="chart-card">
          <template #header>
            <span>Token状态分布</span>
          </template>
          <div class="chart-container">
            <v-chart :option="pieChartOption" :loading="loading" />
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- 热门Token -->
    <el-row :gutter="20">
      <el-col :xs="24">
        <el-card class="table-card">
          <template #header>
            <span>热门Token</span>
          </template>
          <el-table :data="topTokens" stripe v-loading="loading" element-loading-text="加载中...">
            <el-table-column label="TOKEN">
              <template #default="{ row }">
                <code v-if="row && row.token">{{ maskToken(row.token) }}</code>
                <span v-else class="text-muted">-</span>
              </template>
            </el-table-column>
            <el-table-column prop="tenant_url" label="租户地址" />
            <el-table-column prop="success_requests" label="请求数" align="right">
              <template #default="{ row }">
                {{ row?.success_requests || 0 }}
              </template>
            </el-table-column>
            <el-table-column prop="total_requests" label="请求次数" align="right">
              <template #default="{ row }">
                {{ row?.total_requests || 0 }}
              </template>
            </el-table-column>
            <el-table-column prop="success_rate" label="成功率" align="right">
              <template #default="{ row }">
                <el-tag :type="getSuccessRateType(row?.success_rate ?? 0)" v-if="row">
                  {{ (row.success_rate ?? 0) }}%
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart, PieChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent
} from 'echarts/components'
import VChart from 'vue-echarts'
import { useStatsStore } from '@/store'

// 注册ECharts组件
use([
  CanvasRenderer,
  LineChart,
  PieChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent
])

const statsStore = useStatsStore()

// 响应式数据
const loading = ref(false)
const trendPeriod = ref('7d')
const topTokens = ref([])
const trendData = ref([])

// 计算属性
const overview = computed(() => statsStore.overview || {})

const statsCards = computed(() => [
  {
    key: 'total_requests',
    label: '总请求次数',
    value: overview.value.total_requests ?? 0,
    icon: 'TrendCharts',
    iconClass: 'icon-blue'
  },
  {
    key: 'active_tokens',
    label: '正常Token',
    value: overview.value.active_tokens ?? 0,
    icon: 'Key',
    iconClass: 'icon-green'
  },
  {
    key: 'expired_tokens',
    label: '过期Token',
    value: overview.value.expired_tokens ?? 0,
    icon: 'Clock',
    iconClass: 'icon-warning'
  },
  {
    key: 'disabled_tokens',
    label: '禁用Token',
    value: overview.value.disabled_tokens ?? 0,
    icon: 'Lock',
    iconClass: 'icon-danger'
  },
  {
    key: 'success_rate',
    label: '成功率',
    value: overview.value.success_rate ?? 0,
    icon: 'CircleCheck',
    iconClass: 'icon-success'
  }
])

const trendChartOption = computed(() => {
  // 确保trendData是数组
  const data = Array.isArray(trendData.value) ? trendData.value : []
  const dates = data.map(item => item.date || '')
  const requests = data.map(item => item.requests || 0)
  const success = data.map(item => item.success || 0)
  const errors = data.map(item => item.errors || 0)

  return {
    tooltip: {
      trigger: 'axis'
    },
    legend: {
      data: ['请求次数', '成功数', '错误数']
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: dates
    },
    yAxis: {
      type: 'value'
    },
    series: [
      {
        name: '请求次数',
        type: 'line',
        data: requests,
        smooth: true
      },
      {
        name: '成功数',
        type: 'line',
        data: success,
        smooth: true
      },
      {
        name: '错误数',
        type: 'line',
        data: errors,
        smooth: true
      }
    ]
  }
})

const pieChartOption = computed(() => ({
  tooltip: {
    trigger: 'item',
    formatter: '{a} <br/>{b}: {c} ({d}%)'
  },
  legend: {
    orient: 'vertical',
    left: 'left'
  },
  series: [
    {
      name: 'Token状态',
      type: 'pie',
      radius: '50%',
      data: [
        {
          value: overview.value.active_tokens || 0,
          name: '正常',
          itemStyle: { color: '#43e97b' }
        },
        {
          value: overview.value.expired_tokens || 0,
          name: '过期',
          itemStyle: { color: '#f093fb' }
        },
        {
          value: overview.value.disabled_tokens || 0,
          name: '禁用',
          itemStyle: { color: '#ff6b6b' }
        }
      ]
    }
  ]
}))



// 方法
const formatNumber = (num) => {
  if (typeof num !== 'number' || isNaN(num)) return '0'
  if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M'
  } else if (num >= 1000) {
    return (num / 1000).toFixed(1) + 'K'
  }
  return num.toString()
}

const getSuccessRateType = (rate) => {
  if (rate >= 95) return 'success'
  if (rate >= 90) return 'warning'
  return 'danger'
}

const maskToken = (token) => {
  if (!token) return ''
  if (token.length <= 12) return token // 如果TOKEN太短，直接显示
  const prefix = token.substring(0, 6)
  const suffix = token.substring(token.length - 6)
  return `${prefix}****${suffix}`
}



const loadData = async () => {
  loading.value = true
  try {
    await statsStore.fetchOverview()
    const topTokensData = await statsStore.fetchTopTokens(5)
    topTokens.value = topTokensData || []

    // 获取趋势数据
    const days = parseInt(trendPeriod.value.replace('d', ''))
    const trend = await statsStore.fetchTrendData(days)
    trendData.value = trend || []
  } catch (error) {
    console.error('加载数据失败:', error)
    // 设置默认数据
    topTokens.value = []
    trendData.value = []
  } finally {
    loading.value = false
  }
}

// 监听趋势周期变化
const onTrendPeriodChange = async () => {
  try {
    const days = parseInt(trendPeriod.value.replace('d', ''))
    const trend = await statsStore.fetchTrendData(days)
    trendData.value = trend || []
  } catch (error) {
    console.error('获取趋势数据失败:', error)
    trendData.value = []
  }
}

// 生命周期
onMounted(() => {
  loadData()
})
</script>

<style scoped>
.dashboard {
  width: 100%;
  min-height: calc(100vh - 140px);
}

.stats-cards {
  margin-bottom: 20px;
}

/* 5个卡片均分宽度 */
.stats-col {
  flex: 1;
  max-width: 20%; /* 100% / 5 = 20% */
}

@media (max-width: 1200px) {
  .stats-col {
    max-width: 25%; /* 4个一行 */
  }
}

@media (max-width: 768px) {
  .stats-col {
    max-width: 50%; /* 2个一行 */
  }
}

@media (max-width: 576px) {
  .stats-col {
    max-width: 100%; /* 1个一行 */
  }
}

.stat-card {
  height: 130px;
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12);
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  color: white;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
}

.icon-blue { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); }
.icon-green { background: linear-gradient(135deg, #43e97b 0%, #38f9d7 100%); }
.icon-warning { background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); }
.icon-danger { background: linear-gradient(135deg, #ff6b6b 0%, #ee5a52 100%); }
.icon-success { background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%); }

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--text-color-primary);
  line-height: 1.2;
}

.stat-label {
  font-size: 15px;
  color: var(--text-color-secondary);
  margin-top: 6px;
  font-weight: 500;
}



.charts-section {
  margin-bottom: 20px;
}

.chart-card,
.table-card {
  height: 420px;
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  transition: all 0.3s ease;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.chart-card:hover,
.table-card:hover {
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.chart-container {
  height: 320px;
}


</style>
