<template>
  <el-dialog
    :model-value="modelValue"
    :title="`Token统计 - ${token?.name || ''}`"
    width="800px"
    @update:model-value="$emit('update:modelValue', $event)"
    @open="loadStats"
  >
    <div class="stats-content" v-loading="loading">
      <!-- 统计卡片 -->
      <el-row :gutter="20" class="stats-cards">
        <el-col :span="6" v-for="card in statsCards" :key="card.key">
          <div class="stat-card">
            <div class="stat-icon" :class="card.iconClass">
              <el-icon><component :is="card.icon" /></el-icon>
            </div>
            <div class="stat-info">
              <div class="stat-value">{{ formatNumber(card.value) }}</div>
              <div class="stat-label">{{ card.label }}</div>
            </div>
          </div>
        </el-col>
      </el-row>

      <!-- 使用趋势图 -->
      <el-card class="chart-card">
        <template #header>
          <div class="card-header">
            <span>使用趋势</span>
            <el-select v-model="period" size="small" style="width: 120px" @change="loadUsageHistory">
              <el-option label="最近7天" value="7" />
              <el-option label="最近30天" value="30" />
              <el-option label="最近90天" value="90" />
            </el-select>
          </div>
        </template>
        <div class="chart-container">
          <div v-if="!usageHistory || usageHistory.length === 0" class="no-data-placeholder">
            <el-empty description="暂无数据" :image-size="80" />
          </div>
          <v-chart v-else :option="chartOption" :loading="chartLoading" />
        </div>
      </el-card>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="$emit('update:modelValue', false)">关闭</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { LineChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent
} from 'echarts/components'
import VChart from 'vue-echarts'
import { useStatsStore } from '@/store'
import { formatNumber } from '@/utils'

// 注册ECharts组件
use([
  CanvasRenderer,
  LineChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent
])

const props = defineProps({
  modelValue: Boolean,
  token: {
    type: Object,
    default: null
  }
})

const emit = defineEmits(['update:modelValue'])

const statsStore = useStatsStore()

// 响应式数据
const loading = ref(false)
const chartLoading = ref(false)
const period = ref('7')
const stats = ref(null)
const usageHistory = ref([])

// 计算属性
const statsCards = computed(() => [
  {
    key: 'total_requests',
    label: '总请求次数',
    value: stats.value?.total_requests || 0,
    icon: 'DataLine',
    iconClass: 'icon-blue'
  },
  {
    key: 'success_requests',
    label: '成功请求',
    value: stats.value?.success_requests || 0,
    icon: 'CircleCheck',
    iconClass: 'icon-green'
  },
  {
    key: 'error_requests',
    label: '错误请求',
    value: stats.value?.error_requests || 0,
    icon: 'CircleClose',
    iconClass: 'icon-red'
  },
  {
    key: 'success_rate',
    label: '成功率',
    value: stats.value?.success_rate || 0,
    icon: 'TrendCharts',
    iconClass: 'icon-orange'
  }
])

const chartOption = computed(() => ({
  tooltip: {
    trigger: 'axis',
    formatter: (params) => {
      let result = `${params[0].axisValue}<br/>`
      params.forEach(param => {
        result += `${param.marker}${param.seriesName}: ${param.value}<br/>`
      })
      return result
    }
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
    data: (usageHistory.value || []).map(item => item.date)
  },
  yAxis: {
    type: 'value'
  },
  series: [
    {
      name: '请求次数',
      type: 'line',
      data: (usageHistory.value || []).map(item => item.requests),
      smooth: true
    },
    {
      name: '成功数',
      type: 'line',
      data: (usageHistory.value || []).map(item => item.success),
      smooth: true
    },
    {
      name: '错误数',
      type: 'line',
      data: (usageHistory.value || []).map(item => item.errors),
      smooth: true
    }
  ]
}))

const loadStats = async () => {
  if (!props.token?.id) return
  
  loading.value = true
  try {
    stats.value = await statsStore.fetchTokenStats(props.token.id)
    await loadUsageHistory()
  } catch (error) {
    console.error('加载统计数据失败:', error)
  } finally {
    loading.value = false
  }
}

const loadUsageHistory = async () => {
  if (!props.token?.id) return
  
  chartLoading.value = true
  try {
    usageHistory.value = await statsStore.fetchUsageHistory({
      token_id: props.token.id,
      days: parseInt(period.value)
    })
  } catch (error) {
    console.error('加载使用历史失败:', error)
  } finally {
    chartLoading.value = false
  }
}

// 监听token变化
watch(() => props.token, (newToken) => {
  if (newToken) {
    stats.value = null
    usageHistory.value = []
  }
})
</script>

<style scoped>
.stats-content {
  min-height: 400px;
}

.stats-cards {
  margin-bottom: 20px;
}

.stat-card {
  background: var(--bg-color);
  border: 1px solid var(--border-color-lighter);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  color: white;
}

.icon-blue { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); }
.icon-green { background: linear-gradient(135deg, #56ab2f 0%, #a8e6cf 100%); }
.icon-red { background: linear-gradient(135deg, #ff416c 0%, #ff4b2b 100%); }
.icon-orange { background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); }

.stat-info {
  flex: 1;
}

.stat-value {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-color-primary);
}

.stat-label {
  font-size: 12px;
  color: var(--text-color-secondary);
  margin-top: 4px;
}

.chart-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.chart-container {
  height: 300px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.no-data-placeholder {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
}
</style>
