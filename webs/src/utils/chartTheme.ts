import { onBeforeUnmount, onMounted, shallowRef } from "vue";

/** Resolve CSS colors through the browser, including color-mix and aliases. */
function readChartTheme() {
  const probe = document.createElement("span");
  probe.style.display = "none";
  document.documentElement.append(probe);
  const color = (token: string) => {
    probe.style.color = `var(${token})`;
    return getComputedStyle(probe).color;
  };
  const theme = {
    ink: color("--c-ink"),
    muted: color("--c-muted"),
    surface: color("--c-surface"),
    line: color("--c-line"),
    data: color("--c-data"),
    ok: color("--c-ok"),
    warn: color("--c-warn-ink"),
    bad: color("--c-bad"),
    grid: "",
    font: getComputedStyle(document.documentElement)
      .getPropertyValue("--font-ui")
      .trim(),
    mono: getComputedStyle(document.documentElement)
      .getPropertyValue("--font-mono")
      .trim(),
  };
  probe.style.color = "color-mix(in srgb, var(--c-line) 35%, transparent)";
  theme.grid = getComputedStyle(probe).color;
  probe.remove();
  return theme;
}

/** Reactive chart palette; every consumer's watcher can repaint its chart. */
export function useChartTheme() {
  const theme = shallowRef(readChartTheme());
  let observer: MutationObserver | undefined;
  onMounted(() => {
    observer = new MutationObserver(() => {
      theme.value = readChartTheme();
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class", "style", "data-theme"],
    });
  });
  onBeforeUnmount(() => observer?.disconnect());
  return theme;
}
