<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { countryFlag } from "@/utils/flag";
const props = defineProps<{ code?: string; label?: string }>();
const code = computed(() =>
  /^[a-z]{2}$/i.test(props.code || "") ? props.code!.toUpperCase() : "??"
);
const supported = ref(false);
// Only display a flag after proving that the platform paints a colour emoji.
// Letter pairs, missing-glyph boxes and monochrome fonts use the safe code label.
onMounted(() => {
  try {
    const canvas = document.createElement("canvas");
    canvas.width = 64;
    canvas.height = 40;
    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    if (!ctx) return;
    ctx.font =
      '28px "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji", sans-serif';
    ctx.fillText(countryFlag("US"), 0, 30);
    const pixels = ctx.getImageData(0, 0, 64, 40).data;
    for (let i = 0; i < pixels.length; i += 4) {
      if (
        pixels[i + 3] > 128 &&
        Math.max(pixels[i], pixels[i + 1], pixels[i + 2]) -
          Math.min(pixels[i], pixels[i + 1], pixels[i + 2]) >
          40
      ) {
        supported.value = true;
        break;
      }
    }
  } catch {
    /* Canvas may be unavailable; country codes remain readable. */
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
  font-size: 17px;
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
