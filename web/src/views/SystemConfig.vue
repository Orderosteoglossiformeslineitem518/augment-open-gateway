<template>
  <div class="system-config-page">
    <!-- 配置统计信息 -->
    <el-card class="stats-card">
      <template #header>
        <span>配置统计</span>
      </template>
      <div class="config-stats">
        <div class="stat-item">
          <div class="stat-icon blue"><el-icon><User /></el-icon></div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.total_users || 0 }}</div>
            <div class="stat-label">总用户数</div>
          </div>
        </div>
        <div class="stat-item">
          <div class="stat-icon green"><el-icon><Check /></el-icon></div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.active_users || 0 }}</div>
            <div class="stat-label">活跃用户</div>
          </div>
        </div>
        <div class="stat-item">
          <div class="stat-icon orange"><el-icon><Key /></el-icon></div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.assigned_tokens || 0 }}</div>
            <div class="stat-label">已分配令牌</div>
          </div>
        </div>
        <div class="stat-item">
          <div class="stat-icon red"><el-icon><Warning /></el-icon></div>
          <div class="stat-content">
            <div class="stat-value">{{ stats.banned_users || 0 }}</div>
            <div class="stat-label">封禁用户</div>
          </div>
        </div>
      </div>
    </el-card>

    <!-- 系统配置表单 -->
    <el-card class="config-card" v-loading="loading">
      <template #header>
        <div class="card-header">
          <span>系统配置</span>
          <el-button @click="loadConfig" :loading="loading" size="small">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </template>

      <div class="config-list">
        <!-- 用户注册 -->
        <div class="config-item">
          <div class="config-label">用户注册</div>
          <div class="config-value">
            <el-switch v-model="configForm.registration_enabled" active-text="开启" inactive-text="关闭" />
            <span class="status-text" :class="configForm.registration_enabled ? 'success' : 'danger'">
              {{ configForm.registration_enabled ? '允许新用户注册账号' : '已关闭用户注册' }}
            </span>
          </div>
        </div>

        <div class="config-divider"></div>

        <!-- 默认频率限制 -->
        <div class="config-item">
          <div class="config-label">默认频率限制</div>
          <div class="config-value">
            <el-input-number v-model="configForm.default_rate_limit" :min="1" :max="100" :step="5" controls-position="right" />
            <span class="unit-text">次/分钟</span>
          </div>
          <div class="config-desc">新用户默认的请求频率限制</div>
        </div>

        <div class="config-divider"></div>

        <!-- 维护模式 -->
        <div class="config-item">
          <div class="config-label">维护模式</div>
          <div class="config-value">
            <el-switch v-model="configForm.maintenance_mode" active-text="开启" inactive-text="关闭" />
            <span class="status-text" :class="configForm.maintenance_mode ? 'danger' : 'success'">
              {{ configForm.maintenance_mode ? '系统处于维护模式' : '系统正常运行中' }}
            </span>
          </div>
        </div>

        <!-- 维护提示信息 -->
        <div class="config-item" v-if="configForm.maintenance_mode">
          <div class="config-label">维护提示信息</div>
          <div class="config-value full">
            <el-input
              v-model="configForm.maintenance_message"
              type="textarea"
              :rows="3"
              placeholder="请输入维护期间显示给用户的提示信息"
              maxlength="500"
              show-word-limit
            />
          </div>
        </div>

        <div class="config-divider"></div>

        <!-- 操作按钮 -->
        <div class="config-actions">
          <el-button type="primary" @click="saveConfig" :loading="saving">
            <el-icon><Check /></el-icon>
            保存配置
          </el-button>
          <el-button @click="resetConfig">
            <el-icon><Refresh /></el-icon>
            重置
          </el-button>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { systemConfigAPI } from '@/api'

const loading = ref(false)
const saving = ref(false)

const configForm = reactive({
  registration_enabled: true,
  default_rate_limit: 30,
  maintenance_mode: false,
  maintenance_message: ''
})

const originalConfig = reactive({
  registration_enabled: true,
  default_rate_limit: 30,
  maintenance_mode: false,
  maintenance_message: ''
})

const stats = reactive({
  total_users: 0,
  active_users: 0,
  assigned_tokens: 0,
  banned_users: 0
})

const loadConfig = async () => {
  loading.value = true
  try {
    const data = await systemConfigAPI.get()
    Object.assign(configForm, {
      registration_enabled: data.registration_enabled,
      default_rate_limit: data.default_rate_limit,
      maintenance_mode: data.maintenance_mode,
      maintenance_message: data.maintenance_message || ''
    })
    Object.assign(originalConfig, configForm)
  } catch (error) {
    console.error('加载配置失败:', error)
  } finally {
    loading.value = false
  }
}

const saveConfig = async () => {
  saving.value = true
  try {
    await systemConfigAPI.update({
      registration_enabled: configForm.registration_enabled,
      default_rate_limit: configForm.default_rate_limit,
      maintenance_mode: configForm.maintenance_mode,
      maintenance_message: configForm.maintenance_message
    })
    ElMessage.success('配置保存成功')
    Object.assign(originalConfig, configForm)
  } catch (error) {
    console.error('保存配置失败:', error)
  } finally {
    saving.value = false
  }
}

const resetConfig = () => {
  Object.assign(configForm, originalConfig)
  ElMessage.info('配置已重置')
}

const loadStats = async () => {
  try {
    const data = await systemConfigAPI.stats()
    Object.assign(stats, {
      total_users: data.total_users || 0,
      active_users: data.active_users || 0,
      assigned_tokens: data.assigned_tokens || 0,
      banned_users: data.banned_users || 0
    })
  } catch (error) {
    console.error('加载统计数据失败:', error)
  }
}

onMounted(() => {
  loadConfig()
  loadStats()
})
</script>

<style scoped>
.system-config-page {
  width: 100%;
  min-height: calc(100vh - 140px);
}

.stats-card {
  margin-bottom: 24px;
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
}

.config-stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 20px;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px;
  background: #f8f9fa;
  border-radius: 10px;
}

.stat-icon {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  font-size: 20px;
  color: white;
}

.stat-icon.blue { background: #409eff; }
.stat-icon.green { background: #67c23a; }
.stat-icon.orange { background: #e6a23c; }
.stat-icon.red { background: #f56c6c; }

.stat-value {
  font-size: 22px;
  font-weight: 600;
  color: #303133;
}

.stat-label {
  font-size: 13px;
  color: #909399;
  margin-top: 2px;
}

.config-card {
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.config-list {
  max-width: 800px;
}

.config-item {
  padding: 10px 0;
}

.config-label {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}

.config-value {
  display: flex;
  align-items: center;
  gap: 16px;
}

.config-value.full {
  display: block;
}

.config-value.full .el-textarea {
  max-width: 500px;
}

.status-text {
  font-size: 13px;
  color: #909399;
}

.status-text.success { color: #67c23a; }
.status-text.danger { color: #f56c6c; }

.unit-text {
  font-size: 14px;
  color: #606266;
}

.config-desc {
  font-size: 12px;
  color: #909399;
  margin-top: 6px;
}

.config-divider {
  height: 1px;
  background: #ebeef5;
  margin: 4px 0;
}

.config-actions {
  padding: 12px 0 4px;
  display: flex;
  gap: 12px;
}

@media (max-width: 992px) {
  .config-stats {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 576px) {
  .config-stats {
    grid-template-columns: 1fr;
  }
  
  .config-value {
    flex-wrap: wrap;
  }
}


</style>
