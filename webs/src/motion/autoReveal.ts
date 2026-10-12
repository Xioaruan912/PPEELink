import type { Router } from "vue-router";

const REVEAL_SELECTOR = [
  "[data-autoreveal]",
  ".el-card",
  ".search-container",
  ".dashboard-panel",
  ".dashboard-hero",
  ".kpi-grid > article",
  ".scene-card",
  ".metric-tile",
].join(",");

/**
 * Route-level choreography: stagger the main content in on every navigation.
 * Pure Web Animations API on transform/opacity — no layout thrash, no scroll listeners.
 */
export function setupAutoReveal(router: Router) {
  if (typeof window === "undefined") return;
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

  let runId = 0;

  router.afterEach(() => {
    const id = ++runId;
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        if (id !== runId) return;
        const root = document.querySelector(".app-main");
        if (!root) return;

        const all = Array.from(
          root.querySelectorAll<HTMLElement>(REVEAL_SELECTOR)
        );
        const picked: HTMLElement[] = [];
        for (const node of all) {
          if (picked.some((parent) => parent.contains(node))) continue;
          const rect = node.getBoundingClientRect();
          if (rect.width < 48 || rect.height < 28) continue;
          picked.push(node);
        }

        picked.slice(0, 14).forEach((node, index) => {
          node.animate(
            [
              { opacity: 0, transform: "translateY(18px)", filter: "blur(6px)" },
              { opacity: 1, transform: "none", filter: "none" },
            ],
            {
              duration: 640,
              delay: index * 55,
              easing: "cubic-bezier(0.22, 1, 0.36, 1)",
              fill: "backwards",
            }
          );
        });

        // second pass: table rows land after their async data resolves
        window.setTimeout(() => {
          if (id !== runId) return;
          const rows = root.querySelectorAll<HTMLElement>(
            ".el-table__body tbody tr"
          );
          Array.from(rows)
            .slice(0, 12)
            .forEach((row, index) => {
              row.animate(
                [
                  { opacity: 0, transform: "translateY(8px)" },
                  { opacity: 1, transform: "none" },
                ],
                {
                  duration: 420,
                  delay: index * 26,
                  easing: "cubic-bezier(0.22, 1, 0.36, 1)",
                  fill: "backwards",
                }
              );
            });
        }, 340);
      });
    });
  });
}
