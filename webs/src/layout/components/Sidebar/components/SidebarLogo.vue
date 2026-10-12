<template>
  <div class="logo-container">
    <transition enter-active-class="animate__animated animate__fadeInLeft">
      <router-link v-if="collapse" class="wh-full flex-center" to="/">
        <span class="brand-mark">
          <img v-if="settingsStore.sidebarLogo" :src="logo" class="logo-image" />
          <i class="brand-mark-ring" aria-hidden="true"></i>
        </span>
      </router-link>

      <router-link v-else class="wh-full logo-link" to="/">
        <span class="brand-mark">
          <img v-if="settingsStore.sidebarLogo" :src="logo" class="logo-image" />
          <i class="brand-mark-ring" aria-hidden="true"></i>
        </span>
        <span class="logo-copy"><strong>{{ defaultSettings.title }}</strong><small>节点与订阅控制台</small></span>
      </router-link>
    </transition>
  </div>
</template>

<script lang="ts" setup>
import defaultSettings from "@/settings";
import { useSettingsStore } from "@/store";

const settingsStore = useSettingsStore();

defineProps({
  collapse: {
    type: Boolean,
    required: true,
  },
});

const logo = ref(new URL(`../../../../assets/logo.png`, import.meta.url).href);
</script>

<style lang="scss" scoped>
.logo-container {
  width: 100%;
  height: $navbar-height;
  background-color: $sidebar-logo-background;

  .logo-image {
    width: 42px;
    height: 42px;
    border-radius: 12px;
    object-fit: cover;
    box-shadow: 0 8px 20px var(--ui-accent-shadow);
    transition: transform 420ms cubic-bezier(0.22, 1, 0.36, 1);
  }
}

.brand-mark {
  position: relative;
  display: grid;
  place-items: center;
}

.brand-mark-ring {
  position: absolute;
  inset: -4px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--ui-accent) 34%, transparent);
  opacity: 0.5;
  pointer-events: none;
}

.logo-link:hover .brand-mark-ring {
  animation: ring-sweep 900ms cubic-bezier(0.22, 1, 0.36, 1);
}

.logo-link:hover .logo-image {
  transform: scale(1.04) rotate(-2deg);
}

@keyframes ring-sweep {
  0% {
    transform: scale(1);
    opacity: 0.5;
  }
  55% {
    transform: scale(1.14);
    opacity: 1;
  }
  100% {
    transform: scale(1);
    opacity: 0.5;
  }
}

.logo-link {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 11px;
  padding: 9px 18px;
  text-decoration: none;
}

.logo-copy {
  min-width: 0;
}

.logo-copy strong,
.logo-copy small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.logo-copy strong {
  color: var(--ui-text);
  font-size: 15px;
  letter-spacing: -0.01em;
}

.logo-copy small {
  margin-top: 3px;
  color: var(--ui-text-muted);
  font-size: 11px;
  font-weight: 500;
}

.layout-top,
.layout-mix {
  .logo-container {
    width: $sidebar-width;
  }

  &.hideSidebar {
    .logo-container {
      width: $sidebar-width-collapsed;
    }
  }
}

@media (prefers-reduced-motion: reduce) {
  .logo-link:hover .brand-mark-ring {
    animation: none;
  }
}
</style>
