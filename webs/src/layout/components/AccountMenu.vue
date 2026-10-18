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
  width: 32px;
  height: 32px;
  margin-left: 4px;
  border-radius: var(--radius-pill);
  background: linear-gradient(
    140deg,
    var(--c-signal),
    color-mix(in srgb, var(--c-signal) 40%, var(--c-data))
  );
  color: var(--c-on-signal);
  font: 600 13px var(--font-mono);
  box-shadow: var(--edge-highlight);
  transition: transform var(--duration-medium) var(--ease-spring);
}
.account-avatar:hover {
  transform: scale(1.06);
}
</style>
