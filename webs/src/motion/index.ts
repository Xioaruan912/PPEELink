import {
  computed,
  onBeforeUnmount,
  onMounted,
  ref,
  toValue,
  watch,
  type MaybeRefOrGetter,
  type ObjectDirective,
} from "vue";
import gsap from "gsap";

export const prefersReducedMotion = () =>
  window.matchMedia("(prefers-reduced-motion: reduce)").matches ||
  document.documentElement.dataset.motion === "reduced";

export function usePrefersReducedMotion() {
  const reduced = ref(prefersReducedMotion());
  const media = window.matchMedia("(prefers-reduced-motion: reduce)");
  const update = () => {
    reduced.value = prefersReducedMotion();
  };
  onMounted(() => {
    media.addEventListener("change", update);
    window.addEventListener("workspace-motion-change", update);
  });
  onBeforeUnmount(() => {
    media.removeEventListener("change", update);
    window.removeEventListener("workspace-motion-change", update);
  });
  return reduced;
}

export function useMotion() {
  const reduced = usePrefersReducedMotion();
  const owned = new Set<gsap.core.Animation>();
  const stop = () => {
    owned.forEach((tween) => tween.progress(1).kill());
    owned.clear();
  };
  watch(reduced, (value) => {
    if (value) stop();
  });
  onBeforeUnmount(stop);
  function animate(
    target: gsap.TweenTarget,
    from: gsap.TweenVars,
    to: gsap.TweenVars
  ) {
    if (reduced.value) {
      gsap.set(target, { ...to, duration: 0, delay: 0, stagger: 0 });
      return;
    }
    const onComplete = to.onComplete;
    const onInterrupt = to.onInterrupt;
    const tween = gsap.fromTo(target, from, {
      ...to,
      onComplete: () => {
        if (tween) owned.delete(tween);
        onComplete?.();
      },
      onInterrupt: () => {
        if (tween) owned.delete(tween);
        onInterrupt?.();
      },
    });
    owned.add(tween);
    return tween;
  }
  return { reduced, animate, stop };
}

export function useRouteTransition() {
  const { reduced, animate } = useMotion();
  const pending = new Map<Element, () => void>();
  function cancel(element: Element) {
    pending.get(element)?.();
  }
  function run(
    element: Element,
    targets: HTMLElement[],
    from: gsap.TweenVars,
    to: gsap.TweenVars,
    limit: number,
    done: () => void
  ) {
    cancel(element);
    let finished = false;
    let tween: gsap.core.Tween | undefined;
    const finish = () => {
      if (finished) return;
      finished = true;
      clearTimeout(timer);
      pending.delete(element);
      tween?.kill();
      // Restore visibility without depending on GSAP, including failed starts.
      for (const target of targets) {
        target.style.removeProperty("opacity");
        target.style.removeProperty("transform");
        target.style.removeProperty("clip-path");
      }
      done();
    };
    // Vue's out-in transition must finish even if the animation is interrupted.
    const timer = setTimeout(finish, limit);
    pending.set(element, finish);
    if (reduced.value) {
      finish();
      return;
    }
    try {
      tween = animate(targets, from, {
        ...to,
        onComplete: finish,
        onInterrupt: finish,
      });
    } catch {
      finish();
    }
  }
  onBeforeUnmount(() => {
    pending.forEach((finish) => finish());
  });
  function enter(element: Element, done: () => void) {
    const root = element as HTMLElement;
    const parts = Array.from(
      root.querySelectorAll<HTMLElement>(
        "[data-enter], .page-header, .el-table__row"
      )
    );
    const children = Array.from(root.children).filter(
      (child) => child instanceof HTMLElement
    );
    const targets = parts.length
      ? parts.slice(0, 18)
      : children.length
        ? children.slice(0, 18)
        : [root];
    run(
      element,
      targets,
      { opacity: 0, y: 20, clipPath: "inset(0 0 12% 0)" },
      {
        opacity: 1,
        y: 0,
        clipPath: "inset(0 0 0% 0)",
        duration: 0.32,
        stagger: Math.min(0.04, 0.24 / Math.max(1, targets.length - 1)),
        ease: "power3.out",
        clearProps: "transform,opacity,clipPath",
      },
      560,
      done
    );
  }
  function leave(element: Element, done: () => void) {
    run(
      element,
      [element as HTMLElement],
      { opacity: 1, y: 0 },
      {
        opacity: 0,
        y: -6,
        duration: 0.14,
      },
      140,
      done
    );
  }
  return { enter, leave, cancel };
}

export function useCountUp(value: MaybeRefOrGetter<number>, decimals = 0) {
  const current = ref(0);
  const { animate, stop } = useMotion();
  watch(
    () => toValue(value),
    (target) => {
      stop();
      const next = Number.isFinite(target) ? target : 0;
      animate(
        current,
        { value: current.value },
        { value: next, duration: 0.56, ease: "power3.out" }
      );
    },
    { immediate: true }
  );
  return computed(() =>
    current.value.toLocaleString("zh-CN", {
      minimumFractionDigits: decimals,
      maximumFractionDigits: decimals,
    })
  );
}

const magneticCleanups = new WeakMap<HTMLElement, () => void>();
export const vMagnetic: ObjectDirective<HTMLElement, boolean | undefined> = {
  mounted(element, binding) {
    if (binding.value === false) return;
    let tween: gsap.core.Tween | undefined;
    const reset = () => {
      tween?.kill();
      tween = gsap.to(element, {
        x: 0,
        y: 0,
        duration: prefersReducedMotion() ? 0 : 0.56,
        ease: "elastic.out(1,.6)",
      });
    };
    const move = (event: PointerEvent) => {
      if (
        prefersReducedMotion() ||
        !matchMedia("(pointer: fine)").matches ||
        element.matches(":disabled")
      )
        return;
      const bounds = element.getBoundingClientRect();
      const dx = event.clientX - (bounds.left + bounds.width / 2);
      const dy = event.clientY - (bounds.top + bounds.height / 2);
      if (
        event.clientX < bounds.left - 24 ||
        event.clientX > bounds.right + 24 ||
        event.clientY < bounds.top - 24 ||
        event.clientY > bounds.bottom + 24
      ) {
        reset();
        return;
      }
      tween?.kill();
      tween = gsap.to(element, {
        x: Math.max(-8, Math.min(8, dx * 0.08)),
        y: Math.max(-6, Math.min(6, dy * 0.12)),
        duration: 0.18,
        ease: "power3.out",
      });
    };
    const media = matchMedia("(prefers-reduced-motion: reduce)");
    const parent = element.parentElement || element;
    parent.addEventListener("pointermove", move);
    parent.addEventListener("pointerleave", reset);
    media.addEventListener("change", reset);
    window.addEventListener("workspace-motion-change", reset);
    magneticCleanups.set(element, () => {
      parent.removeEventListener("pointermove", move);
      parent.removeEventListener("pointerleave", reset);
      media.removeEventListener("change", reset);
      window.removeEventListener("workspace-motion-change", reset);
      tween?.kill();
      gsap.set(element, { clearProps: "transform" });
    });
  },
  beforeUnmount(element) {
    magneticCleanups.get(element)?.();
    magneticCleanups.delete(element);
  },
};

/** FLIP snapshots use stable keys; invoke after Vue has patched the list. */
export function useFlip() {
  const { animate } = useMotion();
  let positions = new Map<string, DOMRect>();
  const capture = (elements: HTMLElement[]) => {
    positions = new Map(
      elements.map((el) => [
        el.dataset.flipKey || el.id,
        el.getBoundingClientRect(),
      ])
    );
  };
  const play = (elements: HTMLElement[]) => {
    elements.forEach((el) => {
      const previous = positions.get(el.dataset.flipKey || el.id);
      if (!previous) return;
      const next = el.getBoundingClientRect();
      animate(
        el,
        { x: previous.left - next.left, y: previous.top - next.top },
        {
          x: 0,
          y: 0,
          duration: 0.56,
          ease: "back.out(1.6)",
          clearProps: "transform",
        }
      );
    });
    positions.clear();
  };
  return { capture, play };
}

/** Owns first-screen list stagger and cancels previous results before animating. */
export function useStagger() {
  const { animate, stop } = useMotion();
  return (
    elements: string | HTMLElement[] | NodeListOf<Element>,
    delay = 0.05,
    stagger = 0.04
  ) => {
    stop();
    const targets =
      typeof elements === "string"
        ? Array.from(document.querySelectorAll(elements))
        : Array.from(elements);
    if (!targets.length) return;
    const visibleTargets = targets.slice(0, 18);
    const boundedDelay = Math.min(0.08, Math.max(0, delay));
    return animate(
      visibleTargets,
      { opacity: 0, y: 16 },
      {
        opacity: 1,
        y: 0,
        duration: 0.32,
        stagger: Math.min(
          Math.max(0, stagger),
          (0.24 - boundedDelay) / Math.max(1, visibleTargets.length - 1)
        ),
        delay: boundedDelay,
        ease: "power3.out",
        clearProps: "opacity,transform",
      }
    );
  };
}
