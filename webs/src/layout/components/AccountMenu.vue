<template>
  <el-dropdown class="account-dropdown" trigger="click" placement="top-start"
    ><button class="account-row" :aria-label="`账户菜单：${name}`">
      <span class="account-avatar" aria-hidden="true">{{
        name.charAt(0).toUpperCase()
      }}</span
      ><span v-if="!compact" class="account-copy"
        ><strong>{{ name }}</strong
        ><small>管理员</small></span
      ><MoreFilled v-if="!compact" class="account-more" aria-hidden="true" />
    </button>
    <template #dropdown
      ><el-dropdown-menu
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
import { computed } from "vue";
import { useRouter } from "vue-router";
import { MoreFilled } from "@element-plus/icons-vue";
const props = defineProps<{ username?: string; compact?: boolean }>();
const emit = defineEmits<{ logout: [] }>();
const router = useRouter();
const name = computed(() => props.username || "admin");
</script>
<style scoped>
.account-row {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 44px;
  margin-top: 6px;
  padding: 4px 6px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  background: var(--c-canvas);
  color: var(--c-ink);
  text-align: left;
}
.account-row:hover {
  border-color: var(--c-muted);
}
.account-avatar {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 30px;
  height: 30px;
  border-radius: 7px;
  background: var(--c-ink);
  color: var(--c-canvas);
  font: 600 14px var(--font-mono);
}
.account-copy {
  display: grid;
  min-width: 0;
  line-height: 1.2;
}
.account-copy strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 500;
}
.account-copy small {
  color: var(--c-muted);
  font-size: 12px;
}
.account-more {
  width: 16px;
  height: 16px;
  margin-left: auto;
  color: var(--c-muted);
}
.account-dropdown {
  display: block;
  width: 100%;
}
</style>
