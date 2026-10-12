<script setup lang="ts">
import { onMounted, ref, computed } from "vue";
import { getAuditLogs } from "@/api/ops";

defineOptions({ name: "OpsCenter" });

const audits = ref<any[]>([]);
const loading = ref(false);

const load = async () => {
  loading.value = true;
  try {
    const res = await getAuditLogs();
    audits.value = res?.data || [];
  } finally {
    loading.value = false;
  }
};

const openStatus = () => window.open("/status", "_blank");

const todayCount = computed(() => {
  const today = new Date().toDateString();
  return audits.value.filter(
    (row) => new Date(row.createdAt).toDateString() === today
  ).length;
});

const loginCount = computed(
  () =>
    audits.value.filter((row) => String(row.path || "").includes("auth/login"))
      .length
);

const stats = computed(() => [
  { label: "总记录", value: audits.value.length },
  { label: "今日操作", value: todayCount.value, tone: "ok" as const },
  { label: "登录次数", value: loginCount.value },
]);

const methodTone = (method: string) => {
  const m = String(method || "").toUpperCase();
  if (m === "POST") return "primary";
  if (m === "DELETE") return "danger";
  if (m === "PUT" || m === "PATCH") return "warning";
  return "info";
};

onMounted(load);
</script>

<template>
  <div class="ops-page">
    <PageHero
      title="审计日志"
      desc="只记录操作者、IP、动作、路径、状态码与时间；不记录请求体和凭据。"
    >
      <template #actions>
        <el-button @click="openStatus">公开状态页</el-button>
        <el-button :loading="loading" @click="load">刷新</el-button>
      </template>
      <template #stats>
        <StatStrip :items="stats" />
      </template>
    </PageHero>

    <PanelSection title="管理操作" desc="最近的写操作与登录记录" flush>
      <div class="audit-wrap">
        <el-table
          v-loading="loading"
          :data="audits"
          size="small"
          max-height="560"
        >
          <el-table-column prop="createdAt" label="时间" min-width="170">
            <template #default="{ row }">{{
              new Date(row.createdAt).toLocaleString()
            }}</template>
          </el-table-column>
          <el-table-column prop="actor" label="操作者" min-width="120" />
          <el-table-column prop="authType" label="认证" width="90" />
          <el-table-column prop="ip" label="IP" min-width="130" />
          <el-table-column label="方法" width="90">
            <template #default="{ row }">
              <el-tag :type="methodTone(row.method)" size="small" effect="plain">{{
                row.method
              }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column
            prop="path"
            label="路径"
            min-width="260"
            show-overflow-tooltip
          />
          <el-table-column label="状态" width="80">
            <template #default="{ row }">
              <span
                class="status-code"
                :class="row.status < 400 ? 'ok' : 'bad'"
                >{{ row.status }}</span
              >
            </template>
          </el-table-column>
        </el-table>
        <EmptyState
          v-if="!loading && !audits.length"
          title="暂无审计记录"
          desc="任意管理操作或登录之后，记录会自动出现在这里。"
        />
      </div>
    </PanelSection>
  </div>
</template>

<style lang="scss" scoped>
.ops-page {
  width: min(1180px, 100%);
  margin: 0 auto;
}

.audit-wrap {
  padding: 8px 12px 12px;
}

.status-code {
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 600;

  &.ok {
    color: var(--el-color-success);
  }

  &.bad {
    color: var(--el-color-danger);
  }
}
</style>
