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
import { countryFlag } from "@/utils/flag";
import { useSubscriptions } from "./useSubscriptions";
const {
  drawerVisible,
  drawerSub,
  drawerLoading,
  drawerGroupRefs,
  drawerNodesByCountry,
  previewNodesList,
  previewExcluded,
  detailSection,
  previewError,
  openDrawer,
  parseConfig,
  validLabel,
  expireClear,
  expirePick,
  expireSaving,
  saveExpire,
} = useSubscriptions();
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
      <section v-if="detailSection === 'nodes'" class="form-section">
        <h3>
          包含节点 <span class="mono">{{ previewNodesList.length }}</span>
        </h3>
        <div v-if="previewExcluded.length" class="excluded-box">
          <strong
            >暂不下发
            <span class="mono">{{ previewExcluded.length }}</span></strong
          >
          <ul>
            <li v-for="item in previewExcluded" :key="item.id">
              <span>{{ item.name }}</span
              ><small
                >{{ item.reason
                }}{{
                  item.until ? `，${item.until.slice(5, 10)} 恢复` : ""
                }}</small
              >
            </li>
          </ul>
        </div>
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
                :state="
                  node.valid === undefined ? 'idle' : node.valid ? 'ok' : 'bad'
                "
                :label="validLabel(node)"
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
.excluded-box {
  display: grid;
  gap: 6px;
  margin: 0 0 12px;
  padding: 12px 14px;
  border-radius: var(--radius-panel);
  background: color-mix(in srgb, var(--c-warn) 10%, transparent);
  font-size: 13px;
}
.excluded-box ul {
  display: grid;
  gap: 4px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.excluded-box li {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.excluded-box small {
  color: var(--c-warn-ink);
  text-align: right;
}
</style>
