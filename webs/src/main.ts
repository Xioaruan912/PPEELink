import { createApp } from "vue";
import App from "./App.vue";
import router from "@/router";
import { setupStore } from "@/store";
import { setupElIcons, setupI18n, setupPermission } from "@/plugins";
import { installSpotlight } from "@/motion";

// 本地SVG图标
import "virtual:svg-icons-register";

// 样式
import "@/styles/fonts.css";
import "@/styles/index.scss";
import "uno.css";

const app = createApp(App);
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
installSpotlight();
