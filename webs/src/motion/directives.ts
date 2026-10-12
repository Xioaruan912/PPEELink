import type { Directive, DirectiveBinding } from "vue";

const prefersReduced = () =>
  typeof window !== "undefined" &&
  window.matchMedia &&
  window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/* ------------------------------------------------------------------ */
/* v-reveal — viewport entrance reveal (IntersectionObserver)          */
/* usage: v-reveal or v-reveal="120" (delay ms)                        */
/* ------------------------------------------------------------------ */

let revealObserver: IntersectionObserver | null = null;

function ensureRevealObserver() {
  if (revealObserver) return revealObserver;
  revealObserver = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          entry.target.classList.add("reveal-in");
          revealObserver?.unobserve(entry.target);
        }
      }
    },
    { rootMargin: "0px 0px -8% 0px", threshold: 0.08 }
  );
  return revealObserver;
}

export const vReveal: Directive<HTMLElement, number | undefined> = {
  mounted(el, binding) {
    el.classList.add("reveal");
    if (typeof binding.value === "number" && binding.value > 0) {
      el.style.setProperty("--reveal-delay", `${binding.value}ms`);
    }
    if (prefersReduced()) {
      el.classList.add("reveal-in");
      return;
    }
    ensureRevealObserver().observe(el);
  },
  unmounted(el) {
    revealObserver?.unobserve(el);
  },
};

/* ------------------------------------------------------------------ */
/* v-magnetic — subtle pointer physics for CTAs                        */
/* usage: v-magnetic or v-magnetic="0.22" (strength)                   */
/* ------------------------------------------------------------------ */

export const vMagnetic: Directive<HTMLElement, number | undefined> = {
  mounted(el, binding) {
    if (prefersReduced() || !window.matchMedia("(pointer: fine)").matches) {
      return;
    }
    const strength =
      typeof binding.value === "number" ? binding.value : 0.18;
    let raf = 0;
    let dx = 0;
    let dy = 0;

    const apply = () => {
      raf = 0;
      el.style.transform = `translate3d(${dx}px, ${dy}px, 0)`;
    };

    const onMove = (event: PointerEvent) => {
      const rect = el.getBoundingClientRect();
      dx = (event.clientX - (rect.left + rect.width / 2)) * strength;
      dy = (event.clientY - (rect.top + rect.height / 2)) * strength;
      el.style.transition = "transform 120ms ease-out";
      if (!raf) raf = requestAnimationFrame(apply);
    };

    const onLeave = () => {
      dx = 0;
      dy = 0;
      el.style.transition =
        "transform 420ms cubic-bezier(0.22, 1, 0.36, 1)";
      if (!raf) raf = requestAnimationFrame(apply);
    };

    el.style.willChange = "transform";
    el.addEventListener("pointermove", onMove);
    el.addEventListener("pointerleave", onLeave);
    (el as any).__magneticCleanup = () => {
      el.removeEventListener("pointermove", onMove);
      el.removeEventListener("pointerleave", onLeave);
    };
  },
  unmounted(el) {
    (el as any).__magneticCleanup?.();
  },
};

/* ------------------------------------------------------------------ */
/* v-tilt — 3D hover depth for cards                                   */
/* usage: v-tilt or v-tilt="4" (max degrees)                           */
/* ------------------------------------------------------------------ */

export const vTilt: Directive<HTMLElement, number | undefined> = {
  mounted(el, binding) {
    if (prefersReduced() || !window.matchMedia("(pointer: fine)").matches) {
      return;
    }
    const max = typeof binding.value === "number" ? binding.value : 3.5;
    let raf = 0;
    let rx = 0;
    let ry = 0;

    const apply = () => {
      raf = 0;
      el.style.transform = `perspective(900px) rotateX(${rx}deg) rotateY(${ry}deg) translateY(-2px)`;
    };

    const onMove = (event: PointerEvent) => {
      const rect = el.getBoundingClientRect();
      const px = (event.clientX - rect.left) / rect.width - 0.5;
      const py = (event.clientY - rect.top) / rect.height - 0.5;
      ry = px * max * 2;
      rx = -py * max * 2;
      el.style.transition = "transform 140ms ease-out";
      if (!raf) raf = requestAnimationFrame(apply);
    };

    const onLeave = () => {
      rx = 0;
      ry = 0;
      el.style.transition =
        "transform 520ms cubic-bezier(0.22, 1, 0.36, 1)";
      if (!raf) raf = requestAnimationFrame(apply);
    };

    el.style.willChange = "transform";
    el.addEventListener("pointermove", onMove);
    el.addEventListener("pointerleave", onLeave);
    (el as any).__tiltCleanup = () => {
      el.removeEventListener("pointermove", onMove);
      el.removeEventListener("pointerleave", onLeave);
    };
  },
  unmounted(el) {
    (el as any).__tiltCleanup?.();
  },
};

/** helper for manual use inside components */
export type MotionBinding = DirectiveBinding<number | undefined>;
