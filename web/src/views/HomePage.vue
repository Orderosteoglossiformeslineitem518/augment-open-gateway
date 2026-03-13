<template>
  <div class="home-page">
    <!-- 顶部导航栏 -->
    <header class="navbar" :class="{ 'navbar-scrolled': isScrolled }">
      <div class="navbar-content">
        <div class="navbar-left" @click="moveToSection(0)" style="cursor: pointer;">
          <img src="/logo.svg" alt="AugmentGateway" class="navbar-logo" />
          <span class="navbar-title">AugmentGateway</span>
        </div>
        <div class="navbar-right">
          <button class="nav-btn nav-btn-icon" @click="toggleDarkMode" title="切换主题">
            <el-icon><Sunny v-if="isDarkMode" /><Moon v-else /></el-icon>
          </button>
          <button class="nav-btn nav-btn-icon" @click="showNotification = true" title="公告">
            <el-icon><Bell /></el-icon>
          </button>
          <button class="nav-btn nav-btn-outline" @click="showLogin = true">登录</button>
          <button class="nav-btn nav-btn-primary" @click="showRegister = true">注册</button>
        </div>
      </div>
    </header>

    <!-- 主体内容 - fullPage 容器 -->
    <div ref="fullpageRef" id="fullpage">
      <!-- Hero 区域 -->
      <div class="section hero-section-wrapper">
        <div class="hero-section-container">
          <div class="hero-section">
            <div class="hero-content">
              <h1 class="hero-title">一站式享受</h1>
              <h1 class="hero-title hero-title-highlight">AugmentCode 服务</h1>
              <p class="hero-subtitle">开发者的 Vibe Coding 最佳拍档 ！</p>
              <div class="hero-actions">
                <button class="hero-btn hero-btn-primary" @click="showLogin = true">
                  立即开始
                  <el-icon><ArrowRight /></el-icon>
                </button>
              </div>
            </div>
            <div class="hero-visual">
              <div class="hero-card">
                <div class="hero-card-header">
                  <div class="hero-card-dots">
                    <span></span><span></span><span></span>
                  </div>
                </div>
                <div class="hero-card-body">
                  <div class="code-line"><span class="code-keyword">const</span> <span class="code-var">ace</span> = <span class="code-func">createEngine</span>();</div>
                  <div class="code-line"><span class="code-keyword">await</span> ace.<span class="code-func">analyze</span>(<span class="code-string">'your-project'</span>);</div>
                  <div class="code-line"><span class="code-comment">// {{ displayedComment }}<span class="cursor">|</span></span></div>
                  <div class="code-line typing-line">
                    <span class="code-var">ace</span>.<span class="code-func">{{ displayedMethod }}</span>()<span class="cursor">|</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 核心能力区域 -->
      <div class="section features-section-wrapper">
        <div id="features" class="features-section">
        <div class="section-content">
          <div class="features-header">
            <span class="features-label">井然有序</span>
            <p class="features-subtitle">从插件安装到账号配置，为开发者提供完整的 AugmentCode 编程插件解决方案</p>
          </div>
          <div class="features-grid">
            <div class="feature-card">
              <div class="feature-card-decoration decoration-purple"></div>
              <div class="feature-icon icon-purple">
                <el-icon><Box /></el-icon>
              </div>
              <h3 class="feature-title">一键部署</h3>
              <p class="feature-desc">全流程的插件安装、账号配置、渠道添加一站式解决方案，告别繁琐的重置流程</p>
            </div>
            <div class="feature-card">
              <div class="feature-card-decoration decoration-blue"></div>
              <div class="feature-icon icon-blue">
                <el-icon><Connection /></el-icon>
              </div>
              <h3 class="feature-title">无缝中转</h3>
              <p class="feature-desc">完善且持续迭代的Claude渠道中转支持，稳定可靠的服务保障，让你专注于代码本身</p>
            </div>
            <div class="feature-card">
              <div class="feature-card-decoration decoration-cyan"></div>
              <div class="feature-icon icon-cyan">
                <el-icon><Lock /></el-icon>
              </div>
              <h3 class="feature-title">安全稳定</h3>
              <p class="feature-desc">基于公开社区同类服务迭代超 5 个月的防护策略，为你的账号安全保驾护航</p>
            </div>
            <div class="feature-card">
              <div class="feature-card-decoration decoration-green"></div>
              <div class="feature-icon icon-green">
                <el-icon><Cpu /></el-icon>
              </div>
              <h3 class="feature-title">原生体验</h3>
              <p class="feature-desc">原生注入 Augment 官方插件，自动跟随官网版本更新，第一时间体验最新功能</p>
            </div>
          </div>
          </div>
        </div>
      </div>

      <!-- 深度兼容区域 -->
      <div class="section compatibility-section-wrapper">
        <div class="compatibility-section">
          <div class="section-content">
            <div class="compat-header">
              <span class="compat-label-title">原生体验</span>
              <p class="compat-header-subtitle">使用自定义API也能畅享原生ACE上下文引擎</p>
            </div>
            <div class="compat-wrapper">
              <div class="compat-text">
                <ul class="compat-list">
                  <li>
                    <el-icon><Check /></el-icon>
                    <span>完善的Claude接口适配，无缝兼容外部渠道</span>
                  </li>
                  <li>
                    <el-icon><Check /></el-icon>
                    <span>接近原生的服务体验，内外渠道一键切换</span>
                  </li>
                  <li>
                    <el-icon><Check /></el-icon>
                    <span>实时的插件版本更新，享受最新的插件服务</span>
                  </li>
                  <li>
                    <el-icon><Check /></el-icon>
                    <span>承诺不保存敏感KEY信息，特殊信息落库加密</span>
                  </li>
                  <li>
                    <el-icon><Check /></el-icon>
                    <span>活跃的社区支持，有朋至远方来，不亦说乎</span>
                  </li>
                </ul>
              </div>
              <div class="compat-visual">
                <img src="/demo.png" alt="Demo" class="compat-demo-image" />
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- CTA 区域 + 页脚 -->
      <div class="section cta-section-wrapper">
        <!-- 水波纹背景 -->
        <div class="wave-container">
          <svg class="wave-svg" viewBox="0 0 1440 320" preserveAspectRatio="none">
            <path class="wave-path wave-path-1" d="M0,192L48,197.3C96,203,192,213,288,229.3C384,245,480,267,576,250.7C672,235,768,181,864,181.3C960,181,1056,235,1152,234.7C1248,235,1344,181,1392,154.7L1440,128L1440,320L1392,320C1344,320,1248,320,1152,320C1056,320,960,320,864,320C768,320,672,320,576,320C480,320,384,320,288,320C192,320,96,320,48,320L0,320Z"></path>
            <path class="wave-path wave-path-2" d="M0,256L48,240C96,224,192,192,288,181.3C384,171,480,181,576,197.3C672,213,768,235,864,224C960,213,1056,171,1152,165.3C1248,160,1344,192,1392,208L1440,224L1440,320L1392,320C1344,320,1248,320,1152,320C1056,320,960,320,864,320C768,320,672,320,576,320C480,320,384,320,288,320C192,320,96,320,48,320L0,320Z"></path>
            <path class="wave-path wave-path-3" d="M0,288L48,272C96,256,192,224,288,213.3C384,203,480,213,576,229.3C672,245,768,267,864,261.3C960,256,1056,224,1152,213.3C1248,203,1344,213,1392,218.7L1440,224L1440,320L1392,320C1344,320,1248,320,1152,320C1056,320,960,320,864,320C768,320,672,320,576,320C480,320,384,320,288,320C192,320,96,320,48,320L0,320Z"></path>
          </svg>
        </div>
        <div class="cta-section">
          <!-- 内容层 -->
          <div class="section-content cta-content">
            <div class="cta-wrapper">
              <h2 class="cta-title">准备好了吗？</h2>
              <p class="cta-subtitle">将 AugmentCodeGateway 应用到任何项目中，Start Vibe Coding Now !<br/></p>
              <button class="cta-btn" @click="showLogin = true">
                开始使用
                <el-icon><ArrowRight /></el-icon>
              </button>
            </div>
          </div>
        </div>
        <!-- 页脚 -->
        <footer class="footer">
          <div class="footer-content">
            <div class="footer-links">
              <a href="#" class="footer-link">文档</a>
              <router-link to="/about" class="footer-link">关于</router-link>
              <span class="footer-divider">|</span>
              <a href="#" class="footer-link">服务条款</a>
              <a href="#" class="footer-link">使用政策</a>
            </div>
            <span class="footer-copyright">© 2025 AugmentGateway. 版权所有</span>
          </div>
        </footer>
      </div>
    </div>

    <!-- 返回顶部按钮 -->
    <transition name="fade">
      <button 
        v-show="currentSection > 0" 
        class="back-to-top" 
        @click="moveToSection(0)"
        title="返回顶部"
      >
        <el-icon><ArrowUp /></el-icon>
      </button>
    </transition>

    <!-- 公告弹窗 -->
    <el-dialog
      v-model="showNotification"
      title="公告"
      width="500px"
      class="notification-modal"
    >
      <div class="notification-content">
        <p v-if="notifications.length === 0" class="no-notification">暂无公告</p>
        <div v-else class="notification-list">
          <div v-for="item in notifications" :key="item.id" class="notification-item">
            <h4>{{ item.title }}</h4>
            <p>{{ item.content }}</p>
            <span class="notification-time">{{ item.created_at }}</span>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 登录弹窗 -->
    <LoginModal 
      v-model="showLogin" 
      @switch-to-register="switchToRegister"
    />

    <!-- 注册弹窗 -->
    <RegisterModal 
      v-model="showRegister" 
      @switch-to-login="switchToLogin"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { ArrowRight, ArrowUp, Bell, Box, Connection, Lock, Check, Cpu, Sunny, Moon } from '@element-plus/icons-vue'
import LoginModal from '@/components/LoginModal.vue'
import RegisterModal from '@/components/RegisterModal.vue'
import { systemAnnouncementAPI } from '@/api'

const showLogin = ref(false)
const showRegister = ref(false)
const showNotification = ref(false)
const isScrolled = ref(false)
const notifications = ref([])
const isDarkMode = ref(false)

// 代码卡片打字机效果
const currentCodeIndex = ref(0)
const displayedMethod = ref('')
const displayedComment = ref('')
const codeCommands = [
  { method: 'suggest' },
  { method: 'retrieve' },
  { method: 'enhance' }
]
const codeComments = [
  '智能代码建议',
  '上下文检索',
  '代码增强'
]

let codeInterval = null
let typewriterTimeout = null

// 打字机效果函数
const typeWriter = (text, targetRef, callback, speed = 50) => {
  let i = 0
  targetRef.value = ''
  const type = () => {
    if (i < text.length) {
      targetRef.value += text.charAt(i)
      i++
      typewriterTimeout = setTimeout(type, speed)
    } else if (callback) {
      callback()
    }
  }
  type()
}

// 开始打字机动画
const startTypewriterAnimation = () => {
  const cmd = codeCommands[currentCodeIndex.value]
  const comment = codeComments[currentCodeIndex.value]
  
  // 先清空
  displayedMethod.value = ''
  displayedComment.value = ''
  
  // 依次打字：注释 -> 方法名
  typeWriter(comment, displayedComment, () => {
    setTimeout(() => {
      typeWriter(cmd.method, displayedMethod, null, 60)
    }, 200)
  }, 40)
}

// 切换到下一个命令
const nextCommand = () => {
  currentCodeIndex.value = (currentCodeIndex.value + 1) % codeCommands.length
  startTypewriterAnimation()
}

// 切换暗黑模式
const toggleDarkMode = () => {
  isDarkMode.value = !isDarkMode.value
  document.documentElement.classList.toggle('dark-mode', isDarkMode.value)
  localStorage.setItem('darkMode', isDarkMode.value ? 'true' : 'false')
}

// 整页滚动状态
const currentSection = ref(0)
const totalSections = 4
const isAnimating = ref(false)

const switchToRegister = () => {
  showLogin.value = false
  setTimeout(() => {
    showRegister.value = true
  }, 100)
}

const switchToLogin = () => {
  showRegister.value = false
  setTimeout(() => {
    showLogin.value = true
  }, 100)
}

// 使用原生 CSS 动画滚动到指定区域
const moveToSection = (index) => {
  if (isAnimating.value || index < 0 || index >= totalSections) return
  if (index === currentSection.value) return
  
  isAnimating.value = true
  const container = document.getElementById('fullpage')
  const targetY = -index * window.innerHeight
  
  // 过渡动画 —— 0.5s 更灵敏
  container.style.transition = 'transform 0.5s cubic-bezier(0.25, 0.46, 0.45, 0.94)'
  container.style.transform = `translateY(${targetY}px)`
  currentSection.value = index
  isScrolled.value = index > 0
  
  // 动画结束后解锁
  setTimeout(() => {
    isAnimating.value = false
  }, 500)
}

// 检查是否有弹窗打开
const isModalOpen = () => {
  return showLogin.value || showRegister.value || showNotification.value
}

// 滚轮防抖：简单的方向累积 + 阈值触发
const TRIGGER_THRESHOLD = 80 // 降低阈值，更灵敏
let accumulatedDelta = 0
let wheelResetTimer = null

// 处理滚轮事件 —— 去掉橡皮筋效果，直接触发翻页
const handleWheel = (e) => {
  if (isModalOpen()) return
  e.preventDefault()
  if (isAnimating.value) return

  // 清除重置定时器
  if (wheelResetTimer) clearTimeout(wheelResetTimer)

  // 累积滚动量
  accumulatedDelta += e.deltaY
  const direction = accumulatedDelta > 0 ? 1 : -1
  const absAccumulated = Math.abs(accumulatedDelta)

  // 边界检查：到顶/到底不做任何处理，避免"卡住"感
  const atTop = currentSection.value === 0 && direction < 0
  const atBottom = currentSection.value === totalSections - 1 && direction > 0
  if (atTop || atBottom) {
    accumulatedDelta = 0
    return
  }

  // 达到阈值立即触发翻页
  if (absAccumulated >= TRIGGER_THRESHOLD) {
    accumulatedDelta = 0
    if (direction > 0) {
      moveToSection(currentSection.value + 1)
    } else {
      moveToSection(currentSection.value - 1)
    }
    return
  }

  // 停止滚动后重置累积量
  wheelResetTimer = setTimeout(() => {
    accumulatedDelta = 0
  }, 150)
}

// 处理触摸事件
let touchStartY = 0
let touchStartTime = 0
let touchTriggered = false

const handleTouchStart = (e) => {
  touchStartY = e.touches[0].clientY
  touchStartTime = Date.now()
  touchTriggered = false
}

const handleTouchMove = (e) => {
  // 弹窗打开时不处理滚动
  if (isModalOpen()) return
  if (isAnimating.value || touchTriggered) return
  
  const touchCurrentY = e.touches[0].clientY
  const diff = touchStartY - touchCurrentY
  
  // 只要有明确的滑动方向就触发（阈值5px，仅用于过滤抖动）
  if (Math.abs(diff) > 5) {
    touchTriggered = true
    e.preventDefault()
    if (diff > 0) {
      moveToSection(currentSection.value + 1)
    } else {
      moveToSection(currentSection.value - 1)
    }
  }
}

const handleTouchEnd = (e) => {
  // 如果已经在touchmove中触发过，直接返回
  if (touchTriggered) return
  
  // 弹窗打开时不处理滚动
  if (isModalOpen()) return
  if (isAnimating.value) return
  
  const touchEndY = e.changedTouches[0].clientY
  const diff = touchStartY - touchEndY
  
  // 兜底：任何有效滑动都触发（阈值5px）
  if (Math.abs(diff) > 5) {
    if (diff > 0) {
      moveToSection(currentSection.value + 1)
    } else {
      moveToSection(currentSection.value - 1)
    }
  }
}

// 处理键盘事件
const handleKeydown = (e) => {
  if (isAnimating.value) return
  if (e.key === 'ArrowDown' || e.key === 'PageDown') {
    e.preventDefault()
    moveToSection(currentSection.value + 1)
  } else if (e.key === 'ArrowUp' || e.key === 'PageUp') {
    e.preventDefault()
    moveToSection(currentSection.value - 1)
  }
}

const fetchNotifications = async () => {
  try {
    const res = await systemAnnouncementAPI.getPublished()
    notifications.value = res.announcements || []
  } catch (e) {
    notifications.value = []
  }
}

onMounted(() => {
  fetchNotifications()
  
  // 初始化暗黑模式
  const savedDarkMode = localStorage.getItem('darkMode')
  if (savedDarkMode === 'true') {
    isDarkMode.value = true
    document.documentElement.classList.add('dark-mode')
  }
  
  // 启动代码卡片打字机效果
  startTypewriterAnimation()
  codeInterval = setInterval(nextCommand, 4000)
  
  // 添加事件监听
  const container = document.querySelector('.home-page')
  container.addEventListener('wheel', handleWheel, { passive: false })
  container.addEventListener('touchstart', handleTouchStart, { passive: true })
  container.addEventListener('touchmove', handleTouchMove, { passive: false })
  container.addEventListener('touchend', handleTouchEnd, { passive: true })
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  const container = document.querySelector('.home-page')
  if (container) {
    container.removeEventListener('wheel', handleWheel)
    container.removeEventListener('touchstart', handleTouchStart)
    container.removeEventListener('touchmove', handleTouchMove)
    container.removeEventListener('touchend', handleTouchEnd)
  }
  window.removeEventListener('keydown', handleKeydown)
  
  // 清除代码卡片定时器
  if (codeInterval) {
    clearInterval(codeInterval)
  }
  if (typewriterTimeout) {
    clearTimeout(typewriterTimeout)
  }
})
</script>

<style scoped>
.home-page {
  min-height: 100vh;
  background: #fff;
  font-family: -apple-system, BlinkMacSystemFont, 'SF Pro Display', 'Segoe UI', Roboto, sans-serif;
  overflow: hidden;
}

/* 滚动动画 */
.hero-section-container {
  opacity: 0;
  transform: translateY(40px);
  animation: fadeInUp 0.8s ease 0.1s forwards;
}

.animate-on-scroll {
  opacity: 0;
  transform: translateY(60px);
  transition: opacity 0.8s ease, transform 0.8s cubic-bezier(0.25, 0.46, 0.45, 0.94);
}

.animate-on-scroll.is-visible {
  opacity: 1;
  transform: translateY(0);
}

@keyframes fadeInUp {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* 导航栏 */
.navbar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 100;
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-bottom: 1px solid transparent;
  transition: all 0.3s ease;
}

.navbar-scrolled {
  background: rgba(255, 255, 255, 0.95);
  border-bottom-color: rgba(0, 0, 0, 0.1);
}

.navbar-content {
  max-width: 60%;
  margin: 0 auto;
  padding: 16px 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.navbar-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.navbar-logo {
  width: 32px;
  height: 32px;
}

.navbar-title {
  font-size: 18px;
  font-weight: 600;
  color: #1d1d1f;
}

.navbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

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
  margin-left: 8px;
}

.nav-btn-icon {
  padding: 8px;
  margin-left: 4px;
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

.nav-btn-primary {
  border-color: #000;
}

.nav-btn-primary:hover {
  background: #000;
  color: #fff;
}

/* 整页滚动容器 */
#fullpage {
  padding-top: 64px;
  will-change: transform;
  backface-visibility: hidden;
  -webkit-backface-visibility: hidden;
  transition: transform 0.5s cubic-bezier(0.25, 0.46, 0.45, 0.94);
}

.section {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

/* hero section：垂直居中，动态适配 */
.hero-section-wrapper {
  align-items: center;
}

.hero-section-wrapper,
.features-section-wrapper,
.compatibility-section-wrapper {
  background: #fff;
}

.cta-section-wrapper {
  background: #fff;
  flex-direction: column;
  justify-content: space-between;
  align-items: stretch;
  padding: 60px 0 0;
  position: relative;
  overflow: hidden;
}



/* Hero 容器 */
.hero-section-container {
  max-width: 60%;
  margin: 0 auto;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: clamp(8px, 2vh, 24px);
  padding: 0;
}

/* Hero 区域 */
.hero-section {
  width: 100%;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: clamp(24px, 4vw, 60px);
  align-items: center;
}

.hero-title {
  font-size: clamp(32px, 4vw, 48px);
  font-weight: 700;
  color: #1d1d1f;
  line-height: 1.1;
  margin: 0;
  letter-spacing: -0.02em;
}

.hero-title-highlight {
  font-size: clamp(28px, 3.5vw, 42px);
  background: linear-gradient(90deg, #000 0%, #666 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.hero-subtitle {
  font-size: clamp(14px, 1.5vw, 18px);
  color: #86868b;
  margin: clamp(8px, 1.5vh, 16px) 0 clamp(12px, 2vh, 24px);
  line-height: 1.5;
}

.hero-actions {
  display: flex;
  gap: 16px;
}

.hero-btn {
  padding: 12px 24px;
  font-size: 15px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid #000;
  display: flex;
  align-items: center;
  gap: 6px;
  background: transparent;
  color: #1d1d1f;
  border-radius: 10px;
}

.hero-btn:hover {
  background: #000;
  color: #fff;
}

.hero-btn-primary {
  background: #000;
  color: #fff;
}

.hero-btn-primary:hover {
  background: #333;
  border-color: #333;
}

/* Hero 视觉卡片 */
.hero-visual {
  display: flex;
  justify-content: center;
}

.hero-card {
  width: 100%;
  max-width: 480px;
  background: #1d1d1f;
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 40px 80px rgba(0, 0, 0, 0.3);
}

.hero-card-header {
  padding: 12px 16px;
  background: #2d2d2f;
}

.hero-card-dots {
  display: flex;
  gap: 8px;
}

.hero-card-dots span {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  background: #3d3d3f;
}

.hero-card-dots span:first-child { background: #ff5f57; }
.hero-card-dots span:nth-child(2) { background: #febc2e; }
.hero-card-dots span:last-child { background: #28c840; }

.hero-card-body {
  padding: 24px;
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 14px;
  line-height: 1.8;
}

.code-line {
  color: #fff;
}

.code-keyword { color: #ff79c6; }
.code-var { color: #8be9fd; }
.code-func { color: #50fa7b; }
.code-string { color: #f1fa8c; }
.code-comment { color: #6272a4; }

.typing-line {
  position: relative;
}

.cursor {
  animation: cursor-blink 0.8s infinite;
}

@keyframes cursor-blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

/* 核心能力区域 */
.features-section {
  width: 100%;
}

.features-header {
  margin-bottom: 40px;
}

.features-label {
  display: inline-block;
  font-size: 36px;
  font-weight: 700;
  color: #1d1d1f;
  margin-bottom: 16px;
}

.features-title {
  font-size: 36px;
  font-weight: 700;
  color: #1d1d1f;
  margin: 0 0 16px;
  line-height: 1.2;
}

.features-subtitle {
  font-size: 16px;
  color: #86868b;
  margin: 0;
  line-height: 1.6;
}

.section-content {
  max-width: 60%;
  margin: 0 auto;
  width: 100%;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 24px;
}

.feature-card {
  background: #fff;
  border: 1px solid rgba(0, 0, 0, 0.06);
  border-radius: 16px;
  padding: 24px;
  text-align: left;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
}

.feature-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.08);
}

.feature-card-decoration {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100px;
  opacity: 0.5;
}

.decoration-purple {
  background: linear-gradient(90deg, rgba(102, 126, 234, 0.4) 0%, rgba(102, 126, 234, 0) 60%);
}

.decoration-blue {
  background: linear-gradient(90deg, rgba(59, 130, 246, 0.4) 0%, rgba(59, 130, 246, 0) 60%);
}

.decoration-cyan {
  background: linear-gradient(90deg, rgba(6, 182, 212, 0.4) 0%, rgba(6, 182, 212, 0) 60%);
}

.decoration-green {
  background: linear-gradient(90deg, rgba(34, 197, 94, 0.4) 0%, rgba(34, 197, 94, 0) 60%);
}

.feature-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 16px;
  font-size: 20px;
  color: #fff;
  position: relative;
  z-index: 1;
}

.icon-purple {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.icon-blue {
  background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
}

.icon-cyan {
  background: linear-gradient(135deg, #06b6d4 0%, #0891b2 100%);
}

.icon-green {
  background: linear-gradient(135deg, #22c55e 0%, #16a34a 100%);
}

.feature-title {
  font-size: 18px;
  font-weight: 600;
  color: #1d1d1f;
  margin: 0 0 8px;
  position: relative;
  z-index: 1;
}

.feature-desc {
  font-size: 13px;
  color: #86868b;
  line-height: 1.6;
  margin: 0;
  position: relative;
  z-index: 1;
}

/* 深度兼容区域 */
.compatibility-section {
  width: 100%;
}

.compat-header {
  margin-bottom: 40px;
}

.compat-label-title {
  display: block;
  font-size: 36px;
  font-weight: 700;
  color: #1d1d1f;
  margin-bottom: 16px;
}

.compat-header-subtitle {
  font-size: 16px;
  color: #86868b;
  margin: 0;
  line-height: 1.6;
}

.compat-wrapper {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 80px;
  align-items: center;
}



.compat-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.compat-list li {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 16px;
  color: #1d1d1f;
  padding: 20px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.08);
}

.compat-list li:last-child {
  border-bottom: none;
}

.compat-list .el-icon {
  color: #667eea;
  font-size: 20px;
}

.compat-visual {
  display: flex;
  justify-content: center;
  align-items: flex-start;
}

.compat-demo-image {
  max-width: 70%;
  height: auto;
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.15);
  cursor: pointer;
  transition: transform 0.3s ease, box-shadow 0.3s ease;
}

.compat-demo-image:hover {
  transform: scale(1.03);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.2);
}

/* CTA 区域 */
.cta-section {
  width: 100%;
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  z-index: 1;
}

/* 水波纹容器 */
.wave-container {
  position: absolute;
  bottom: 0;
  left: 0;
  width: 100%;
  height: 50%;
  pointer-events: none;
  overflow: hidden;
  z-index: 0;
}

.wave-svg {
  position: absolute;
  bottom: 0;
  left: 0;
  width: 200%;
  height: 100%;
  animation: wave-move 30s linear infinite;
}

.wave-path {
  fill: rgba(0, 0, 0, 0.03);
}

.wave-path-1 {
  fill: rgba(0, 0, 0, 0.05);
  animation: wave-sway-1 30s ease-in-out infinite;
}

.wave-path-2 {
  fill: rgba(0, 0, 0, 0.04);
  animation: wave-sway-2 32s ease-in-out infinite;
}

.wave-path-3 {
  fill: rgba(0, 0, 0, 0.03);
  animation: wave-sway-3 34s ease-in-out infinite;
}

@keyframes wave-move {
  0% {
    transform: translateX(0);
  }
  100% {
    transform: translateX(-50%);
  }
}

@keyframes wave-sway-1 {
  0%, 100% {
    d: path("M0,192L48,197.3C96,203,192,213,288,229.3C384,245,480,267,576,250.7C672,235,768,181,864,181.3C960,181,1056,235,1152,234.7C1248,235,1344,181,1392,154.7L1440,128L1440,320L1392,320C1344,320,1248,320,1152,320C1056,320,960,320,864,320C768,320,672,320,576,320C480,320,384,320,288,320C192,320,96,320,48,320L0,320Z");
  }
  50% {
    d: path("M0,224L48,213.3C96,203,192,181,288,186.7C384,192,480,224,576,234.7C672,245,768,235,864,213.3C960,192,1056,160,1152,165.3C1248,171,1344,213,1392,234.7L1440,256L1440,320L1392,320C1344,320,1248,320,1152,320C1056,320,960,320,864,320C768,320,672,320,576,320C480,320,384,320,288,320C192,320,96,320,48,320L0,320Z");
  }
}

@keyframes wave-sway-2 {
  0%, 100% {
    d: path("M0,256L48,240C96,224,192,192,288,181.3C384,171,480,181,576,197.3C672,213,768,235,864,224C960,213,1056,171,1152,165.3C1248,160,1344,192,1392,208L1440,224L1440,320L1392,320C1344,320,1248,320,1152,320C1056,320,960,320,864,320C768,320,672,320,576,320C480,320,384,320,288,320C192,320,96,320,48,320L0,320Z");
  }
  50% {
    d: path("M0,224L48,229.3C96,235,192,245,288,240C384,235,480,213,576,208C672,203,768,213,864,229.3C960,245,1056,267,1152,261.3C1248,256,1344,224,1392,208L1440,192L1440,320L1392,320C1344,320,1248,320,1152,320C1056,320,960,320,864,320C768,320,672,320,576,320C480,320,384,320,288,320C192,320,96,320,48,320L0,320Z");
  }
}

@keyframes wave-sway-3 {
  0%, 100% {
    d: path("M0,288L48,272C96,256,192,224,288,213.3C384,203,480,213,576,229.3C672,245,768,267,864,261.3C960,256,1056,224,1152,213.3C1248,203,1344,213,1392,218.7L1440,224L1440,320L1392,320C1344,320,1248,320,1152,320C1056,320,960,320,864,320C768,320,672,320,576,320C480,320,384,320,288,320C192,320,96,320,48,320L0,320Z");
  }
  50% {
    d: path("M0,256L48,261.3C96,267,192,277,288,272C384,267,480,245,576,240C672,235,768,245,864,256C960,267,1056,277,1152,272C1248,267,1344,245,1392,234.7L1440,224L1440,320L1392,320C1344,320,1248,320,1152,320C1056,320,960,320,864,320C768,320,672,320,576,320C480,320,384,320,288,320C192,320,96,320,48,320L0,320Z");
  }
}

/* 内容层 */
.cta-content {
  position: relative;
  z-index: 2;
}

.cta-wrapper {
  text-align: center;
  width: 100%;
}

.cta-title {
  font-size: 42px;
  font-weight: 700;
  margin: 0 0 24px;
  color: #1d1d1f;
}

.cta-subtitle {
  font-size: 18px;
  color: #86868b;
  margin: 0 0 48px;
  line-height: 1.6;
}

.cta-btn {
  padding: 16px 36px;
  background: #000;
  color: #fff;
  border: none;
  border-radius: 12px;
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.cta-btn:hover {
  background: #333;
  transform: translateY(-2px);
}

/* 页脚 */
.footer {
  background: transparent;
  padding: 32px 0;
  flex-shrink: 0;
  position: relative;
  z-index: 1;
}

.footer-content {
  max-width: 60%;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 24px;
}

.footer-links {
  display: flex;
  align-items: center;
  gap: 20px;
}

.footer-link {
  font-size: 14px;
  color: #666;
  text-decoration: none;
  transition: color 0.2s ease;
}

.footer-link:hover {
  color: #1d1d1f;
}

.footer-divider {
  color: #ddd;
  font-size: 14px;
}

.footer-copyright {
  font-size: 13px;
  color: #999;
}

/* 公告弹窗 */
:deep(.notification-modal.el-dialog) {
  border-radius: 16px !important;
  overflow: hidden;
}

.notification-content {
  max-height: 400px;
  overflow-y: auto;
}

.no-notification {
  text-align: center;
  color: #86868b;
  padding: 40px 0;
}

.notification-item {
  padding: 16px 0;
  border-bottom: 1px solid rgba(0, 0, 0, 0.1);
}

.notification-item:last-child {
  border-bottom: none;
}

.notification-item h4 {
  font-size: 16px;
  font-weight: 600;
  color: #1d1d1f;
  margin: 0 0 8px;
}

.notification-item p {
  font-size: 14px;
  color: #86868b;
  margin: 0 0 8px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.notification-time {
  font-size: 12px;
  color: #aeaeb2;
}

/* 响应式 */
@media (max-width: 1024px) {
  .navbar-content,
  .section-content,
  .hero-section,
  .footer-content {
    max-width: 85%;
  }

  .hero-section {
    grid-template-columns: 1fr;
    text-align: center;
    padding: 60px 0 80px;
  }

  .hero-actions {
    justify-content: center;
  }

  .hero-visual {
    order: -1;
  }

  .features-grid {
    grid-template-columns: 1fr;
  }

  .features-title {
    font-size: 28px;
  }

  .compat-wrapper {
    grid-template-columns: 1fr;
    text-align: center;
  }

  .compat-list li {
    justify-content: center;
  }
}

@media (max-width: 768px) {
  .navbar-content {
    padding: 12px 20px;
  }

  .navbar-title {
    display: none;
  }

  .nav-btn {
    padding: 8px 16px;
    font-size: 13px;
  }

  .hero-title {
    font-size: 36px;
  }

  .hero-subtitle {
    font-size: 16px;
  }

  .feature-card {
    padding: 32px 24px;
  }

  .compat-title,
  .cta-title {
    font-size: 32px;
  }

  .footer-content {
    flex-direction: column;
    gap: 24px;
    text-align: center;
  }
}

/* 返回顶部按钮 */
.back-to-top {
  position: fixed;
  bottom: 40px;
  right: 40px;
  width: 48px;
  height: 48px;
  background: #fff;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  z-index: 99;
  transition: all 0.3s ease;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.back-to-top:hover {
  background: #000;
  color: #fff;
  border-color: #000;
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.15);
}

.back-to-top .el-icon {
  font-size: 20px;
}

/* 淡入淡出动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>

<style>
/* 暗黑模式全局样式 */
:root.dark-mode .home-page {
  background: #16161A;
}

:root.dark-mode .navbar {
  background: rgba(22, 22, 26, 0.8);
}

:root.dark-mode .navbar-scrolled {
  background: rgba(22, 22, 26, 0.95);
  border-bottom-color: rgba(255, 255, 255, 0.1);
}

:root.dark-mode .navbar-title {
  color: #f5f5f7;
}

:root.dark-mode .navbar-logo {
  filter: invert(1) hue-rotate(180deg);
}

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
  border-color: #fff;
}

:root.dark-mode .nav-btn-primary {
  border-color: #fff;
}

:root.dark-mode .nav-btn-primary:hover {
  background: #fff;
  color: #000;
}

:root.dark-mode .hero-section-wrapper,
:root.dark-mode .features-section-wrapper,
:root.dark-mode .compatibility-section-wrapper,
:root.dark-mode .cta-section-wrapper {
  background: #16161A;
}

:root.dark-mode .hero-title {
  color: #f5f5f7;
}

:root.dark-mode .hero-title-highlight {
  background: linear-gradient(90deg, #fff 0%, #999 100%);
  -webkit-background-clip: text;
  background-clip: text;
}

:root.dark-mode .hero-subtitle {
  color: #86868b;
}

:root.dark-mode .hero-btn {
  border-color: #fff;
  color: #f5f5f7;
}

:root.dark-mode .hero-btn:hover {
  background: #fff;
  color: #000;
}

:root.dark-mode .hero-btn-primary {
  background: #fff;
  color: #000;
}

:root.dark-mode .hero-btn-primary:hover {
  background: #e5e5e5;
  border-color: #e5e5e5;
}

:root.dark-mode .features-label {
  color: #f5f5f7;
}

:root.dark-mode .feature-card {
  background: #1e1e22;
  border-color: rgba(255, 255, 255, 0.08);
}

:root.dark-mode .feature-card:hover {
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.4);
}

:root.dark-mode .feature-title {
  color: #f5f5f7;
}

:root.dark-mode .feature-desc {
  color: #86868b;
}

:root.dark-mode .compat-label-title {
  color: #f5f5f7;
}

:root.dark-mode .compat-list li {
  color: #f5f5f7;
  border-bottom-color: rgba(255, 255, 255, 0.08);
}

:root.dark-mode .cta-title {
  color: #f5f5f7;
}

:root.dark-mode .cta-btn {
  background: #fff;
  color: #000;
}

:root.dark-mode .cta-btn:hover {
  background: #e5e5e5;
}

:root.dark-mode .wave-path-1 {
  fill: rgba(255, 255, 255, 0.06);
}

:root.dark-mode .wave-path-2 {
  fill: rgba(255, 255, 255, 0.04);
}

:root.dark-mode .wave-path-3 {
  fill: rgba(255, 255, 255, 0.03);
}

:root.dark-mode .footer {
  background: transparent;
}

:root.dark-mode .footer-link {
  color: #86868b;
}

:root.dark-mode .footer-link:hover {
  color: #f5f5f7;
}

:root.dark-mode .footer-divider {
  color: #333;
}

:root.dark-mode .back-to-top {
  background: #1a1a1a;
  border-color: rgba(255, 255, 255, 0.1);
  color: #f5f5f7;
}

:root.dark-mode .back-to-top:hover {
  background: #fff;
  color: #000;
  border-color: #fff;
}
</style>
