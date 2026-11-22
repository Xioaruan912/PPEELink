<template>
  <el-dropdown
    class="account-dropdown"
    trigger="click"
    placement="bottom-end"
    popper-class="account-menu-popper"
  >
    <button
      type="button"
      class="account-avatar"
      :aria-label="`账户菜单：${name}`"
    >
      <span>{{ name.charAt(0).toUpperCase() }}</span>
      <i class="avatar-dot" aria-hidden="true"></i>
    </button>
    <template #dropdown>
      <el-dropdown-menu class="account-studio-menu">
        <div class="account-card" aria-hidden="true">
          <span class="account-card-badge">{{
            name.charAt(0).toUpperCase()
          }}</span>
          <div class="account-card-meta">
            <strong>{{ name }}</strong>
            <span><i class="live-dot"></i>在线会话</span>
          </div>
        </div>
        <el-dropdown-item divided @click="router.push('/system/user/set')">
          <span class="menu-row">
            <IconLock class="menu-icon" aria-hidden="true" />
            <span>修改密码</span>
          </span>
        </el-dropdown-item>
        <el-dropdown-item @click="emit('logout')">
          <span class="menu-row is-danger">
            <IconSignOut class="menu-icon" aria-hidden="true" />
            <span>退出登录</span>
          </span>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import IconLock from "~icons/ph/lock-key";
import IconSignOut from "~icons/ph/sign-out";

const props = defineProps<{ username?: string }>();
const emit = defineEmits<{ logout: [] }>();
const router = useRouter();
const name = computed(() => props.username || "admin");
</script>

<style scoped>
.account-avatar {
  position: relative;
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  margin-left: 4px;
  border: 0;
  border-radius: 10px;
  background: var(--c-ink);
  color: var(--c-surface);
  font: 600 13px/1 var(--font-ui);
  cursor: pointer;
  transition:
    transform 120ms var(--ease-out),
    box-shadow var(--duration-fast) var(--ease-out);
}

.avatar-dot {
  position: absolute;
  right: -2px;
  bottom: -2px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-ok);
  border: 2px solid var(--c-surface);
}

.account-avatar:hover {
  box-shadow: 0 6px 16px -4px rgb(0 0 0 / 0.28);
}

.account-avatar:active {
  transform: scale(0.93);
}

.account-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px 8px;
  min-width: 176px;
}

.account-card-badge {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: var(--c-ink);
  color: var(--c-surface);
  font: 600 13px/1 var(--font-ui);
  flex: none;
}

.account-card-meta {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.account-card-meta strong {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--c-ink);
  line-height: 1.2;
}

.account-card-meta span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: var(--c-muted);
}

.live-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--c-ok);
}

.menu-row {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  font-size: 13px;
}

.menu-icon {
  width: 15px;
  height: 15px;
  color: var(--c-muted);
}

.menu-row.is-danger,
.menu-row.is-danger .menu-icon {
  color: var(--c-bad);
}
</style>
