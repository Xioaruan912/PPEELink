<template>
  <div class="sidebar-footer">
    <div class="theme-selector" role="group" aria-label="界面主题">
      <button :class="{ active: theme === ThemeEnum.DARK }" @click="setTheme(ThemeEnum.DARK)">
        <span class="btn-dot"></span>
        暗色模式
      </button>
      <button :class="{ active: theme === ThemeEnum.LIGHT }" @click="setTheme(ThemeEnum.LIGHT)">
        浅色模式
      </button>
    </div>

    <div class="telemetry-bar font-mono">
      <div class="telemetry-pill">
        <span class="beacon-live"></span>
        <span>ONLINE</span>
      </div>
      <div class="telemetry-stat">
        <span>GATEWAY</span>
        <strong class="text-cyan">8000</strong>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useSettingsStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import defaultSettings from "@/settings";

const settingsStore = useSettingsStore();
const theme = computed(() => settingsStore.theme);
const setTheme = (value: ThemeEnum) => settingsStore.changeTheme(value);
</script>

<style lang="scss" scoped>
.sidebar-footer {
  padding: 14px 16px;
  border-top: 1px solid var(--ui-border);
  background: var(--ui-sidebar);
}

.theme-selector {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px;
  padding: 3px;
  border: 1px solid var(--ui-border);
  border-radius: 9px;
  background: var(--ui-surface);

  button {
    height: 32px;
    border: 0;
    border-radius: 7px;
    background: transparent;
    color: var(--ui-text-muted);
    font-size: 11.5px;
    font-weight: 600;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    transition: all 140ms ease;
    cursor: pointer;

    .btn-dot {
      width: 5px;
      height: 5px;
      border-radius: 50%;
      background: var(--ui-accent);
      opacity: 0;
      transition: opacity 140ms ease;
    }

    &:hover {
      color: var(--ui-text);
      background: var(--ui-hover);
    }

    &.active {
      color: var(--ui-text);
      background: var(--ui-surface-strong);
      box-shadow: inset 0 0 0 1px var(--ui-border-strong), 0 1px 3px rgba(0, 0, 0, 0.2);

      .btn-dot {
        opacity: 1;
      }
    }
  }
}

.telemetry-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
  padding: 6px 8px;
  border-radius: 8px;
  background: var(--ui-surface);
  border: 1px solid var(--ui-border-subtle);
  font-size: 10px;

  .telemetry-pill {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--ui-success);
    font-weight: 700;
  }

  .telemetry-stat {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--ui-text-muted);

    strong {
      color: var(--ui-accent);
      font-weight: 700;
    }
  }
}

.hideSidebar & {
  display: none;
}
</style>
