<template>
  <canvas
    ref="canvasRef"
    class="fireworks-canvas"
    :style="{ pointerEvents: 'none' }"
  />
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'

const props = defineProps({
  active: { type: Boolean, default: false }
})

const emit = defineEmits(['ended'])

const canvasRef = ref(null)
let ctx = null
let animationId = null
let fireworks = []
let particles = []
let startTime = 0
const DURATION = 6000 // 6秒动画

// 烟花颜色
const colors = [
  '#ff4757', '#ff6b81', '#ffa502', '#ffdd59',
  '#2ed573', '#7bed9f', '#1e90ff', '#70a1ff',
  '#9b59b6', '#a55eea', '#ff6348', '#ff7f50'
]

// 烟花类
class Firework {
  constructor(canvas) {
    this.x = Math.random() * canvas.width
    this.y = canvas.height
    this.targetY = canvas.height * (0.2 + Math.random() * 0.3)
    this.speed = 8 + Math.random() * 4
    this.color = colors[Math.floor(Math.random() * colors.length)]
    this.trail = []
    this.exploded = false
  }
  
  update() {
    this.trail.push({ x: this.x, y: this.y })
    if (this.trail.length > 8) this.trail.shift()
    this.y -= this.speed
    if (this.y <= this.targetY) this.exploded = true
  }
  
  draw(ctx) {
    for (let i = 0; i < this.trail.length; i++) {
      ctx.globalAlpha = i / this.trail.length * 0.6
      ctx.fillStyle = this.color
      ctx.beginPath()
      ctx.arc(this.trail[i].x, this.trail[i].y, 2, 0, Math.PI * 2)
      ctx.fill()
    }
    ctx.globalAlpha = 1
    ctx.fillStyle = this.color
    ctx.beginPath()
    ctx.arc(this.x, this.y, 3, 0, Math.PI * 2)
    ctx.fill()
  }
}

// 粒子类
class Particle {
  constructor(x, y, color) {
    this.x = x
    this.y = y
    this.color = color
    const angle = Math.random() * Math.PI * 2
    const speed = 2 + Math.random() * 6
    this.vx = Math.cos(angle) * speed
    this.vy = Math.sin(angle) * speed
    this.gravity = 0.12
    this.life = 1
    this.decay = 0.015 + Math.random() * 0.01
  }
  
  update() {
    this.x += this.vx
    this.y += this.vy
    this.vy += this.gravity
    this.vx *= 0.99
    this.life -= this.decay
  }
  
  draw(ctx) {
    ctx.globalAlpha = this.life
    ctx.fillStyle = this.color
    ctx.beginPath()
    ctx.arc(this.x, this.y, 2.5, 0, Math.PI * 2)
    ctx.fill()
  }
}

function createExplosion(x, y, color) {
  const count = 50 + Math.floor(Math.random() * 30)
  for (let i = 0; i < count; i++) {
    particles.push(new Particle(x, y, color))
  }
}

function animate() {
  if (!ctx || !canvasRef.value) return
  
  const elapsed = Date.now() - startTime
  ctx.clearRect(0, 0, canvasRef.value.width, canvasRef.value.height)
  
  // 前5秒持续发射烟花
  if (elapsed < 5000 && Math.random() < 0.08) {
    fireworks.push(new Firework(canvasRef.value))
  }
  
  // 更新烟花
  for (let i = fireworks.length - 1; i >= 0; i--) {
    fireworks[i].update()
    fireworks[i].draw(ctx)
    if (fireworks[i].exploded) {
      createExplosion(fireworks[i].x, fireworks[i].y, fireworks[i].color)
      fireworks.splice(i, 1)
    }
  }
  
  // 更新粒子
  for (let i = particles.length - 1; i >= 0; i--) {
    particles[i].update()
    particles[i].draw(ctx)
    if (particles[i].life <= 0) particles.splice(i, 1)
  }
  
  ctx.globalAlpha = 1
  
  // 检查是否结束
  if (elapsed >= DURATION && fireworks.length === 0 && particles.length === 0) {
    emit('ended')
    return
  }
  
  animationId = requestAnimationFrame(animate)
}

function start() {
  if (!canvasRef.value) return
  const canvas = canvasRef.value
  canvas.width = window.innerWidth
  canvas.height = window.innerHeight
  ctx = canvas.getContext('2d')
  fireworks = []
  particles = []
  startTime = Date.now()
  animate()
}

function stop() {
  if (animationId) {
    cancelAnimationFrame(animationId)
    animationId = null
  }
  fireworks = []
  particles = []
  if (ctx && canvasRef.value) {
    ctx.clearRect(0, 0, canvasRef.value.width, canvasRef.value.height)
  }
}

watch(() => props.active, (val) => {
  if (val) start()
  else stop()
})

onMounted(() => { if (props.active) start() })
onUnmounted(() => stop())
</script>

<style scoped>
.fireworks-canvas {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  z-index: 99999;
}
</style>

