import defaultSettings from "@/settings";
import { ThemeEnum } from "@/enums/ThemeEnum";

export const useSettingsStore = defineStore("setting", () => {
  const theme = useStorage<string>("theme", defaultSettings.theme);
  const motionReduced = useStorage<boolean>("workspace-reduced-motion", false);
  watch(
    motionReduced,
    (value) => {
      document.documentElement.dataset.motion = value ? "reduced" : "full";
      window.dispatchEvent(new Event("workspace-motion-change"));
    },
    { immediate: true }
  );
  function applyTheme(newTheme: string) {
    document.documentElement.classList.toggle(
      "dark",
      newTheme === ThemeEnum.DARK
    );
    // Read the single source of colour after applying the theme class.
    const color = getComputedStyle(document.documentElement)
      .getPropertyValue("--c-canvas")
      .trim();
    document
      .querySelector('meta[name="theme-color"]')
      ?.setAttribute("content", color);
  }
  watch(theme, applyTheme, { immediate: true });
  function changeTheme(value: string) {
    theme.value = value;
  }
  return { theme, motionReduced, changeTheme };
});
