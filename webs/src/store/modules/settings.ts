import defaultSettings from "@/settings";
import { ThemeEnum } from "@/enums/ThemeEnum";

type SettingsValue = boolean | string;

export const useSettingsStore = defineStore("setting", () => {
  // 是否显示设置
  const settingsVisible = ref(false);
  // 是否显示标签视图
  const tagsView = useStorage<boolean>("tagsView", defaultSettings.tagsView);
  // 是否显示侧边栏logo
  const sidebarLogo = useStorage<boolean>(
    "sidebarLogo",
    defaultSettings.sidebarLogo
  );
  // 是否固定头部
  const fixedHeader = useStorage<boolean>(
    "fixedHeader",
    defaultSettings.fixedHeader
  );
  // 布局模式：left-左侧模式(默认) top-顶部模式 mix-混合模式
  const layout = useStorage<string>("layout", defaultSettings.layout);
  // 主题颜色
  const themeColor = useStorage<string>(
    "themeColor",
    defaultSettings.themeColor
  );
  // 主题：light-亮色(默认) dark-暗色
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
  // 是否开启水印
  const watermarkEnabled = useStorage<boolean>(
    "watermarkEnabled",
    defaultSettings.watermarkEnabled
  );

  // Keep themeColor API compatible; the stylesheet owns Signal Atlas colors.
  function applyTheme(newTheme: string) {
    [
      "--el-color-primary",
      "--el-color-primary-dark-2",
      ...[3, 5, 7, 8, 9].map((n) => `--el-color-primary-light-${n}`),
    ].forEach((name) => document.documentElement.style.removeProperty(name));
    document.documentElement.classList.toggle(
      "dark",
      newTheme === ThemeEnum.DARK
    );
    const color = getComputedStyle(document.documentElement)
      .getPropertyValue("--c-canvas")
      .trim();
    document
      .querySelector('meta[name="theme-color"]')
      ?.setAttribute("content", color);
  }
  watch(theme, applyTheme, { immediate: true });

  const settingsMap: Record<string, Ref<SettingsValue>> = {
    fixedHeader,
    tagsView,
    sidebarLogo,
    layout,
    watermarkEnabled,
  };

  function changeSetting({
    key,
    value,
  }: {
    key: string;
    value: SettingsValue;
  }) {
    const setting = settingsMap[key];
    if (setting) {
      setting.value = value;
    }
  }

  /**
   * 切换主题
   */
  function changeTheme(val: string) {
    theme.value = val;
  }

  /**
   * 切换主题颜色
   *
   * @param color 主题颜色
   *
   */
  function changeThemeColor(color: string) {
    themeColor.value = color;
  }

  /**
   * 切换布局
   */
  function changeLayout(val: string) {
    layout.value = val;
  }

  return {
    settingsVisible,
    tagsView,
    fixedHeader,
    sidebarLogo,
    layout,
    themeColor,
    theme,
    motionReduced,
    watermarkEnabled,
    changeSetting,
    changeTheme,
    changeThemeColor,
    changeLayout,
  };
});
