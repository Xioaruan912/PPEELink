<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { countries, countryFromName, loadCountries } from "@/utils/country";

/**
 * 节点国旗：默认“自动”按名称识别（如 USA、JP、香港），识别不到时提示手动选择。
 * v-model 取值：auto / "" (不加国旗) / 国家代码。
 */
const model = defineModel<string>({ required: true });
const props = defineProps<{ name?: string; multiple?: boolean }>();
onMounted(loadCountries);

const detected = computed(() =>
  props.name ? countryFromName(props.name) : null
);
const chosen = computed(() =>
  countries.value.find((c) => c.code === model.value)
);
// 名字里认不出国家时，引导用户自己选
const needsPick = computed(
  () =>
    model.value === "auto" &&
    !props.multiple &&
    !!props.name?.trim() &&
    !detected.value
);
// 选择框里能按中文名、代码或别名搜索
function filterMethod(query: string) {
  filterText.value = query.trim().toLowerCase();
}
const filterText = ref("");
const visible = computed(() =>
  !filterText.value
    ? countries.value
    : countries.value.filter((c) =>
        [c.code, c.name, ...c.aliases].some((v) =>
          v.toLowerCase().includes(filterText.value)
        )
      )
);
</script>
<template>
  <div class="country-picker">
    <el-select
      v-model="model"
      filterable
      :filter-method="filterMethod"
      aria-label="国家/地区"
      class="picker"
      @visible-change="(open: boolean) => !open && (filterText = '')"
    >
      <el-option value="auto" label="自动（按名称识别）" />
      <el-option value="" label="不加国旗" />
      <el-option
        v-for="c in visible"
        :key="c.code"
        :value="c.code"
        :label="`${c.flag} ${c.name}`"
        ><span class="opt"
          ><span class="flag">{{ c.flag }}</span
          ><span>{{ c.name }}</span
          ><span class="code mono">{{ c.code }}</span></span
        ></el-option
      >
    </el-select>
    <p v-if="model === 'auto' && detected" class="hint ok">
      识别为 <span class="flag">{{ detected.flag }}</span>
      {{ detected.name }}，保存时加到名称前面
    </p>
    <p v-else-if="needsPick" class="hint warn" role="status">
      名称里没有识别到国家。在名称里写上代码（如
      JP、USA）或中文名，或直接在上面选一个。
    </p>
    <p v-else-if="model === 'auto' && multiple" class="hint">
      每个节点按自己的名称识别；识别不到的保持原名。
    </p>
    <p v-else-if="chosen" class="hint ok">
      保存时名称前加上 <span class="flag">{{ chosen.flag }}</span
      >，并替换已有国旗
    </p>
    <p v-else-if="model === ''" class="hint">保存时去掉名称里的国旗</p>
  </div>
</template>
<style scoped>
.country-picker {
  display: grid;
  gap: 6px;
  width: 100%;
}
.picker {
  width: 100%;
}
.opt {
  display: flex;
  align-items: center;
  gap: 8px;
}
.opt .code {
  margin-left: auto;
  color: var(--c-muted);
  font-size: 12px;
}
.flag {
  font-family:
    "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji", sans-serif;
}
.hint {
  margin: 0;
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.6;
}
.hint.ok {
  color: var(--c-ok);
}
.hint.warn {
  color: var(--c-warn-ink);
}
.mono {
  font-family: var(--font-mono);
}
</style>
