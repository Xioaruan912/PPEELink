<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { countryFlag } from "@/utils/flag";
const props = defineProps<{ code?: string; label?: string }>();
const code = computed(() =>
  /^[a-z]{2}$/i.test(props.code || "") ? props.code!.toUpperCase() : "??"
);
// 国旗由自带的 Twemoji 国旗字体绘制（Windows 系统字体没有国旗）。
// 字体加载失败时（极少见）退回显示国家代码。
const supported = ref(true);
onMounted(async () => {
  try {
    await document.fonts.load(
      '17px "Twemoji Country Flags"',
      countryFlag("US")
    );
    supported.value = document.fonts.check(
      '17px "Twemoji Country Flags"',
      countryFlag("US")
    );
  } catch {
    supported.value = false;
  }
});
</script>
<template>
  <span
    class="country-flag"
    :class="{ 'is-code': !supported || countryFlag(code) === '🌐' }"
    :title="label || code"
    :aria-label="label || code"
  >
    {{ supported && countryFlag(code) !== "🌐" ? countryFlag(code) : code }}
  </span>
</template>
<style scoped>
.country-flag {
  display: inline-grid;
  place-items: center;
  flex: 0 0 36px;
  width: 36px;
  height: 28px;
  overflow: hidden;
  font-family:
    "Twemoji Country Flags", "Apple Color Emoji", "Segoe UI Emoji",
    "Noto Color Emoji", sans-serif;
  font-size: 20px;
  line-height: 1;
}
.is-code {
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font: 400 13px/1 var(--font-mono);
}
</style>
