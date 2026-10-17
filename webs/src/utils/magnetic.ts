import type { ObjectDirective } from "vue";
import gsap from "gsap";

// Pointer physics stay outside Vue's render cycle. Every listener and tween is
// owned by the element and disposed when the view leaves the page.
const cleanups = new WeakMap<HTMLElement, () => void>();
export const vMagnetic: ObjectDirective<HTMLElement> = {
  mounted(element) {
    const pointer = matchMedia("(pointer: fine)");
    const reduced = matchMedia("(prefers-reduced-motion: reduce)");
    const x = gsap.quickTo(element, "x", {
      duration: 0.55,
      ease: "power3.out",
    });
    const y = gsap.quickTo(element, "y", {
      duration: 0.55,
      ease: "power3.out",
    });
    const reset = () => {
      x(0);
      y(0);
    };
    const move = (event: PointerEvent) => {
      if (
        !pointer.matches ||
        reduced.matches ||
        document.documentElement.dataset.motion === "reduced"
      )
        return;
      const bounds = element.getBoundingClientRect();
      x(
        Math.max(
          -5,
          Math.min(5, (event.clientX - bounds.left - bounds.width / 2) * 0.08)
        )
      );
      y(
        Math.max(
          -3,
          Math.min(3, (event.clientY - bounds.top - bounds.height / 2) * 0.08)
        )
      );
    };
    element.addEventListener("pointermove", move, { passive: true });
    element.addEventListener("pointerleave", reset);
    window.addEventListener("workspace-motion-change", reset);
    cleanups.set(element, () => {
      element.removeEventListener("pointermove", move);
      element.removeEventListener("pointerleave", reset);
      window.removeEventListener("workspace-motion-change", reset);
      x.tween.kill();
      y.tween.kill();
    });
  },
  unmounted(element) {
    cleanups.get(element)?.();
    cleanups.delete(element);
  },
};
