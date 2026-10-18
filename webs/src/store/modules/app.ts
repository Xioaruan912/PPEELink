import defaultSettings from "@/settings";
import zhCn from "element-plus/es/locale/lang/zh-cn";
import en from "element-plus/es/locale/lang/en";
import { store } from "@/store";

export const useAppStore = defineStore("app", () => {
  const size = useStorage("size", defaultSettings.size);
  const language = useStorage("language", defaultSettings.language);
  const locale = computed(() => (language.value === "en" ? en : zhCn));
  return { size, language, locale };
});

export function useAppStoreHook() {
  return useAppStore(store);
}
