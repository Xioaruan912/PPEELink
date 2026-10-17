import type { App } from "vue";

import { vMagnetic, vReveal } from "@/motion";
import { hasPerm } from "./permission";

// 全局注册 directive
export function setupDirective(app: App<Element>) {
  // 使 v-hasPerm 在所有组件中都可用
  app.directive("hasPerm", hasPerm);
  app.directive("magnetic", vMagnetic);
  app.directive("reveal", vReveal);
}
