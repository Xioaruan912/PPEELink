<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from "vue";
import IconPlus from "~icons/ph/plus";
import IconPencil from "~icons/ph/pencil-simple";
import IconTrash from "~icons/ph/trash";
import { createUser, deleteUser, editUser, listUsers } from "@/api/user";
import type { ManagedUser, ManagedUserForm } from "@/api/user/types";
import { getSubs } from "@/api/subcription/subs";
import {
  EmptyState,
  PageHeader,
  UiButton,
  UiDrawer,
  UiPanel,
  UiSegmented,
  UiSkeleton,
  UiTag,
} from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useStagger } from "@/motion";
import { useUserStore } from "@/store";

defineOptions({ name: "UserList" });
const me = useUserStore();
const users = ref<ManagedUser[]>([]);
const subs = ref<{ ID: number; Name: string }[]>([]);
const loading = ref(true);
const loadError = ref("");
const root = ref<HTMLElement>();
const stagger = useStagger();

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
    await nextTick();
    if (root.value) stagger(root.value.querySelectorAll(".user-row"));
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
  { label: "普通用户", value: "user" },
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
    ElMessage.success(editing.value ? "已保存" : "已创建，首次登录需修改密码");
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
      `删除账户 ${row.username}？分配关系会一并清除。`,
      "删除用户",
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
onMounted(load);
</script>
<template>
  <div class="users-page">
    <PageHeader
      title="用户管理"
      description="给家人或朋友开账号，只分配他们需要的订阅。"
    >
      <template #actions>
        <UiButton variant="primary" :icon="IconPlus" @click="openCreate"
          >新建用户</UiButton
        >
      </template>
    </PageHeader>
    <UiPanel
      title="全部账户"
      :count="users.length"
      description="普通用户登录后只看到“我的订阅”。"
      flush
      data-enter
    >
      <EmptyState
        v-if="loadError"
        size="section"
        title="无法加载用户"
        :description="loadError"
        ><template #action
          ><UiButton @click="load">重新加载</UiButton></template
        ></EmptyState
      >
      <UiSkeleton v-else-if="loading" :lines="3" height="64px" />
      <div v-else ref="root" class="user-list">
        <article
          v-for="row in users"
          :key="row.id"
          class="user-row"
          :class="{ 'is-disabled': row.disabled }"
        >
          <span class="user-avatar" aria-hidden="true">{{
            row.username.charAt(0).toUpperCase()
          }}</span>
          <div class="user-main">
            <strong
              >{{ row.nickname || row.username
              }}<small v-if="row.nickname"> @{{ row.username }}</small></strong
            >
            <span class="user-meta">
              <UiTag :tone="row.role === 'admin' ? 'signal' : 'neutral'">{{
                row.role === "admin" ? "管理员" : "普通用户"
              }}</UiTag>
              <UiTag v-if="row.disabled" tone="bad">已停用</UiTag>
              <UiTag v-else-if="row.mustChangePassword" tone="warn"
                >待改密码</UiTag
              >
              <span v-if="row.remark" class="user-remark">{{
                row.remark
              }}</span>
            </span>
          </div>
          <div class="user-subs">
            <template v-if="row.role === 'admin'"
              ><span class="muted">全部订阅</span></template
            >
            <template v-else-if="row.subscriptionIds.length">
              <span
                v-for="id in row.subscriptionIds.slice(0, 3)"
                :key="id"
                class="sub-chip"
                >{{ subName(id) || `#${id}` }}</span
              >
              <span v-if="row.subscriptionIds.length > 3" class="muted"
                >+{{ row.subscriptionIds.length - 3 }}</span
              >
            </template>
            <span v-else class="muted">未分配</span>
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
    </UiPanel>
    <UiDrawer
      v-model="drawer"
      :title="editing ? `编辑 ${form.username}` : '新建用户'"
      size="min(520px, 100vw)"
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
          <p class="field-hint">
            设置或重置后，对方首次登录需要自己改一次密码。
          </p>
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
          <el-input v-model="form.remark" placeholder="例如：家人的手机" />
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
        <UiButton @click="drawer = false">取消</UiButton>
        <UiButton variant="primary" :loading="saving" @click="save">{{
          editing ? "保存" : "创建"
        }}</UiButton>
      </template>
    </UiDrawer>
  </div>
</template>
<style scoped>
.users-page {
  min-width: 0;
}
.user-row {
  display: grid;
  grid-template-columns: 40px minmax(0, 1.2fr) minmax(0, 1fr) auto;
  align-items: center;
  gap: 14px;
  min-height: 68px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--c-line);
  transition: background-color var(--duration-fast);
}
.user-row:last-child {
  border-bottom: 0;
}
.user-row:hover {
  background: var(--c-surface-2);
}
.user-row.is-disabled {
  opacity: 0.6;
}
.user-avatar {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: var(--radius-pill);
  background: linear-gradient(
    140deg,
    var(--c-signal),
    color-mix(in srgb, var(--c-signal) 40%, var(--c-data))
  );
  color: var(--c-on-signal);
  font: 600 14px var(--font-mono);
}
.user-main {
  display: grid;
  gap: 4px;
  min-width: 0;
}
.user-main strong {
  overflow: hidden;
  font-size: 14px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.user-main small {
  color: var(--c-muted);
  font-size: 12px;
  font-weight: 400;
}
.user-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
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
  padding: 2px 10px;
  border-radius: var(--radius-pill);
  background: var(--c-surface-2);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.user-actions {
  display: flex;
  gap: 2px;
}
.full {
  width: 100%;
}
.field-hint {
  margin-top: 4px;
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.5;
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
