<script setup lang="ts">
import IconPanel from "~icons/ph/squares-four";
import IconServer from "~icons/ph/hard-drives";
import IconCloud from "~icons/ph/cloud-arrow-up";
import IconDevices from "~icons/ph/devices";
import IconArrow from "~icons/ph/arrow-right";
import IconShield from "~icons/ph/shield-check";

/** 还没有服务器时的引导：拓扑示意 + 三步说明 + 直接选择服务器类型。 */
defineProps<{ hasLocal?: boolean }>();
const emit = defineEmits<{ add: [kind: "local" | "agent"] }>();

const steps = [
  { title: "接入服务器", text: "用面板这台机器，或在其他 VPS 装 agent" },
  { title: "添加入站", text: "Reality Vision 或 XHTTP，端口自动分配" },
  { title: "给账户开通", text: "订阅里自动出现节点，按配额统计流量" },
];
</script>
<template>
  <section class="onboard cell" aria-labelledby="onboard-title">
    <div class="stage" aria-hidden="true">
      <div class="grid-bg"></div>
      <div class="topo">
        <span class="node n1"><IconPanel /><em>面板</em></span>
        <span class="wire w1"><i></i><b class="mono">spec</b></span>
        <span class="node n2 core"><IconServer /><em>xray-core</em></span>
        <span class="wire w2"><i></i><b class="mono">vless</b></span>
        <span class="node n3"><IconDevices /><em>客户端</em></span>
      </div>
      <p class="chip mono"><IconShield />SHA-256 校验 · v26.3.27</p>
    </div>

    <div class="copy">
      <h2 id="onboard-title">接入第一台服务器</h2>
      <ol class="steps">
        <li v-for="(s, i) in steps" :key="s.title" :style="{ '--i': i }">
          <span class="num mono">0{{ i + 1 }}</span>
          <span
            ><strong>{{ s.title }}</strong
            ><small>{{ s.text }}</small></span
          >
        </li>
      </ol>
    </div>

    <div class="choices">
      <button
        type="button"
        class="choice is-primary"
        :disabled="hasLocal"
        @click="emit('add', 'local')"
      >
        <IconServer class="c-icon" />
        <span
          ><strong>面板服务器</strong
          ><small>xray 装在运行面板的这台机器上</small></span
        >
        <IconArrow class="c-go" />
      </button>
      <button type="button" class="choice" @click="emit('add', 'agent')">
        <IconCloud class="c-icon" />
        <span
          ><strong>远程服务器</strong
          ><small>在其他 VPS 上一行命令装 agent</small></span
        >
        <IconArrow class="c-go" />
      </button>
    </div>
  </section>
</template>
<style scoped>
.onboard {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(0, 1fr);
  grid-template-areas:
    "stage copy"
    "stage choices";
  gap: 0;
  overflow: hidden;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  box-shadow: var(--shadow-panel), var(--edge-highlight);
}

/* ---- 拓扑舞台 ---- */
.stage {
  position: relative;
  grid-area: stage;
  display: grid;
  place-items: center;
  min-height: 300px;
  padding: 28px 20px 56px;
  overflow: hidden;
  background:
    radial-gradient(
      60% 70% at 50% 45%,
      color-mix(in srgb, var(--c-signal) 16%, transparent),
      transparent 70%
    ),
    var(--c-surface-2);
  border-right: 1px solid var(--c-line);
}
.grid-bg {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(
    color-mix(in srgb, var(--c-ink) 14%, transparent) 1px,
    transparent 1.2px
  );
  background-size: 18px 18px;
  mask-image: radial-gradient(70% 70% at 50% 50%, #000 30%, transparent 85%);
  animation: drift 24s linear infinite;
}
@keyframes drift {
  to {
    background-position: 18px 18px;
  }
}
.topo {
  position: relative;
  display: flex;
  align-items: center;
  width: min(100%, 460px);
}
.node {
  position: relative;
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 56px;
  height: 56px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
  color: var(--c-ink);
  box-shadow: var(--shadow-panel);
  animation: node-in 0.8s var(--ease-spring) both;
}
.node svg {
  width: 24px;
  height: 24px;
}
.node em {
  position: absolute;
  top: calc(100% + 10px);
  color: var(--c-muted);
  font-size: 12px;
  font-style: normal;
  white-space: nowrap;
}
.n2 {
  animation-delay: 0.12s;
}
.n3 {
  animation-delay: 0.24s;
}
.core {
  width: 72px;
  height: 72px;
  border-radius: 22px;
  border-color: transparent;
  background: var(--c-signal);
  color: var(--c-on-signal);
  box-shadow:
    0 0 0 8px var(--c-glow),
    0 18px 40px -12px color-mix(in srgb, var(--c-signal) 70%, transparent);
}
.core svg {
  width: 30px;
  height: 30px;
}
.core::after {
  position: absolute;
  inset: -8px;
  border: 1px solid var(--c-signal);
  border-radius: 28px;
  content: "";
  animation: ping 2.4s var(--ease-out) infinite;
}
@keyframes ping {
  from {
    opacity: 0.8;
    transform: scale(0.9);
  }
  to {
    opacity: 0;
    transform: scale(1.45);
  }
}
@keyframes node-in {
  from {
    opacity: 0;
    transform: translateY(14px) scale(0.8);
  }
}
.wire {
  position: relative;
  flex: 1;
  height: 2px;
  margin: 0 6px;
  border-radius: 2px;
  background: repeating-linear-gradient(
    90deg,
    color-mix(in srgb, var(--c-ink) 22%, transparent) 0 6px,
    transparent 6px 11px
  );
  background-size: 11px 2px;
  animation: dash 0.9s linear infinite;
}
.wire i {
  position: absolute;
  top: 50%;
  left: 0;
  width: 8px;
  height: 8px;
  margin-top: -4px;
  border-radius: 50%;
  background: var(--c-signal);
  box-shadow: 0 0 12px 2px var(--c-signal);
  animation: packet 2.2s var(--ease-workspace, ease-in-out) infinite;
}
.w2 i {
  animation-delay: 1.1s;
}
.wire b {
  position: absolute;
  bottom: 10px;
  left: 50%;
  color: var(--c-muted);
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  white-space: nowrap;
  transform: translateX(-50%);
}
@keyframes dash {
  to {
    background-position: 11px 0;
  }
}
@keyframes packet {
  0% {
    left: 0;
    opacity: 0;
  }
  15%,
  85% {
    opacity: 1;
  }
  100% {
    left: calc(100% - 8px);
    opacity: 0;
  }
}
.chip {
  position: absolute;
  bottom: 16px;
  left: 50%;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-pill);
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 11px;
  white-space: nowrap;
  transform: translateX(-50%);
}
.chip svg {
  width: 14px;
  height: 14px;
  color: var(--c-ok);
}

/* ---- 文案与步骤 ---- */
@media (min-width: 900px) {
  .topo {
    width: min(100%, 540px);
  }
  .node {
    width: 68px;
    height: 68px;
    border-radius: 20px;
  }
  .node svg {
    width: 28px;
    height: 28px;
  }
  .core {
    width: 92px;
    height: 92px;
    border-radius: 26px;
  }
  .core svg {
    width: 38px;
    height: 38px;
  }
}
.copy {
  grid-area: copy;
  padding: 28px 28px 8px;
}
.copy h2 {
  margin: 0 0 18px;
  font-size: 22px;
  letter-spacing: -0.02em;
}
.steps {
  position: relative;
  display: grid;
  gap: 16px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.steps::before {
  position: absolute;
  top: 14px;
  bottom: 14px;
  left: 13px;
  width: 1px;
  background: var(--c-line);
  content: "";
}
.steps li {
  position: relative;
  display: flex;
  gap: 14px;
  align-items: flex-start;
  animation: step-in 0.6s var(--ease-out) both;
  animation-delay: calc(0.2s + var(--i) * 90ms);
}
@keyframes step-in {
  from {
    opacity: 0;
    transform: translateX(-10px);
  }
}
.num {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 28px;
  height: 28px;
  border: 1px solid var(--c-line);
  border-radius: 50%;
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 11px;
}
.steps li:first-child .num {
  border-color: transparent;
  background: var(--c-ink);
  color: var(--c-surface);
}
.steps strong {
  display: block;
  font-size: 15px;
  font-weight: 600;
}
.steps small {
  display: block;
  margin-top: 2px;
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.5;
}

/* ---- 选择 ---- */
.choices {
  grid-area: choices;
  display: grid;
  gap: 10px;
  align-content: end;
  padding: 20px 28px 28px;
}
.choice {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-canvas);
  color: var(--c-ink);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition:
    translate var(--duration-medium) var(--ease-spring),
    border-color var(--duration-fast) var(--ease-out),
    background var(--duration-fast) var(--ease-out);
}
.choice:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--c-ink) 30%, transparent);
  translate: 0 -2px;
}
.choice:active:not(:disabled) {
  translate: 0 0;
  scale: 0.99;
}
.choice:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 2px;
}
.choice:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}
.choice.is-primary {
  border-color: transparent;
  background: var(--c-signal);
  color: var(--c-on-signal);
}
.choice > span {
  display: grid;
  flex: 1;
  min-width: 0;
}
.choice strong {
  font-size: 15px;
  font-weight: 600;
}
.choice small {
  color: var(--c-muted);
  font-size: 12px;
}
.choice.is-primary small {
  color: color-mix(in srgb, var(--c-on-signal) 72%, transparent);
}
.c-icon {
  flex: 0 0 auto;
  width: 22px;
  height: 22px;
}
.c-go {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
  transition: transform var(--duration-medium) var(--ease-spring);
}
.choice:hover:not(:disabled) .c-go {
  transform: translateX(3px);
}

@media (max-width: 899px) {
  .onboard {
    grid-template-columns: minmax(0, 1fr);
    grid-template-areas: "stage" "copy" "choices";
  }
  .stage {
    min-height: 220px;
    padding: 24px 16px 52px;
    border-right: 0;
    border-bottom: 1px solid var(--c-line);
  }
  .copy {
    padding: 22px 18px 4px;
  }
  .copy h2 {
    font-size: 20px;
  }
  .choices {
    padding: 18px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .grid-bg,
  .wire,
  .wire i,
  .core::after,
  .node,
  .steps li {
    animation: none;
  }
  .wire i {
    display: none;
  }
}
</style>
