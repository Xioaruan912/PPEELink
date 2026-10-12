import { createApp } from "vue";
import App from "./App.vue";
import router from "@/router";
import { setupStore } from "@/store";
import { setupDirective } from "@/directive";
import { setupMotion } from "@/motion";
import { setupElIcons, setupI18n, setupPermission } from "@/plugins";

// 本地SVG图标
import "virtual:svg-icons-register";

// 样式
import "element-plus/theme-chalk/dark/css-vars.css";
import "@/styles/index.scss";
import "uno.css";
import "animate.css";

const app = createApp(App);
// 全局注册 自定义指令(directive)
setupDirective(app);
// 注册动效系统（v-reveal / v-magnetic / v-tilt / 页面编排 / 光标聚光）
setupMotion(app, router);
// 全局注册 状态管理(store)
setupStore(app);
// 全局注册Element-plus图标
setupElIcons(app);
// 国际化
setupI18n(app);
// 注册动态路由
setupPermission();
app.use(router);
app.mount("#app");
