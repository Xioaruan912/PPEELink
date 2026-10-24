<script setup lang="ts">
defineProps<{
  section: "subscriptions" | "nodes";
  title: string;
  description: string;
}>();
</script>
<template>
  <header class="subscription-heading">
    <div class="heading-line">
      <div>
        <h1>{{ title }}</h1>
        <p>{{ description }}</p>
      </div>
      <div class="heading-actions"><slot name="actions" /></div>
    </div>
    <nav aria-label="订阅与节点">
      <RouterLink
        to="/subcription/subs"
        :aria-current="section === 'subscriptions' ? 'page' : undefined"
        >订阅管理<span>生成与分发</span></RouterLink
      ><RouterLink
        to="/subcription/nodes"
        :aria-current="section === 'nodes' ? 'page' : undefined"
        >节点管理<span>连接与分组</span></RouterLink
      >
    </nav>
  </header>
</template>
<style scoped>
.subscription-heading {
  margin-bottom: 22px;
  min-width: 0;
}
.heading-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  flex-wrap: wrap;
  padding: 8px 0 22px;
}
h1 {
  font: 600 30px/1.2 var(--font-display);
  letter-spacing: -0.04em;
  margin: 0 0 9px;
}
p {
  font-size: 13px;
  color: var(--c-muted);
  margin: 0;
  line-height: 1.6;
}
.heading-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
nav {
  display: flex;
  gap: 26px;
  border-bottom: 1px solid var(--c-line);
}
nav a {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 0;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
  font-weight: 600;
  font-size: 14px;
  color: var(--c-muted);
  transition:
    color 0.16s,
    border-color 0.16s;
}
nav a[aria-current="page"] {
  border-color: var(--c-signal-text);
  color: var(--c-ink);
}
nav span {
  font-size: 11px;
  font-weight: 400;
  color: var(--c-muted);
}
a:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 3px;
}
@media (max-width: 600px) {
  h1 {
    font-size: 26px;
  }
  .heading-line {
    padding-bottom: 16px;
    gap: 14px;
  }
  .heading-actions {
    width: 100%;
  }
  .heading-actions :deep(button) {
    min-height: 44px;
    flex: 1;
    padding: 8px 12px;
  }
  nav {
    gap: 24px;
  }
  nav a {
    min-height: 48px;
  }
  nav span {
    display: none;
  }
  .subscription-heading {
    margin-bottom: 16px;
  }
}
@media (prefers-reduced-motion: reduce) {
  nav a {
    transition: none;
  }
}
</style>
