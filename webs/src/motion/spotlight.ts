/** Jade cursor spotlight — subtle, pointer-fine only, reduced-motion aware. */
export function setupSpotlight() {
  if (typeof window === "undefined") return;
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  if (!window.matchMedia("(pointer: fine)").matches) return;

  const el = document.createElement("div");
  el.className = "cursor-spotlight";
  el.setAttribute("aria-hidden", "true");
  document.body.appendChild(el);

  let raf = 0;
  let x = window.innerWidth / 2;
  let y = window.innerHeight / 3;
  let active = false;

  const apply = () => {
    raf = 0;
    el.style.transform = `translate3d(${x}px, ${y}px, 0)`;
  };

  window.addEventListener(
    "pointermove",
    (event) => {
      x = event.clientX;
      y = event.clientY;
      if (!active) {
        active = true;
        el.classList.add("is-on");
      }
      if (!raf) raf = requestAnimationFrame(apply);
    },
    { passive: true }
  );

  window.addEventListener("pointerleave", () => {
    el.classList.remove("is-on");
  });
}
