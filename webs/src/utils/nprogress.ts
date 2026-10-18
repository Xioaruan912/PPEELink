import NProgress from "nprogress";
import "nprogress/nprogress.css";
import { prefersReducedMotion } from "@/motion";

// 进度条
NProgress.configure({
  // NProgress still queries [role="bar"] internally; select the class instead.
  barSelector: ".bar",
  template: '<div class="bar" aria-hidden="true"><div class="peg"></div></div>',
  // 动画方式
  easing: "ease",
  // 递增进度条的速度
  speed: 320,
  // 是否显示加载ico
  showSpinner: false,
  // 自动递增间隔
  trickleSpeed: 200,
  // 初始化时的最小百分比
  minimum: 0.3,
});

/** Re-evaluate the shared motion preference on every navigation. */
export function startProgress() {
  const reduced = prefersReducedMotion();
  NProgress.configure({ speed: reduced ? 0 : 320, trickle: !reduced });
  NProgress.start();
}

export default NProgress;
