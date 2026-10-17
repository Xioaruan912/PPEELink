<template>
  <div class="cyber-canvas-container" aria-hidden="true">
    <canvas ref="canvasRef" class="cyber-canvas"></canvas>
    <div class="vignette-overlay"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'

const canvasRef = ref<HTMLCanvasElement | null>(null)
let animId = 0

interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  size: number
  baseAlpha: number
}

onMounted(() => {
  const canvas = canvasRef.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (!ctx) return

  let width = (canvas.width = window.innerWidth)
  let height = (canvas.height = window.innerHeight)

  const handleResize = () => {
    if (!canvas) return
    width = canvas.width = window.innerWidth
    height = canvas.height = window.innerHeight
  }
  window.addEventListener('resize', handleResize)

  // 粒子池
  const count = Math.min(Math.floor((width * height) / 18000), 75)
  const particles: Particle[] = []
  for (let i = 0; i < count; i++) {
    particles.push({
      x: Math.random() * width,
      y: Math.random() * height,
      vx: (Math.random() - 0.5) * 0.45,
      vy: (Math.random() - 0.5) * 0.45,
      size: Math.random() * 1.6 + 0.8,
      baseAlpha: Math.random() * 0.4 + 0.2
    })
  }

  let mouseX = -9999
  let mouseY = -9999
  const handleMouseMove = (e: MouseEvent) => {
    mouseX = e.clientX
    mouseY = e.clientY
  }
  window.addEventListener('mousemove', handleMouseMove, { passive: true })

  const render = () => {
    ctx.clearRect(0, 0, width, height)

    // 绘制粒子与连线
    for (let i = 0; i < particles.length; i++) {
      const p = particles[i]
      p.x += p.vx
      p.y += p.vy

      if (p.x < 0) p.x = width
      else if (p.x > width) p.x = 0
      if (p.y < 0) p.y = height
      else if (p.y > height) p.y = 0

      // 鼠标斥力微交互
      const dx = mouseX - p.x
      const dy = mouseY - p.y
      const dist = Math.sqrt(dx * dx + dy * dy)
      if (dist < 120) {
        p.x -= (dx / dist) * 0.8
        p.y -= (dy / dist) * 0.8
      }

      // 绘制粒子光点
      ctx.beginPath()
      ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2)
      ctx.fillStyle = `rgba(56, 189, 248, ${p.baseAlpha})`
      ctx.fill()

      // 邻近连线
      for (let j = i + 1; j < particles.length; j++) {
        const p2 = particles[j]
        const dX = p.x - p2.x
        const dY = p.y - p2.y
        const d = Math.sqrt(dX * dX + dY * dY)
        if (d < 130) {
          ctx.beginPath()
          ctx.moveTo(p.x, p.y)
          ctx.lineTo(p2.x, p2.y)
          const lineAlpha = (1 - d / 130) * 0.22
          ctx.strokeStyle = `rgba(56, 189, 248, ${lineAlpha})`
          ctx.lineWidth = 0.8
          ctx.stroke()
        }
      }
    }

    animId = requestAnimationFrame(render)
  }

  // 尊重减弱动效
  if (!window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    animId = requestAnimationFrame(render)
  }

  onBeforeUnmount(() => {
    cancelAnimationFrame(animId)
    window.removeEventListener('resize', handleResize)
    window.removeEventListener('mousemove', handleMouseMove)
  })
})
</script>

<style scoped>
.cyber-canvas-container {
  position: fixed;
  inset: 0;
  pointer-events: none;
  z-index: 0;
  overflow: hidden;
}

.cyber-canvas {
  width: 100%;
  height: 100%;
  opacity: 0.45;
}

.vignette-overlay {
  position: absolute;
  inset: 0;
  background: radial-gradient(
    ellipse at 50% 15%,
    transparent 20%,
    rgba(9, 13, 22, 0.4) 60%,
    rgba(9, 13, 22, 0.85) 100%
  );
}
</style>
