<template>
  <div class="logo-container">
    <transition enter-active-class="animate__animated animate__fadeInLeft">
      <router-link v-if="collapse" class="wh-full flex-center monogram-link" to="/">
        <span class="monogram font-mono">PL</span>
      </router-link>

      <router-link v-else class="wh-full logo-link" to="/">
        <div class="brand-badge font-mono">CORE</div>
        <div class="logo-copy">
          <div class="brand-row">
            <strong class="brand-title">{{ defaultSettings.title }}</strong>
            <span class="status-indicator">
              <span class="beacon-live"></span>
              <small>LIVE</small>
            </span>
          </div>
          <small class="brand-sub font-mono">TELEMETRY & SUB v{{ defaultSettings.version }}</small>
        </div>
      </router-link>
    </transition>
  </div>
</template>

<script lang="ts" setup>
import defaultSettings from "@/settings";

defineProps({
  collapse: {
    type: Boolean,
    required: true,
  },
});
</script>

<style lang="scss" scoped>
.logo-container {
  width: 100%;
  height: $navbar-height;
  background-color: transparent;
  border-bottom: 1px solid var(--ui-border);
  display: flex;
  align-items: center;
}

.monogram-link {
  text-decoration: none;
  .monogram {
    width: 38px;
    height: 38px;
    border-radius: 9px;
    background: var(--ui-surface-strong);
    border: 1px solid var(--ui-border-strong);
    color: var(--ui-accent);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 13px;
    font-weight: 700;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  }
}

.logo-link {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 18px;
  text-decoration: none;
  width: 100%;
}

.brand-badge {
  padding: 3px 6px;
  border-radius: 6px;
  background: var(--ui-surface-strong);
  border: 1px solid var(--ui-border-strong);
  color: var(--ui-accent);
  font-size: 10.5px;
  font-weight: 700;
  letter-spacing: 0.05em;
  flex-shrink: 0;
}

.logo-copy {
  min-width: 0;
  flex: 1;

  .brand-row {
    display: flex;
    align-items: center;
    gap: 8px;

    .brand-title {
      color: var(--ui-text);
      font-size: 16px;
      font-weight: 750;
      letter-spacing: -0.02em;
    }

    .status-indicator {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 1px 6px;
      border-radius: 9999px;
      background: var(--ui-success-soft);
      border: 1px solid rgba(16, 185, 129, 0.25);
      color: var(--ui-success);

      small {
        font-size: 9px;
        font-weight: 700;
        letter-spacing: 0.04em;
      }
    }
  }

  .brand-sub {
    display: block;
    margin-top: 2px;
    color: var(--ui-text-muted);
    font-size: 10px;
    letter-spacing: 0.02em;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}
</style>
