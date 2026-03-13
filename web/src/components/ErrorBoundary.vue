<template>
  <div v-if="hasError" class="error-boundary">
    <el-card class="error-card">
      <div class="error-content">
        <el-icon class="error-icon" size="48">
          <Warning />
        </el-icon>
        <h3>页面加载出错</h3>
        <p>{{ errorMessage }}</p>
        <div class="error-actions">
          <el-button type="primary" @click="retry">重试</el-button>
          <el-button @click="goHome">返回首页</el-button>
        </div>
      </div>
    </el-card>
  </div>
  <div v-else>
    <slot />
  </div>
</template>

<script setup>
import { ref, onErrorCaptured } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Warning } from '@element-plus/icons-vue'

const router = useRouter()
const hasError = ref(false)
const errorMessage = ref('')

// 捕获组件内的错误
onErrorCaptured((error, vm, info) => {
  console.error('Error caught by ErrorBoundary:', error, info)
  hasError.value = true
  errorMessage.value = error.message || '未知错误'
  ElMessage.error('页面渲染出错，请重试')
  return false // 阻止错误继续传播
})

// 重试方法
const retry = () => {
  hasError.value = false
  errorMessage.value = ''
  // 刷新当前路由
  router.go(0)
}

// 返回首页
const goHome = () => {
  hasError.value = false
  errorMessage.value = ''
  router.push('/')
}
</script>

<style scoped>
.error-boundary {
  min-height: 400px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.error-card {
  max-width: 500px;
  width: 100%;
}

.error-content {
  text-align: center;
  padding: 20px;
}

.error-icon {
  color: var(--el-color-warning);
  margin-bottom: 16px;
}

.error-content h3 {
  margin: 16px 0 8px 0;
  color: var(--el-text-color-primary);
}

.error-content p {
  margin: 8px 0 24px 0;
  color: var(--el-text-color-regular);
}

.error-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
}
</style>