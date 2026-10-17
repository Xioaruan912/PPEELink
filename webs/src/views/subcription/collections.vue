<script setup lang="ts">
import {
  ref,
  computed,
  onMounted,
  onBeforeUnmount,
  nextTick,
  watch,
} from "vue";
import { ElMessage } from "element-plus";
import { Plus, Edit, MoreFilled } from "@element-plus/icons-vue";
import {
  getCollections,
  addCollection,
  updateCollection,
  deleteCollection,
  resetCollectionToken,
} from "@/api/subcription/collection";
import { getSubs } from "@/api/subcription/subs";
import {
  PageHeader,
  UiButton,
  UiDrawer,
  UiDialog,
  UiSegmented,
  UiStat,
  UiSkeleton,
  StatusDot,
  EmptyState,
} from "@/ui";
import { useMotion, useStagger } from "@/motion";
import SecretLink from "./subs/SecretLink.vue";
import CopyLinkButton from "./subs/CopyLinkButton.vue";
import RowIconButton from "./subs/RowIconButton.vue";
import { formatDateTime } from "@/utils/date";
defineOptions({ name: "Collections" });
interface Collection {
  id: number;
  name: string;
  token: string;
  memberIds: string;
  dedupe: boolean;
  expiresAt: string | null;
  pipeline: string;
  note: string;
}
const clients = [
  "clash",
  "surge",
  "loon",
  "v2ray",
  "singbox",
  "qx",
  "shadowrocket",
];
const client = ref("clash"),
  items = ref<Collection[]>([]),
  subs = ref<{ ID: number; Name: string }[]>([]),
  loading = ref(true),
  loadError = ref(""),
  search = ref(""),
  dialog = ref(false),
  saving = ref(false),
  section = ref<string | number>("basic"),
  root = ref<HTMLElement>();
const { reduced } = useMotion();
const stagger = useStagger();
let alive = true;
let loadVersion = 0;
const form = ref({
  id: 0,
  name: "",
  memberList: [] as number[],
  dedupe: true,
  expires: null as Date | null,
  pipeline: "",
  note: "",
});
const confirmation = ref({
  visible: false,
  title: "",
  message: "",
  action: "",
});
let pendingAction: (() => Promise<void>) | undefined;
const deleting = ref(false);
const memberIds = (row: Collection): number[] => {
  try {
    const ids = JSON.parse(row.memberIds || "[]");
    return Array.isArray(ids) ? ids : [];
  } catch {
    return [];
  }
};
const filtered = computed(() =>
  items.value.filter((row) =>
    row.name.toLowerCase().includes(search.value.trim().toLowerCase())
  )
);
const expired = (row: Collection) =>
  !!row.expiresAt && new Date(row.expiresAt).getTime() <= Date.now();
const collectionLink = (row: Collection) =>
  `${location.origin}/cc/?token=${row.token}&client=${client.value}`;
const clientOptions = clients.map((value) => ({
  label:
    value === "qx"
      ? "Quantumult X"
      : value === "v2ray"
        ? "V2Ray"
        : value === "singbox"
          ? "Sing-box"
          : value === "shadowrocket"
            ? "Shadowrocket"
            : value[0].toUpperCase() + value.slice(1),
  value,
}));
const editingCollection = computed(() =>
  items.value.find((row) => row.id === form.value.id)
);
async function load() {
  const version = ++loadVersion;
  loading.value = true;
  loadError.value = "";
  try {
    const [c, s] = await Promise.all([getCollections(), getSubs()]);
    if (!alive || version !== loadVersion) return;
    items.value = c.data || [];
    subs.value = s.data || [];
  } catch {
    if (alive && version === loadVersion)
      loadError.value = "合集或成员订阅加载失败，请重试";
  } finally {
    if (alive && version === loadVersion) loading.value = false;
  }
}
function openEdit(row?: Collection) {
  form.value = row
    ? {
        id: row.id,
        name: row.name,
        memberList: memberIds(row),
        dedupe: !!row.dedupe,
        expires: row.expiresAt ? new Date(row.expiresAt) : null,
        pipeline: row.pipeline || "",
        note: row.note || "",
      }
    : {
        id: 0,
        name: "",
        memberList: [],
        dedupe: true,
        expires: null,
        pipeline: "",
        note: "",
      };
  section.value = "basic";
  dialog.value = true;
}
async function save() {
  if (!form.value.name.trim()) {
    section.value = "basic";
    ElMessage.warning("名称不能为空");
    return;
  }
  if (form.value.pipeline.trim()) {
    try {
      const parsed = JSON.parse(form.value.pipeline);
      if (!parsed || Array.isArray(parsed) || typeof parsed !== "object")
        throw new Error("object required");
    } catch {
      section.value = "advanced";
      ElMessage.warning("处理链必须是 JSON 对象");
      return;
    }
  }
  if (
    form.value.expires &&
    !Number.isFinite(new Date(form.value.expires).getTime())
  ) {
    ElMessage.warning("请选择有效过期时间");
    return;
  }
  saving.value = true;
  try {
    const payload = {
      id: form.value.id,
      name: form.value.name.trim(),
      dedupe: form.value.dedupe,
      memberIds: JSON.stringify(form.value.memberList || []),
      pipeline: form.value.pipeline || "",
      note: form.value.note || "",
      expiresAt: form.value.expires
        ? new Date(form.value.expires).toISOString()
        : null,
    };
    if (form.value.id) await updateCollection(payload);
    else await addCollection(payload);
    ElMessage.success("已保存");
    dialog.value = false;
    await load();
  } catch {
    ElMessage.error("合集未保存，请重试");
  } finally {
    saving.value = false;
  }
}
function resetToken(row: Collection) {
  confirmation.value = {
    visible: true,
    title: "重置合集令牌",
    message: `重置「${row.name}」的令牌？旧链接立即失效。`,
    action: "重置令牌",
  };
  pendingAction = async () => {
    await resetCollectionToken(row.id);
    ElMessage.success("已重置");
    await load();
  };
}
function remove(row: Collection) {
  confirmation.value = {
    visible: true,
    title: "删除合集",
    message: `删除合集「${row.name}」？`,
    action: "删除合集",
  };
  pendingAction = async () => {
    await deleteCollection(row.id);
    ElMessage.success("已删除");
    await load();
  };
}
async function confirm() {
  if (!pendingAction || deleting.value) return;
  deleting.value = true;
  try {
    await pendingAction();
    confirmation.value.visible = false;
    pendingAction = undefined;
  } catch {
    ElMessage.error("操作失败，请重试");
  } finally {
    deleting.value = false;
  }
}
watch(loading, async (value) => {
  if (!value) {
    await nextTick();
    if (root.value) stagger(root.value.querySelectorAll(".subscription-row"));
  }
});
onMounted(load);
onBeforeUnmount(() => {
  alive = false;
  pendingAction = undefined;
});
</script>
<template>
  <div class="collections-page">
    <PageHeader eyebrow="订阅 / 02 合集" title="多订阅合集"
      ><template #actions
        ><UiButton variant="primary" :icon="Plus" @click="openEdit()"
          >新建合集</UiButton
        ></template
      ></PageHeader
    >
    <div class="subscription-summary" data-enter>
      <UiStat size="large" label="合集总数" :value="items.length" />
      <p class="collection-intro">合并订阅，统一下发到设备。</p>
    </div>
    <div class="list-toolbar" data-enter>
      <el-input
        v-model="search"
        placeholder="搜索合集名称"
        aria-label="搜索合集名称"
        clearable
      /><UiButton :loading="loading" @click="load">刷新</UiButton>
    </div>
    <EmptyState
      v-if="loadError"
      size="section"
      title="合集加载失败"
      :description="loadError"
      ><template #action
        ><UiButton @click="load">重新加载</UiButton></template
      ></EmptyState
    ><UiSkeleton v-else-if="loading" :lines="4" height="80px" />
    <div v-else ref="root">
      <EmptyState
        v-if="!items.length"
        size="page"
        title="把订阅汇在一起"
        description="选择成员订阅，生成统一链接。"
        ><template #action
          ><UiButton @click="openEdit()">新建合集</UiButton></template
        ></EmptyState
      ><EmptyState
        v-else-if="!filtered.length"
        size="section"
        title="没有匹配合集"
        ><template #action
          ><UiButton @click="search = ''">清除搜索</UiButton></template
        ></EmptyState
      >
      <div v-if="filtered.length" class="list-heading" aria-hidden="true">
        <span>名称 / 状态</span><span>成员数</span><span>过期时间</span
        ><span>节点去重</span><span class="actions-heading">操作</span>
      </div>
      <TransitionGroup
        tag="div"
        class="subscription-list"
        :name="reduced ? '' : 'subscription'"
        ><article
          v-for="row in filtered"
          :key="row.id"
          class="subscription-row"
        >
          <button
            class="name-button"
            :title="row.note ? `${row.name} · ${row.note}` : row.name"
            @click="openEdit(row)"
          >
            <span class="name-text">{{ row.name }}</span
            ><span class="name-meta"
              ><StatusDot
                :state="expired(row) ? 'bad' : 'ok'"
                :label="expired(row) ? '已过期' : '有效'"
              /><span v-if="row.note" class="collection-note">{{
                row.note
              }}</span></span
            >
          </button>
          <div class="row-value">
            <span class="mobile-label">成员数</span
            ><span class="mono">{{ memberIds(row).length }}</span>
          </div>
          <div class="row-value">
            <span class="mobile-label">过期时间</span
            ><time class="mono">{{
              row.expiresAt ? formatDateTime(row.expiresAt) : "永不过期"
            }}</time>
          </div>
          <div class="row-value">
            <span class="mobile-label">节点去重</span
            ><span>{{ row.dedupe ? "开启" : "关闭" }}</span>
          </div>
          <div class="row-actions collection-actions">
            <CopyLinkButton
              v-model="client"
              :value="collectionLink(row)"
              :options="clientOptions"
              :label="row.name"
            />
            <RowIconButton
              :icon="Edit"
              :label="`编辑 ${row.name}`"
              @click="openEdit(row)"
            />
            <el-dropdown
              trigger="click"
              @command="
                (command: string) =>
                  command === 'reset' ? resetToken(row) : remove(row)
              "
              ><RowIconButton
                :icon="MoreFilled"
                :label="`${row.name} 更多操作`"
              /><template #dropdown
                ><el-dropdown-menu
                  ><el-dropdown-item command="reset">重置令牌</el-dropdown-item
                  ><el-dropdown-item command="delete"
                    ><span class="danger-text">删除合集</span></el-dropdown-item
                  ></el-dropdown-menu
                ></template
              ></el-dropdown
            >
          </div>
        </article></TransitionGroup
      >
    </div>
    <UiDrawer
      v-model="dialog"
      :title="form.id ? '编辑合集' : '新建合集'"
      size="min(760px, 100vw)"
      :close-on-click-modal="!saving"
      :close-on-press-escape="!saving"
      :show-close="!saving"
      ><UiSegmented
        v-model="section"
        :options="[
          { label: '基本信息', value: 'basic' },
          { label: '处理与到期', value: 'advanced' },
        ]"
        label="合集编辑分区"
      />
      <section v-if="editingCollection" class="collection-link">
        <h3>合集链接</h3>
        <SecretLink
          :value="collectionLink(editingCollection)"
          label="合集链接"
        />
      </section>
      <el-form
        label-position="top"
        class="collection-form"
        @submit.prevent="save"
        ><section v-show="section === 'basic'">
          <el-form-item label="名称" required
            ><el-input
              v-model="form.name"
              placeholder="填写合集名称" /></el-form-item
          ><el-form-item label="成员订阅"
            ><el-select
              v-model="form.memberList"
              multiple
              filterable
              placeholder="选择要合并的订阅"
              ><el-option
                v-for="sub in subs"
                :key="sub.ID"
                :label="sub.Name"
                :value="sub.ID" /></el-select></el-form-item
          ><EmptyState
            v-if="!subs.length"
            size="section"
            title="暂无成员订阅"
            description="先创建订阅，再加入合集。"
          /><el-form-item label="备注"
            ><el-input v-model="form.note"
          /></el-form-item>
        </section>
        <section v-show="section === 'advanced'">
          <el-form-item label="节点去重"
            ><el-switch v-model="form.dedupe" /></el-form-item
          ><el-form-item label="到期时间（留空永不过期）"
            ><el-date-picker
              v-model="form.expires"
              type="datetime"
              format="YYYY-MM-DD HH:mm"
              placeholder="选择到期时间"
              clearable /></el-form-item
          ><el-form-item label="合集处理链（可选，JSON）"
            ><el-input
              v-model="form.pipeline"
              type="textarea"
              :rows="6"
              placeholder='{"dedupe":true}'
          /></el-form-item></section></el-form
      ><template #footer
        ><UiButton :disabled="saving" @click="dialog = false">取消</UiButton
        ><UiButton variant="primary" :loading="saving" @click="save"
          >保存</UiButton
        ></template
      ></UiDrawer
    >
    <UiDialog
      v-model="confirmation.visible"
      :title="confirmation.title"
      width="480px"
      :close-on-click-modal="!deleting"
      :close-on-press-escape="!deleting"
      :show-close="!deleting"
      ><p>{{ confirmation.message }}</p>
      <template #footer
        ><UiButton :disabled="deleting" @click="confirmation.visible = false"
          >取消</UiButton
        ><UiButton variant="danger" :loading="deleting" @click="confirm">{{
          confirmation.action
        }}</UiButton></template
      ></UiDialog
    >
  </div>
</template>
<style scoped src="./subs/lists.scss"></style>
<style scoped>
.collections-page {
  min-width: 0;
}
.collection-intro,
.collection-note {
  color: var(--c-muted);
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.collection-link {
  max-width: 760px;
  margin-top: 24px;
}
.collection-form {
  margin-top: 32px;
}
.collection-form :deep(.el-select),
.collection-form :deep(.el-date-editor) {
  width: 100%;
}
</style>
