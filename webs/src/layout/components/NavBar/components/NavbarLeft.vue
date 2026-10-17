<template>
  <div class="navbar-left">
    <hamburger
      :is-active="appStore.sidebar.opened"
      @toggle-click="toggleSideBar"
    />
    <div class="route-heading">
      <div class="heading-row">
        <span class="route-slash font-mono">/</span>
        <strong>{{ routeTitle }}</strong>
      </div>
      <span class="route-desc">{{ routeDescription }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useAppStore } from "@/store";

const appStore = useAppStore();
const route = useRoute();
const routeTitle = computed(() => {
  const t = String(route.meta?.title || "控制台");
  const titles: Record<string, string> = {
    dashboard: "系统仪表盘",
    sublist: "订阅管理",
    多订阅合集: "多订阅合集",
    nodelist: "节点管理",
    机场管理: "机场管理",
    clientdownload: "客户端直链",
    真实测速: "出口带宽测速",
    unlocktest: "流媒体解锁",
    tcptest: "全国 TCP 延迟",
    templatelist: "配置模板库",
    templatebuilder: "规则构建器",
    rulecenter: "规则分流中心",
    userset: "账户安全设置",
  };
  return titles[t] || t;
});

const descriptions: Record<string, string> = {
  "/": "全球拓扑实时遥测与高并发服务总线",
  "/dashboard": "全球拓扑实时遥测与高并发服务总线",
  "/subcription": "节点池调度、机场自动测活与客户端订阅编排",
  "/test": "多维出口带宽吞吐、低延迟握手与流媒体解锁深度审计",
  "/template": "自动化分流策略组与多平台订阅客户端配置下发",
  "/system": "系统安全、管理员凭证与 API 访问令牌控制",
};

const routeDescription = computed(() => {
  const key = Object.keys(descriptions).find(k => k !== "/" && route.path.startsWith(k));
  return descriptions[key || "/"];
});

function toggleSideBar() {
  appStore.toggleSidebar();
}
</script>

<style scoped lang="scss">
.navbar-left {
  display: flex;
  align-items: center;
  min-width: 0;
  height: 100%;
  gap: 12px;
}

.route-heading {
  min-width: 0;

  .heading-row {
    display: flex;
    align-items: center;
    gap: 6px;

    .route-slash {
      color: var(--ui-accent);
      font-weight: 700;
      font-size: 14px;
    }

    strong {
      color: var(--ui-text);
      font-size: 15px;
      font-weight: 750;
      letter-spacing: -0.01em;
      line-height: 1.2;
    }
  }

  .route-desc {
    display: block;
    margin-top: 2px;
    color: var(--ui-text-muted);
    font-size: 11px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

@media (max-width: 700px) {
  .route-heading .route-desc {
    display: none;
  }
}
</style>
