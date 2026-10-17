<template>
  <el-dropdown trigger="click"
    ><button class="account-menu" :aria-label="label">
      {{ (username || "U").charAt(0).toUpperCase() }}</button
    ><template #dropdown
      ><el-dropdown-menu
        ><el-dropdown-item disabled>{{
          username || "当前账户"
        }}</el-dropdown-item
        ><el-dropdown-item @click="router.push('/system/user/set')"
          >修改密码</el-dropdown-item
        ><el-dropdown-item divided @click="emit('logout')"
          >退出登录</el-dropdown-item
        ></el-dropdown-menu
      ></template
    ></el-dropdown
  >
</template>
<script setup lang="ts">
import { useRouter } from "vue-router";
withDefaults(defineProps<{ username?: string; label?: string }>(), {
  label: "用户菜单",
});
const emit = defineEmits<{ logout: [] }>();
const router = useRouter();
</script>
<style scoped>
.account-menu {
  display: inline-grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border: 1px solid transparent;
  border-radius: var(--radius-control);
  background: var(--c-surface-2);
  color: var(--c-ink);
  font: 500 16px var(--font-mono);
}
.account-menu:hover {
  border-color: var(--c-line);
}
</style>
