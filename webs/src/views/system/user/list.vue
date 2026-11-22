<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import gsap from "gsap";
import IconPlus from "~icons/ph/plus";
import IconPencil from "~icons/ph/pencil-simple";
import IconTrash from "~icons/ph/trash";
import { createUser, deleteUser, editUser, listUsers } from "@/api/user";
import type { ManagedUser, ManagedUserForm } from "@/api/user/types";
import { getSubs } from "@/api/subcription/subs";
import {
  EmptyState,
  UiButton,
  UiDrawer,
  UiSegmented,
  UiSkeleton,
  UiTag,
} from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useUserStore } from "@/store";

defineOptions({ name: "UserList" });

const me = useUserStore();
const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const users = ref<ManagedUser[]>([]);
const subs = ref<{ ID: number; Name: string }[]>([]);
const loading = ref(true);
const loadError = ref("");
const roleFilter = ref<"all" | "admin" | "user">("all");

const filteredUsers = computed(() => {
  if (roleFilter.value === "all") return users.value;
  return users.value.filter((u) => u.role === roleFilter.value);
});

const subName = (id: number) => subs.value.find((s) => s.ID === id)?.Name;

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const [u, s] = await Promise.all([listUsers(), getSubs()]);
    users.value = u.data?.list || [];
    subs.value = (s.data || []).map((x: any) => ({ ID: x.ID, Name: x.Name }));
  } catch (error: any) {
    loadError.value = error?.message || "请检查网络后重试";
  } finally {
    loading.value = false;
  }
}

const drawer = ref(false);
const saving = ref(false);
const formError = ref("");
const blank = (): ManagedUserForm => ({
  username: "",
  password: "",
  nickname: "",
  role: "user",
  remark: "",
  disabled: false,
  subscriptionIds: [],
});
const form = reactive<ManagedUserForm>(blank());
const editing = computed(() => !!form.id);
const roleOptions = [
  { label: "普通成员", value: "user" },
  { label: "管理员", value: "admin" },
];

function openCreate() {
  Object.assign(form, blank(), { id: undefined });
  formError.value = "";
  drawer.value = true;
}

function openEdit(row: ManagedUser) {
  Object.assign(form, {
    ...row,
    password: "",
    subscriptionIds: [...row.subscriptionIds],
  });
  formError.value = "";
  drawer.value = true;
}

async function save() {
  if (saving.value) return;
  if (
    !editing.value &&
    (!form.username.trim() || (form.password || "").length < 8)
  ) {
    formError.value = "请填写用户名，初始密码至少 8 位";
    return;
  }
  if (form.password && form.password.length < 8) {
    formError.value = "重置密码至少 8 位";
    return;
  }
  saving.value = true;
  formError.value = "";
  try {
    if (editing.value) await editUser({ ...form });
    else await createUser({ ...form, username: form.username.trim() });
    ElMessage.success(editing.value ? "已保存" : "成员已创建");
    drawer.value = false;
    await load();
  } catch (error: any) {
    formError.value = error?.message || "保存失败";
  } finally {
    saving.value = false;
  }
}

async function remove(row: ManagedUser) {
  try {
    await ElMessageBox.confirm(
      `删除成员 ${row.username}？订阅分配会一并清除。`,
      "删除成员",
      {
        type: "warning",
        confirmButtonText: "删除",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
      }
    );
  } catch {
    return;
  }
  try {
    await deleteUser(row.id);
    ElMessage.success("已删除");
    await load();
  } catch {
    /* 请求层提示错误 */
  }
}

onMounted(() => {
  void load();
  if (rootEl.value) {
    gsapCtx = gsap.context(() => {
      const mm = gsap.matchMedia();
      mm.add("(prefers-reduced-motion: no-preference)", () => {
        const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
        tl.from("[data-title-reveal]", {
          yPercent: 108,
          duration: 0.62,
          clearProps: "transform",
        }).from(
          "[data-top-rule]",
          {
            scaleX: 0,
            duration: 0.68,
            clearProps: "transform",
          },
          "-=0.45"
        );
      });
    }, rootEl.value);
  }
});

onBeforeUnmount(() => {
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<template>
  <div ref="rootEl" class="users-gallery">
    <!-- Pure Gallery Masthead -->
    <header class="users-masthead">
      <div class="masthead-left">
        <div class="title-mask">
          <h1 class="gallery-title" data-title-reveal>成员与权限</h1>
        </div>
        <span v-if="!loading" class="count-pill mono">
          {{ users.length }} 个账户
        </span>
      </div>

      <div class="masthead-actions">
        <UiButton variant="primary" :icon="IconPlus" @click="openCreate">
          新建成员
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <!-- Clean Filter Pills -->
    <div class="filter-bar">
      <div class="pill-group" role="group" aria-label="角色筛选">
        <button
          type="button"
          class="seg-pill"
          :class="{ 'is-active': roleFilter === 'all' }"
          @click="roleFilter = 'all'"
        >
          全部 ({{ users.length }})
        </button>
        <button
          type="button"
          class="seg-pill"
          :class="{ 'is-active': roleFilter === 'admin' }"
          @click="roleFilter = 'admin'"
        >
          管理员
        </button>
        <button
          type="button"
          class="seg-pill"
          :class="{ 'is-active': roleFilter === 'user' }"
          @click="roleFilter = 'user'"
        >
          普通成员
        </button>
      </div>
    </div>

    <EmptyState
      v-if="loadError"
      size="section"
      title="无法加载成员列表"
      :description="loadError"
    >
      <template #action>
        <UiButton @click="load">重新加载</UiButton>
      </template>
    </EmptyState>

    <UiSkeleton v-else-if="loading" :lines="4" height="64px" />

    <!-- Borderless Full-Width Gallery Rows -->
    <div v-else class="user-list">
      <article
        v-for="row in filteredUsers"
        :key="row.id"
        class="user-row"
        :class="{ 'is-disabled': row.disabled }"
      >
        <span
          class="user-avatar mono"
          :class="{ 'is-admin': row.role === 'admin' }"
          aria-hidden="true"
        >
          {{ row.username.charAt(0).toUpperCase() }}
        </span>

        <div class="user-main">
          <div class="user-name-line">
            <strong>{{ row.nickname || row.username }}</strong>
            <small v-if="row.nickname" class="mono">@{{ row.username }}</small>
            <UiTag :tone="row.role === 'admin' ? 'signal' : 'neutral'">
              {{ row.role === "admin" ? "管理员" : "普通成员" }}
            </UiTag>
            <UiTag v-if="row.disabled" tone="bad">已停用</UiTag>
            <UiTag v-else-if="row.mustChangePassword" tone="warn">待改密</UiTag>
          </div>
          <span v-if="row.remark" class="user-remark">{{ row.remark }}</span>
        </div>

        <div class="user-subs">
          <template v-if="row.role === 'admin'">
            <span class="muted">全部订阅</span>
          </template>
          <template v-else-if="row.subscriptionIds.length">
            <span
              v-for="id in row.subscriptionIds.slice(0, 3)"
              :key="id"
              class="sub-chip"
            >
              {{ subName(id) || `#${id}` }}
            </span>
            <span v-if="row.subscriptionIds.length > 3" class="muted mono">
              +{{ row.subscriptionIds.length - 3 }}
            </span>
          </template>
          <span v-else class="muted">未分配订阅</span>
        </div>

        <div class="user-actions">
          <RowIconButton
            :icon="IconPencil"
            :label="`编辑 ${row.username}`"
            @click="openEdit(row)"
          />
          <RowIconButton
            :icon="IconTrash"
            :label="`删除 ${row.username}`"
            :disabled="row.username === me.user.username"
            @click="remove(row)"
          />
        </div>
      </article>
    </div>

    <!-- Clean Member Drawer -->
    <UiDrawer
      v-model="drawer"
      :title="editing ? `编辑 ${form.username}` : '新建成员'"
      size="min(480px, 100vw)"
    >
      <el-form label-position="top" class="user-form" @submit.prevent="save">
        <el-form-item label="用户名" required>
          <el-input
            v-model="form.username"
            :disabled="editing"
            autocomplete="off"
            placeholder="登录用户名"
          />
        </el-form-item>
        <el-form-item
          :label="editing ? '重置密码（留空不改）' : '初始密码'"
          :required="!editing"
        >
          <el-input
            v-model="form.password"
            type="password"
            show-password
            autocomplete="new-password"
            placeholder="至少 8 位"
          />
        </el-form-item>
        <el-form-item label="昵称">
          <el-input v-model="form.nickname" placeholder="显示名称（可选）" />
        </el-form-item>
        <el-form-item label="角色">
          <UiSegmented
            v-model="form.role"
            :options="roleOptions"
            label="角色"
          />
        </el-form-item>
        <el-form-item v-if="form.role === 'user'" label="可见订阅">
          <el-select
            v-model="form.subscriptionIds"
            multiple
            filterable
            placeholder="选择分配的订阅"
            class="full"
          >
            <el-option
              v-for="s in subs"
              :key="s.ID"
              :label="s.Name"
              :value="s.ID"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" placeholder="例如：家人的设备" />
        </el-form-item>
        <el-form-item v-if="editing" label="停用账户">
          <el-switch
            v-model="form.disabled"
            :disabled="form.username === me.user.username"
          />
        </el-form-item>
        <p v-if="formError" class="form-error" role="alert">{{ formError }}</p>
      </el-form>
      <template #footer>
        <UiButton variant="ghost" @click="drawer = false">取消</UiButton>
        <UiButton variant="primary" :loading="saving" @click="save">
          {{ editing ? "保存" : "创建" }}
        </UiButton>
      </template>
    </UiDrawer>
  </div>
</template>

<style scoped>
.users-gallery {
  display: grid;
  gap: 20px;
  max-width: 1120px;
  min-width: 0;
}

.users-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.masthead-left {
  display: flex;
  align-items: baseline;
  gap: 14px;
  flex-wrap: wrap;
}

.title-mask {
  overflow: hidden;
  padding-bottom: 4px;
  margin-bottom: -4px;
}

.gallery-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(26px, 2.8vw, 36px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.count-pill {
  padding: 5px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-muted);
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
}

.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.seg-pill {
  padding: 6px 13px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.seg-pill:hover {
  color: var(--c-ink);
  border-color: var(--c-line);
}

.seg-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

/* Borderless Full-Width Gallery Rows */
.user-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.user-list:hover .user-row:not(:hover) {
  opacity: 0.58;
}

.user-row {
  position: relative;
  display: grid;
  grid-template-columns: 40px minmax(0, 1.3fr) minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  padding: 18px 10px;
  border-bottom: 1px solid var(--c-line);
  transition: opacity 220ms var(--ease-out);
}

.user-row::before {
  content: "";
  position: absolute;
  inset: 0;
  background: var(--c-surface);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 320ms cubic-bezier(0.16, 1, 0.3, 1);
  z-index: 0;
}

.user-row:hover::before {
  transform: scaleX(1);
}

.user-row > * {
  position: relative;
  z-index: 1;
}

.user-row.is-disabled {
  opacity: 0.55;
}

.user-avatar {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 12px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 13px;
  font-weight: 600;
}

.user-avatar.is-admin {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.user-main {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.user-name-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.user-name-line strong {
  font-size: 14.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.user-name-line small {
  font-size: 12px;
  color: var(--c-muted);
}

.user-remark,
.muted {
  color: var(--c-muted);
  font-size: 12px;
}

.user-subs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.sub-chip {
  max-width: 160px;
  overflow: hidden;
  padding: 3px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 11.5px;
  color: var(--c-ink);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-actions {
  display: flex;
  gap: 4px;
}

.full {
  width: 100%;
}

.form-error {
  color: var(--c-bad);
  font-size: 13px;
}

@media (max-width: 767px) {
  .user-row {
    grid-template-columns: 36px minmax(0, 1fr) auto;
  }
  .user-subs {
    grid-column: 2 / -1;
    grid-row: 2;
  }
}
</style>
