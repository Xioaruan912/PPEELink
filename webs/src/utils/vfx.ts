import gsap from "gsap"

/**
 * 现代化视觉特效与 GSAP 动效引擎 (Taste-Skill / Elite VFX)
 */

/**
 * 鼠标聚光灯追踪特效 (Mouse Spotlight Glow)
 * 在卡片容器上自动监听 pointermove，设置 --mouse-x 与 --mouse-y CSS 变量
 */
export function initSpotlight(container: HTMLElement | null) {
  if (!container) return () => {}

  const onPointerMove = (e: PointerEvent) => {
    const cards = container.querySelectorAll<HTMLElement>(".spotlight-card, .client-card, .sub-card, .node-card, .kpi-article")
    cards.forEach((card) => {
      const rect = card.getBoundingClientRect()
      const x = e.clientX - rect.left
      const y = e.clientY - rect.top
      card.style.setProperty("--mouse-x", `${x}px`)
      card.style.setProperty("--mouse-y", `${y}px`)
    })
  }

  container.addEventListener("pointermove", onPointerMove, { passive: true })
  return () => {
    container.removeEventListener("pointermove", onPointerMove)
  }
}

/**
 * GSAP 平滑数字渐变增长动画
 */
export function animateCounter(
  getter: () => number,
  setter: (val: number) => void,
  target: number,
  duration = 1.2,
  decimals = 0
) {
  const prefersReduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches || document.documentElement.dataset.motion === 'reduced'
  if (prefersReduced) {
    setter(target)
    return
  }

  const obj = { val: getter() }
  gsap.to(obj, {
    val: target,
    duration,
    ease: "power3.out",
    onUpdate: () => {
      if (decimals === 0) {
        setter(Math.round(obj.val))
      } else {
        const factor = Math.pow(10, decimals)
        setter(Math.round(obj.val * factor) / factor)
      }
    }
  })
}

/**
 * GSAP 元素阶梯淡入动画 (Stagger Reveal)
 */
export function staggerReveal(elements: string | HTMLElement[] | NodeListOf<Element>, delay = 0.05, stagger = 0.08) {
  const prefersReduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches || document.documentElement.dataset.motion === 'reduced'
  if (prefersReduced) return

  gsap.fromTo(
    elements,
    { opacity: 0, y: 18, scale: 0.98 },
    {
      opacity: 1,
      y: 0,
      scale: 1,
      duration: 0.7,
      stagger,
      delay,
      ease: "power3.out",
      clearProps: "transform,opacity"
    }
  )
}
