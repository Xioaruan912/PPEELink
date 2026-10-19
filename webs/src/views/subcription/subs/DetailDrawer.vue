<script setup lang="ts">
import {
  UiDrawer,
  UiSegmented,
  UiButton,
  UiSkeleton,
  StatusDot,
  EmptyState,
} from "@/ui";
import { ref } from "vue";
import IconSync from "~icons/ph/arrows-clockwise";
import { formatDateTime } from "@/utils/date";
import { sourceTrafficOf } from "@/utils/traffic";
import { syncSubSource } from "@/api/subcription/subs";
import TrafficMeter from "@/components/share/TrafficMeter.vue";
import { countryFlag } from "@/utils/flag";
import { useSubscriptions } from "./useSubscriptions";
const {
  drawerVisible,
  drawerSub,
  drawerLoading,
  drawerGroupRefs,
  drawerNodesByCountry,
  previewNodesList,
  detailSection,
  previewError,
  openDrawer,
  getsubs,
  tableData,
  parseConfig,
  rttLabel,
  expireClear,
  expirePick,
  expireSaving,
  saveExpire,
} = useSubscriptions();
// 立即刷新上游，完成后重新加载列表与当前抽屉
const syncing = ref(false);
async function syncNow() {
  const sub = drawerSub.value;
  if (!sub || syncing.value) return;
  syncing.value = true;
  try {
    const res: any = await syncSubSource(sub.ID);
    ElMessage.success(res?.msg || "已同步");
  } finally {
    syncing.value = false;
    await getsubs();
    const fresh = tableData.value.find((s) => s.ID === sub.ID);
    if (fresh) await openDrawer(fresh);
  }
}
const sections = [
  { label: "节点预览", value: "nodes" },
  { label: "模板配置", value: "config" },
  { label: "过期设置", value: "expire" },
];
</script>
<template>
  <UiDrawer
    v-model="drawerVisible"
    :title="drawerSub?.Name || '订阅详情'"
    size="min(820px, 100vw)"
    ><template v-if="drawerSub">
      <UiSegmented
        v-model="detailSection"
        :options="sections"
        label="订阅详情分区"
      />
      <section
        v-if="detailSection === 'nodes' && drawerSub.SourceURLs"
        class="upstream-box"
        aria-label="上游订阅"
      >
        <div class="upstream-head">
          <div>
            <strong>上游订阅</strong>
            <small
              >{{
                drawerSub.lastSyncAt
                  ? `上次同步 ${formatDateTime(drawerSub.lastSyncAt)}`
                  : "尚未同步"
              }}
              · 每 6 小时自动刷新</small
            >
          </div>
          <UiButton :icon="IconSync" :loading="syncing" @click="syncNow"
            >立即同步</UiButton
          >
        </div>
        <p v-if="drawerSub.lastSyncError" class="upstream-error">
          最近一次失败，正在使用上次的节点：{{ drawerSub.lastSyncError }}
        </p>
        <TrafficMeter :traffic="sourceTrafficOf(drawerSub)" />
      </section>
      <section v-if="detailSection === 'nodes'" class="form-section">
        <h3>
          包含节点 <span class="mono">{{ previewNodesList.length }}</span>
        </h3>
        <div v-if="drawerGroupRefs.length" class="group-tags">
          <span v-for="group in drawerGroupRefs" :key="group.ID"
            >{{ group.Name }} · {{ group.NodeCount }} 节点</span
          >
        </div>
        <p v-if="drawerGroupRefs.length" class="field-hint">
          关联分组内的节点变化后，订阅自动跟进。
        </p>
        <UiSkeleton v-if="drawerLoading" :lines="4" /><EmptyState
          v-else-if="previewError"
          size="section"
          title="节点加载失败"
          :description="previewError"
          ><template #action
            ><UiButton @click="openDrawer(drawerSub)"
              >重新加载</UiButton
            ></template
          ></EmptyState
        >
        <template v-else-if="drawerNodesByCountry.length"
          ><section
            v-for="group in drawerNodesByCountry"
            :key="group.country"
            class="country-group"
          >
            <h4>
              {{ countryFlag(group.items[0].countryCode) }}
              {{ group.country }} · {{ group.items.length }}
            </h4>
            <div v-for="node in group.items" :key="node.id" class="detail-node">
              <span
                >{{ node.name
                }}<small class="mono">{{ node.server }}</small></span
              ><StatusDot
                :running="node.rtt === -2"
                :state="
                  node.rtt === -2 || node.rtt === -3
                    ? 'idle'
                    : node.rtt < 0
                      ? 'bad'
                      : node.rtt < 100
                        ? 'ok'
                        : 'warn'
                "
                :label="rttLabel(node.rtt)"
              />
            </div></section></template
        ><EmptyState
          v-else
          size="section"
          title="暂无节点"
          description="编辑订阅，添加节点或关联分组。"
        />
      </section>
      <section v-if="detailSection === 'config'" class="form-section">
        <h3>模板配置</h3>
        <dl class="config-list">
          <template
            v-for="key in ['clash', 'surge', 'loon'] as const"
            :key="key"
            ><dt>{{ key }}</dt>
            <dd class="mono">
              {{ parseConfig(drawerSub.Config)[key] || "未配置" }}
            </dd></template
          >
          <dt>UDP</dt>
          <dd>{{ parseConfig(drawerSub.Config).udp ? "开启" : "关闭" }}</dd>
          <dt>跳过证书</dt>
          <dd>{{ parseConfig(drawerSub.Config).cert ? "开启" : "关闭" }}</dd>
        </dl>
      </section>
      <section v-if="detailSection === 'expire'" class="form-section">
        <h3>过期设置</h3>
        <el-form label-position="top" @submit.prevent="saveExpire"
          ><el-form-item
            ><el-checkbox
              v-model="expireClear"
              @change="
                () => {
                  if (expireClear) expirePick = '';
                }
              "
              >永不过期</el-checkbox
            ></el-form-item
          ><el-form-item label="过期时间"
            ><el-date-picker
              v-model="expirePick"
              type="datetime"
              format="YYYY-MM-DD HH:mm"
              :disabled="expireClear"
              placeholder="选择过期时间" /></el-form-item
          ><UiButton
            variant="primary"
            :loading="expireSaving"
            @click="saveExpire"
            >保存过期时间</UiButton
          ></el-form
        >
      </section>
    </template></UiDrawer
  >
</template>
<style scoped src="./forms.scss"></style>
<style scoped>
.upstream-box {
  display: grid;
  gap: 10px;
  margin: 16px 0 4px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface-2);
}
.upstream-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.upstream-head strong {
  display: block;
  font-size: 14px;
}
.upstream-head small {
  color: var(--c-muted);
  font-size: 12px;
}
.upstream-error {
  margin: 0;
  padding: 8px 10px;
  border-radius: var(--radius-control);
  background: color-mix(in srgb, var(--c-warn) 12%, transparent);
  color: var(--c-warn-ink);
  font-size: 12px;
  overflow-wrap: anywhere;
}
</style>
