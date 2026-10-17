import { computed, watch } from "vue";
import * as monaco from "monaco-editor/esm/vs/editor/editor.api";
import { useChartTheme } from "@/utils/chartTheme";

/** Monaco accepts hex colors; resolve the existing CSS token palette at runtime. */
export function useTemplateMonacoTheme() {
  const palette = useChartTheme();
  const name = computed(() => {
    // Palette changes whenever html.dark changes.
    void palette.value;
    return document.documentElement.classList.contains("dark")
      ? "signal-atlas-dark"
      : "signal-atlas-light";
  });
  watch(
    palette,
    (colors) => {
      const canvas = document.createElement("canvas");
      canvas.width = canvas.height = 1;
      const context = canvas.getContext("2d")!;
      const hex = (color: string, alpha = 1) => {
        context.clearRect(0, 0, 1, 1);
        context.fillStyle = color;
        context.fillRect(0, 0, 1, 1);
        const channels = Array.from(context.getImageData(0, 0, 1, 1).data);
        if (alpha !== 1) channels[3] = Math.round(255 * alpha);
        return `#${channels.map((value) => value.toString(16).padStart(2, "0")).join("")}`;
      };
      const ink = hex(colors.ink),
        muted = hex(colors.muted),
        surface = hex(colors.surface);
      const theme: Omit<monaco.editor.IStandaloneThemeData, "base"> = {
        inherit: true,
        rules: [
          { token: "", foreground: ink.slice(1, 7) },
          { token: "comment", foreground: muted.slice(1, 7) },
          { token: "string", foreground: hex(colors.ok).slice(1, 7) },
          { token: "number", foreground: hex(colors.data).slice(1, 7) },
          { token: "keyword", foreground: hex(colors.bad).slice(1, 7) },
          { token: "type", foreground: hex(colors.data).slice(1, 7) },
        ],
        colors: {
          "editor.background": surface,
          "editor.foreground": ink,
          "editorGutter.background": surface,
          "editorLineNumber.foreground": muted,
          "editorLineNumber.activeForeground": ink,
          "editorCursor.foreground": hex(colors.bad),
          "editor.selectionBackground": hex(colors.data, 0.18),
          "editor.inactiveSelectionBackground": hex(colors.data, 0.1),
          "editor.lineHighlightBackground": hex(colors.line, 0.28),
          "editorIndentGuide.background1": hex(colors.line),
          "editorIndentGuide.activeBackground1": muted,
          "editorWidget.background": surface,
          "editorWidget.border": hex(colors.line),
          "editorError.foreground": hex(colors.bad),
          "editorWarning.foreground": hex(colors.warn),
          "scrollbarSlider.background": hex(colors.muted, 0.2),
          "scrollbarSlider.hoverBackground": hex(colors.muted, 0.35),
          "diffEditor.insertedTextBackground": hex(colors.ok, 0.18),
          "diffEditor.insertedLineBackground": hex(colors.ok, 0.08),
          "diffEditor.removedTextBackground": hex(colors.bad, 0.18),
          "diffEditor.removedLineBackground": hex(colors.bad, 0.08),
          "diffEditor.border": hex(colors.line),
        },
      };
      // Both named themes are refreshed from the active CSS tokens before switching.
      monaco.editor.defineTheme("signal-atlas-light", { ...theme, base: "vs" });
      monaco.editor.defineTheme("signal-atlas-dark", {
        ...theme,
        base: "vs-dark",
      });
      monaco.editor.setTheme(name.value);
    },
    { immediate: true }
  );
  return name;
}
