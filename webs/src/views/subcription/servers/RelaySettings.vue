<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { getNodes, nodeBatch } from "@/api/subcription/node";
import { UiButton, UiDialog } from "@/ui";
interface RelayNode {
  ID: number;
  Name: string;
  flags?: string;
  relayId?: number;
}
const router = useRouter();
const nodes = ref<RelayNode[]>([]);
const loading = ref(false);
const busy = ref(false);
const error = ref("");
const visible = ref(false);
const targets = ref<number[]>([]);
const relayId = ref<number>();
const chains = computed(() => nodes.value.filter((n) => n.relayId));
const relays = computed(() =>
  nodes.value.filter((n) => !targets.value.includes(n.ID))
);
const name = (id?: number) =>
  nodes.value.find((n) => n.ID === id)?.Name || "节点已删除";
async function load() {
  loading.value = true;
  error.value = "";
  try {
    const res = await getNodes(true);
    nodes.value = (res.data || []).map((n: RelayNode) => {
      let relayId = 0;
      try {
        relayId = JSON.parse(n.flags || "{}").dialerProxyId || 0;
      } catch {}
      return { ...n, relayId };
    });
  } catch (e: any) {
    error.value = e?.message || "无法读取中转设置";
  } finally {
    loading.value = false;
  }
}
onMounted(load);
function edit(n?: RelayNode) {
  targets.value = n ? [n.ID] : [];
  relayId.value = n?.relayId || undefined;
  error.value = "";
  visible.value = true;
}
async function save(clear = false) {
  if (!targets.value.length || (!clear && !relayId.value)) return;
  busy.value = true;
  error.value = "";
  try {
    await nodeBatch({
      ids: targets.value,
      action: "chain",
      relayId: clear ? 0 : relayId.value,
    });
    visible.value = false;
    await load();
    ElMessage.success(
      clear
        ? "已取消中转"
        : "中转已保存，请更新客户端订阅；已有连接按原路径继续"
    );
  } catch (e: any) {
    error.value = e?.message || "保存失败";
  } finally {
    busy.value = false;
  }
}
function diagnose(n: RelayNode) {
  void router.push({
    path: "/subcription/diagnostic",
    query: { id: String(n.ID) },
  });
}
</script>
<template>
  <section id="relay-settings" class="relay-settings" aria-label="中转设置">
    <div class="relay-heading">
      <div>
        <p class="eyebrow">连接路径</p>
        <h2>中转设置</h2>
        <p>设备先连接中转，再连接落地。网站看到的是落地出口。</p>
      </div>
      <div class="relay-actions">
        <UiButton :loading="loading" @click="load">刷新</UiButton
        ><UiButton variant="primary" @click="edit()">设置中转</UiButton>
      </div>
    </div>
    <p v-if="error && !visible" class="relay-error" role="alert">{{ error }}</p>
    <p v-if="!loading && !chains.length" class="relay-empty">
      暂无中转关系。可选择一个或多个落地节点，再为它们指定入口。
    </p>
    <div v-for="n in chains" :key="n.ID" class="relay-row">
      <div class="relay-path">
        <span class="relay-role">入口</span
        ><strong>{{ name(n.relayId) }}</strong
        ><span aria-hidden="true">→</span><span class="relay-role">落地</span
        ><strong>{{ n.Name }}</strong>
      </div>
      <div class="relay-actions">
        <UiButton @click="diagnose(n)">检测中转</UiButton
        ><UiButton @click="edit(n)">修改</UiButton>
      </div>
    </div>
    <p class="relay-note">
      保存后需更新客户端订阅。Clash / Mihomo、Surge 和 Loon 完整配置
      支持此中转设置；单个分享链接不携带中转关系。检测从面板网络出发。
    </p>
    <p class="relay-note">
      Loon
      使用客户端中转时，请导入完整配置并在“中转线路”中选择。仅导入节点订阅会显示“链路底层（直连）”，这些底层节点本身没有启用中转。
    </p>
    <UiDialog v-model="visible" title="设置中转" width="560px">
      <el-form label-position="top"
        ><el-form-item label="落地节点"
          ><el-select
            v-model="targets"
            multiple
            filterable
            placeholder="选择最终出口，可多选"
            class="full"
            ><el-option
              v-for="n in nodes"
              :key="n.ID"
              :value="n.ID"
              :label="n.Name" /></el-select></el-form-item
        ><el-form-item label="中转入口"
          ><el-select
            v-model="relayId"
            filterable
            placeholder="选择设备先连接的节点"
            class="full"
            ><el-option
              v-for="n in relays"
              :key="n.ID"
              :value="n.ID"
              :label="n.Name" /></el-select></el-form-item
      ></el-form>
      <p class="relay-note">
        设备 → {{ relayId ? name(relayId) : "中转入口" }} →
        {{
          targets.length === 1
            ? name(targets[0])
            : `${targets.length} 个落地节点`
        }}。保存不会更换落地节点的出口。
      </p>
      <p v-if="error" class="relay-error" role="alert">{{ error }}</p>
      <template #footer
        ><UiButton :disabled="busy" @click="visible = false">取消</UiButton
        ><UiButton
          v-if="targets.some((id) => nodes.find((n) => n.ID === id)?.relayId)"
          :loading="busy"
          @click="save(true)"
          >取消中转</UiButton
        ><UiButton
          variant="primary"
          :loading="busy"
          :disabled="!targets.length || !relayId || targets.includes(relayId)"
          @click="save()"
          >保存中转</UiButton
        ></template
      >
    </UiDialog>
  </section>
</template>
<style scoped>
.relay-settings {
  padding: 24px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel, 20px);
  background: var(--c-surface);
  margin-block: 24px;
}
.relay-heading,
.relay-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}
.relay-heading {
  margin-bottom: 20px;
}
.relay-heading h2 {
  margin: 4px 0 8px;
  font-size: 22px;
}
.relay-heading p,
.relay-note,
.relay-empty {
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.7;
}
.eyebrow {
  font-size: 11px !important;
  letter-spacing: 0.12em;
}
.relay-row {
  padding: 16px 0;
  border-top: 1px solid var(--c-line);
}
.relay-path {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  min-width: 0;
  font-size: 13px;
  overflow-wrap: anywhere;
}
.relay-role {
  color: var(--c-muted);
  font-size: 11px;
}
.relay-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  flex-shrink: 0;
}
.relay-note {
  margin-top: 14px;
}
.relay-error {
  color: var(--c-bad);
  font-size: 13px;
}
.full {
  width: 100%;
}
@media (max-width: 599px) {
  .relay-settings {
    padding: 18px;
  }
  .relay-heading,
  .relay-row {
    align-items: flex-start;
    flex-direction: column;
    gap: 14px;
  }
  .relay-path {
    gap: 7px;
  }
  .relay-actions {
    width: 100%;
  }
  .relay-actions > * {
    flex: 1;
  }
}
</style>
