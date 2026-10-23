<script setup lang="ts">
import { PageHeader } from "@/ui";
import WebsiteTraffic from "@/views/subcription/servers/WebsiteTraffic.vue";
defineOptions({ name: "MonitorWebsite" });
</script>

<template>
  <div class="monitor-website">
    <PageHeader
      title="网站流量"
      description="查看流量主要去向，以及各网站在服务器上的使用情况。"
    />
    <WebsiteTraffic />
  </div>
</template>

<style scoped>
.monitor-website {
  display: grid;
  gap: 14px;
}
</style>
