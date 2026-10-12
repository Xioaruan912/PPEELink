import { gsap } from "gsap";

const reduceMotion = () =>
  window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;

function format(value: number, decimals: number, separator: boolean) {
  const fixed = value.toFixed(decimals);
  if (!separator) return fixed;
  const [int, frac] = fixed.split(".");
  const pretty = int.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  return frac ? `${pretty}.${frac}` : pretty;
}

export function useCountUp(
  source: () => number,
  options: {
    duration?: number;
    decimals?: number;
    separator?: boolean;
  } = {}
) {
  const { duration = 1.2, decimals = 0, separator = true } = options;
  const display = ref(format(Number(source()) || 0, decimals, separator));
  let tween: gsap.core.Tween | null = null;
  let started = false;

  const run = () => {
    started = true;
    const target = Number(source()) || 0;
    if (reduceMotion()) {
      display.value = format(target, decimals, separator);
      return;
    }
    const proxy = { v: Number(display.value.replace(/,/g, "")) || 0 };
    tween?.kill();
    tween = gsap.to(proxy, {
      v: target,
      duration,
      ease: "power3.out",
      onUpdate: () => {
        display.value = format(proxy.v, decimals, separator);
      },
      onComplete: () => {
        display.value = format(target, decimals, separator);
      },
    });
  };

  onBeforeUnmount(() => tween?.kill());

  return {
    display,
    started: () => started,
    /** call once the element is near the viewport */
    start: run,
    refresh: () => {
      if (started) run();
    },
  };
}

/** start a count-up when the host element approaches the viewport */
export function useCountUpInView(host: Ref<HTMLElement | undefined>) {
  const startRef = ref<(() => void) | null>(null);
  let observer: IntersectionObserver | null = null;

  onMounted(() => {
    const el = host.value;
    if (!el) return;
    if (reduceMotion()) return;
    observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          startRef.value?.();
          observer?.disconnect();
        }
      },
      { threshold: 0.2 }
    );
    observer.observe(el);
  });

  onBeforeUnmount(() => observer?.disconnect());

  return {
    register: (starter: () => void) => {
      startRef.value = starter;
    },
  };
}
