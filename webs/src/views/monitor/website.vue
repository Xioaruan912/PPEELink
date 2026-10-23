<script setup lang="ts">
import { PageHeader } from "@/ui";
import WebsiteTraffic from "@/views/subcription/servers/WebsiteTraffic.vue";
</script>

<template>
  <div class="monitor-website">
    <PageHeader
      title="监控网站"
      description="只读统计代理转发的域名与字节，不含设备整机流量；默认按东八区展示。"
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
