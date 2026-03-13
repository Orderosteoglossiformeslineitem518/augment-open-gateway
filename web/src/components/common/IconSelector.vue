<template>
  <div class="icon-selector">
    <!-- 当前选中的图标预览 -->
    <div class="selected-icon-wrapper" @click="openSelector">
      <div v-if="modelValue" class="selected-icon">
        <img :src="getIconUrl(modelValue)" :alt="modelValue" @error="handleIconError" />
        <span class="icon-name">{{ getIconLabel(modelValue) }}</span>
      </div>
      <div v-else class="placeholder">
        <el-icon><Picture /></el-icon>
        <span>点击选择图标</span>
      </div>
      <el-icon class="arrow-icon"><ArrowDown /></el-icon>
    </div>

    <!-- 图标选择弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      title="选择渠道图标"
      width="600px"
      class="icon-selector-dialog"
      :close-on-click-modal="true"
      append-to-body
    >
      <!-- 搜索框 -->
      <div class="search-box">
        <el-input
          v-model="searchKeyword"
          placeholder="搜索图标..."
          clearable
          :prefix-icon="Search"
        />
      </div>

      <!-- 图标分类列表 -->
      <div class="icon-grid-container">
        <template v-for="category in filteredCategories" :key="category.category">
          <div v-if="category.icons.length > 0" class="category-section">
            <div class="category-title">{{ category.category }}</div>
            <div class="icon-grid">
              <div
                v-for="icon in category.icons"
                :key="icon.name"
                class="icon-item"
                :class="{ active: modelValue === icon.name }"
                @click="selectIcon(icon.name)"
              >
                <img :src="getIconUrl(icon.name)" :alt="icon.name" @error="handleIconError" />
                <span class="icon-label">{{ icon.label }}</span>
              </div>
            </div>
          </div>
        </template>
        <div v-if="filteredCategories.every(c => c.icons.length === 0)" class="no-results">
          没有找到匹配的图标
        </div>
      </div>

      <!-- 清除按钮 -->
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="clearIcon">清除图标</el-button>
          <el-button @click="dialogVisible = false">取消</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { Picture, ArrowDown, Search } from '@element-plus/icons-vue'
import { ICON_CATEGORIES, getIconUrl, getIconLabel } from '@/config/lobeIcons'

const props = defineProps({
  modelValue: { type: String, default: '' }
})

const emit = defineEmits(['update:modelValue'])

const dialogVisible = ref(false)
const searchKeyword = ref('')

// 过滤后的分类列表
const filteredCategories = computed(() => {
  if (!searchKeyword.value) return ICON_CATEGORIES
  const keyword = searchKeyword.value.toLowerCase()
  return ICON_CATEGORIES.map(cat => ({
    category: cat.category,
    icons: cat.icons.filter(icon => 
      icon.name.toLowerCase().includes(keyword) || 
      icon.label.toLowerCase().includes(keyword)
    )
  }))
})

const openSelector = () => {
  dialogVisible.value = true
  searchKeyword.value = ''
}

const selectIcon = (iconName) => {
  emit('update:modelValue', iconName)
  dialogVisible.value = false
}

const clearIcon = () => {
  emit('update:modelValue', '')
  dialogVisible.value = false
}

const handleIconError = (e) => {
  e.target.style.display = 'none'
}
</script>

<style scoped>
.icon-selector { width: 100%; }

.selected-icon-wrapper {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  background: #fff;
}
.selected-icon-wrapper:hover { border-color: #0064FA; }

.selected-icon {
  display: flex;
  align-items: center;
  gap: 10px;
}
.selected-icon img { width: 24px; height: 24px; object-fit: contain; }
.icon-name { font-size: 14px; color: #303133; }

.placeholder {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #a8abb2;
  font-size: 14px;
}
.arrow-icon { color: #a8abb2; transition: transform 0.2s; }

.search-box { margin-bottom: 16px; }

.icon-grid-container { max-height: 400px; overflow-y: auto; }
.category-section { margin-bottom: 20px; }
.category-title { font-size: 13px; font-weight: 600; color: #606266; margin-bottom: 12px; }

.icon-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
}

.icon-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 8px;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}
.icon-item:hover { border-color: #0064FA; background: #f5f9ff; }
.icon-item.active { border-color: #0064FA; background: #e6f0ff; }
.icon-item img { width: 32px; height: 32px; object-fit: contain; margin-bottom: 6px; }
.icon-label { font-size: 11px; color: #606266; text-align: center; word-break: break-all; }

.no-results { text-align: center; color: #909399; padding: 40px 0; }
.dialog-footer { display: flex; justify-content: flex-end; gap: 10px; }
</style>

<!-- 弹窗圆角样式 (非scoped) -->
<style>
.icon-selector-dialog.el-dialog,
.el-overlay .icon-selector-dialog.el-dialog {
  border-radius: 16px !important;
  overflow: hidden;
}

.icon-selector-dialog .el-dialog__header,
.el-overlay .icon-selector-dialog .el-dialog__header {
  border-radius: 16px 16px 0 0;
  padding: 16px 20px;
  border-bottom: 1px solid #f1f5f9;
}

.icon-selector-dialog .el-dialog__body,
.el-overlay .icon-selector-dialog .el-dialog__body {
  padding: 20px 24px;
}

.icon-selector-dialog .el-dialog__footer,
.el-overlay .icon-selector-dialog .el-dialog__footer {
  border-radius: 0 0 16px 16px;
  padding: 16px 24px;
}
</style>

