<template>
  <div class="about-page">
    <!-- 顶部导航栏 -->
    <header class="page-header">
      <div class="header-left">
        <img src="/logo.svg" alt="AugmentGateway" class="header-logo" />
        <span class="header-title">AugmentGateway</span>
      </div>
      <div class="header-right">
        <button class="theme-toggle-btn" @click="toggleDarkMode" title="切换主题">
          <el-icon><Sunny v-if="isDarkMode" /><Moon v-else /></el-icon>
        </button>
        <button class="nav-btn nav-btn-outline" @click="goBack">
          <el-icon><Back /></el-icon>
          返回
        </button>
      </div>
    </header>

    <!-- 书信内容区 -->
    <main class="letter-container">
      <div class="letter-wrapper">
        <div class="letter-paper">
          <!-- 信纸装饰 -->
          <div class="letter-decoration top-left"></div>
          <div class="letter-decoration top-right"></div>
          <div class="letter-decoration bottom-left"></div>
          <div class="letter-decoration bottom-right"></div>
          
          <!-- 信件内容 -->
          <div class="letter-content" v-html="renderedContent"></div>
        </div>
      </div>
    </main>

    <!-- 页脚 -->
    <footer class="page-footer">
      <p>© 2026 Augment Gateway. All rights reserved.</p>
    </footer>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Sunny, Moon, Back } from '@element-plus/icons-vue'

const router = useRouter()

// 黑暗模式
const isDarkMode = ref(localStorage.getItem('darkMode') === 'true')

// 切换黑暗模式
const toggleDarkMode = () => {
  isDarkMode.value = !isDarkMode.value
  document.documentElement.classList.toggle('dark-mode', isDarkMode.value)
  localStorage.setItem('darkMode', isDarkMode.value ? 'true' : 'false')
}

// 返回上一页
const goBack = () => {
  router.back()
}

// 关于内容（预渲染HTML）
const renderedContent = `<h1>AugmentCode Gateway</h1>

<p>本项目名为 <strong>AugmentCode Gateway</strong>，顾名思义，它是一个为 AugmentCode 插件设计的高性能网关服务。</p>

<p>如果您正阅读这段文字，大概率是因为您和我一样——认可 AugmentCode 是当前 AI 编程插件中<strong>上下文处理最精准</strong>、<strong>ACE 检索最高效</strong>、<strong>增强提示最智能</strong>的AI插件。</p>

<p>自 AugmentCode 发布以来，我曾借助AI开发过 Augment2API、AugmentGateway 社区版 等多个衍生项目，亲历了从初期用户蜂拥注册、到后期试用机制取消的全过程。诚然，几乎所有 AI 开发工具都难逃"热度周期"与"成本反噬"的命运。但正因如此，我才更希望借助 AugmentCode 强大的本地上下文整合能力，外接可用的 Claude 系列模型，实现接近"原生体验"的开发环境来“降本增效”。</p>

<h2>为何不支持 GPT？</h2>

<p>我始终认为，GPT 系列模型在开放对话场景表现优异，但在面向工程化、结构化、可维护性要求极高的代码生成场景中，<strong>Claude 系列模型</strong>（尤其是 Claude Opus 4.5）在逻辑严谨性、长上下文一致性与调试友好度上更具优势。</p>

<hr />

<p>历经 <strong>5 个月</strong>的社区共建、高频迭代与对抗性测试，当前版本已实现核心功能 <strong>90%</strong> 的完成度，稳定性与响应质量可以满足日常开发，可以发布测试使用了。</p>

<p>当然，我也坦诚地希望：在未来，项目可通过渠道返佣、增值服务或企业定制等方式实现可持续运营。毕竟"为爱发电"令人敬佩，但唯有可持续的热爱，才能走得更远。</p>

<hr />

<p><strong>感谢您的信任与使用——</strong></p>

<p style="text-align: right; margin-top: 40px; font-style: italic;">彼方</p>
<p style="text-align: right; color: #94a3b8; font-size: 14px;">写于 2025 年 12 月 22 日</p>`

// 初始化黑暗模式
onMounted(() => {
  if (isDarkMode.value) {
    document.documentElement.classList.add('dark-mode')
  }
})
</script>


<style scoped>
.about-page {
  min-height: 100vh;
  background: linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%);
  display: flex;
  flex-direction: column;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
}

/* 顶部导航栏 */
.page-header {
  padding: 12px 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(10px);
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 100;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-logo {
  width: 28px;
  height: 28px;
}

.header-title {
  font-size: 18px;
  font-weight: 700;
  color: #1e293b;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.theme-toggle-btn {
  padding: 8px;
  border: none;
  background: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
  color: #334155;
  border-radius: 8px;
}

.theme-toggle-btn:hover {
  background: rgba(0, 0, 0, 0.05);
}

.theme-toggle-btn .el-icon {
  font-size: 20px;
}

/* 导航按钮样式 - 与首页一致 */
.nav-btn {
  padding: 8px 16px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid transparent;
  background: transparent;
  color: #1d1d1f;
  display: flex;
  align-items: center;
  gap: 6px;
  border-radius: 8px;
}

.nav-btn:hover {
  background: rgba(0, 0, 0, 0.05);
}

.nav-btn-outline {
  border-color: rgba(0, 0, 0, 0.15);
}

.nav-btn-outline:hover {
  background: #000;
  color: #fff;
  border-color: #000;
}

/* 书信容器 */
.letter-container {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 80px 24px 40px;
}

.letter-wrapper {
  max-width: 720px;
  width: 100%;
}

/* 信纸样式 */
.letter-paper {
  background: #fffef8;
  border-radius: 4px;
  padding: 48px 56px;
  position: relative;
  box-shadow: 
    0 2px 4px rgba(0, 0, 0, 0.02),
    0 8px 16px rgba(0, 0, 0, 0.04),
    0 16px 32px rgba(0, 0, 0, 0.06),
    inset 0 0 80px rgba(0, 0, 0, 0.01);
  border: 1px solid rgba(0, 0, 0, 0.06);
  /* 信纸纹理效果 */
  background-image: 
    repeating-linear-gradient(
      transparent,
      transparent 31px,
      rgba(0, 0, 0, 0.03) 31px,
      rgba(0, 0, 0, 0.03) 32px
    );
  background-position: 0 24px;
}

/* 信纸装饰角 */
.letter-decoration {
  position: absolute;
  width: 24px;
  height: 24px;
  border: 2px solid rgba(102, 126, 234, 0.2);
}

.letter-decoration.top-left {
  top: 16px;
  left: 16px;
  border-right: none;
  border-bottom: none;
}

.letter-decoration.top-right {
  top: 16px;
  right: 16px;
  border-left: none;
  border-bottom: none;
}

.letter-decoration.bottom-left {
  bottom: 16px;
  left: 16px;
  border-right: none;
  border-top: none;
}

.letter-decoration.bottom-right {
  bottom: 16px;
  right: 16px;
  border-left: none;
  border-top: none;
}

/* 信件内容样式 */
.letter-content {
  color: #334155;
  line-height: 1.7;
  font-size: 15px;
}

.letter-content :deep(h1) {
  font-size: 24px;
  font-weight: 700;
  color: #1e293b;
  margin-bottom: 20px;
  padding-bottom: 12px;
  border-bottom: 2px solid rgba(102, 126, 234, 0.2);
  text-align: center;
}

.letter-content :deep(h2) {
  font-size: 17px;
  font-weight: 600;
  color: #334155;
  margin-top: 20px;
  margin-bottom: 12px;
}

.letter-content :deep(p) {
  margin-bottom: 12px;
  text-indent: 2em;
}

.letter-content :deep(p:first-of-type) {
  text-indent: 0;
}

.letter-content :deep(strong) {
  color: #667eea;
  font-weight: 600;
}

.letter-content :deep(hr) {
  border: none;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(102, 126, 234, 0.3), transparent);
  margin: 20px 0;
}

/* 页脚 */
.page-footer {
  padding: 16px 32px;
  text-align: center;
  background: transparent;
}

.page-footer p {
  margin: 0;
  font-size: 13px;
  color: #64748b;
}

.footer-link {
  color: #667eea;
  text-decoration: none;
  font-weight: 500;
}

.footer-link:hover {
  text-decoration: underline;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .letter-container {
    padding: 70px 16px 24px;
  }
  
  .letter-paper {
    padding: 32px 24px;
  }
  
  .letter-content :deep(h1) {
    font-size: 22px;
  }
  
  .page-header {
    padding: 10px 16px;
  }
  
  .header-title {
    display: none;
  }
}

/* 黑暗模式 */
:root.dark-mode .about-page {
  background: linear-gradient(135deg, #16161A 0%, #1e1e22 100%);
}

:root.dark-mode .page-header {
  background: rgba(22, 22, 26, 0.9);
  border-bottom-color: rgba(255, 255, 255, 0.08);
}

:root.dark-mode .header-title {
  color: #f5f5f7;
}

:root.dark-mode .header-logo {
  filter: invert(1) hue-rotate(180deg);
}

:root.dark-mode .theme-toggle-btn {
  color: #f5f5f7;
}

:root.dark-mode .theme-toggle-btn:hover {
  background: rgba(255, 255, 255, 0.1);
}

:root.dark-mode .letter-paper {
  background: #1e1e22;
  border-color: rgba(255, 255, 255, 0.08);
  box-shadow: 
    0 2px 4px rgba(0, 0, 0, 0.1),
    0 8px 16px rgba(0, 0, 0, 0.15),
    0 16px 32px rgba(0, 0, 0, 0.2);
  background-image: 
    repeating-linear-gradient(
      transparent,
      transparent 31px,
      rgba(255, 255, 255, 0.03) 31px,
      rgba(255, 255, 255, 0.03) 32px
    );
}

:root.dark-mode .letter-decoration {
  border-color: rgba(77, 159, 255, 0.3);
}

:root.dark-mode .letter-content {
  color: #e5e5e7;
}

:root.dark-mode .letter-content :deep(h1) {
  color: #f5f5f7;
  border-bottom-color: rgba(77, 159, 255, 0.3);
}

:root.dark-mode .letter-content :deep(h2) {
  color: #e5e5e7;
}

:root.dark-mode .letter-content :deep(strong) {
  color: #4d9fff;
}

:root.dark-mode .letter-content :deep(hr) {
  background: linear-gradient(90deg, transparent, rgba(77, 159, 255, 0.4), transparent);
}

:root.dark-mode .page-footer p {
  color: #86868b;
}

:root.dark-mode .footer-link {
  color: #4d9fff;
}

/* 黑暗模式 - 导航按钮 */
:root.dark-mode .nav-btn {
  color: #f5f5f7;
}

:root.dark-mode .nav-btn:hover {
  background: rgba(255, 255, 255, 0.1);
}

:root.dark-mode .nav-btn-outline {
  border-color: rgba(255, 255, 255, 0.2);
}

:root.dark-mode .nav-btn-outline:hover {
  background: #fff;
  color: #000;
}
</style>
