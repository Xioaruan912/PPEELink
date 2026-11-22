<template>
  <el-dropdown class="account-dropdown" trigger="click" placement="bottom-end"
    ><button class="account-avatar" :aria-label="`账户菜单：${name}`">
      {{ name.charAt(0).toUpperCase() }}
    </button>
    <template #dropdown
      ><el-dropdown-menu
        ><el-dropdown-item disabled>{{ name }}</el-dropdown-item
        ><el-dropdown-item divided @click="router.push('/system/user/set')"
          >修改密码</el-dropdown-item
        ><el-dropdown-item @click="emit('logout')"
          >退出登录</el-dropdown-item
        ></el-dropdown-menu
      ></template
    ></el-dropdown
  >
</template>
<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
const props = defineProps<{ username?: string }>();
const emit = defineEmits<{ logout: [] }>();
const router = useRouter();
const name = computed(() => props.username || "admin");
</script>
<style scoped>
.account-avatar {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  margin-left: 4px;
  border-radius: var(--radius-control);
  background: var(--c-ink);
  color: var(--c-surface);
  font: 600 13px/1 var(--font-ui);
  transition:
    background-color var(--duration-fast) ease,
    transform var(--duration-press) var(--ease-out);
}
@media (hover: hover) and (pointer: fine) {
  .account-avatar:hover {
    background: var(--c-signal);
    color: var(--c-on-signal);
  }
}
.account-avatar:active {
  transform: scale(0.95);
}
</style>
