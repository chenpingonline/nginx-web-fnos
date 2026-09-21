<script setup lang="ts">
import { computed, reactive, ref, toRaw } from "vue";
import type { ProxyRule, RateLimitPolicy, RateLimitPolicyInput } from "../types";
import HelpHint from "./HelpHint.vue";

const props = defineProps<{
  policies: RateLimitPolicy[];
  rules: ProxyRule[];
  busy: boolean;
  createNew?: boolean;
}>();
const emit = defineEmits<{
  save: [value: RateLimitPolicyInput, id: string, done: (saved?: RateLimitPolicy, error?: string) => void];
  remove: [policy: RateLimitPolicy];
  close: [];
}>();

const blank = (): RateLimitPolicyInput => ({
  name: "",
  settings: {
    enabled: true,
    requests_per_second: 20,
    burst: 40,
    no_delay: true,
    connections: 0,
    download_kbps: 0,
  },
});
const message = ref("");
const messageError = ref(false);
const form = reactive<RateLimitPolicyInput>(blank());
const requestOutcome = computed(() => {
  const rate = Number(form.settings.requests_per_second) || 0;
  const burst = Number(form.settings.burst) || 0;
  if (burst <= 0) return `同一客户端 IP 平均每秒处理 ${rate} 个请求；超出平均速率的请求将返回 503。`;
  return form.settings.no_delay
    ? `同一客户端 IP 平均每秒处理 ${rate} 个请求，可额外容纳 ${burst} 个突发请求；突发容量内立即处理，容量耗尽后返回 503。`
    : `同一客户端 IP 平均每秒处理 ${rate} 个请求；超出平均速率的请求会在 ${burst} 个突发容量内排队，队列满后返回 503。`;
});
const connectionOutcome = computed(() => Number(form.settings.connections) > 0
  ? `同一客户端 IP 最多允许 ${form.settings.connections} 个正在处理的并发请求，超过后返回 503。`
  : "不限制单个客户端 IP 的并发请求数。");
const downloadOutcome = computed(() => Number(form.settings.download_kbps) > 0
  ? `每个响应的下载速度限制为 ${form.settings.download_kbps} KB/s；多个并行下载的速度会叠加。`
  : "不限制单个响应的下载速度。");

function submit() {
  if (props.busy) return;
  message.value = "";
  messageError.value = false;
  form.settings.enabled = true;
  emit("save", structuredClone(toRaw(form)), "", savedResult);
}
function savedResult(saved?: RateLimitPolicy, error?: string) {
  message.value = error ?? "";
  messageError.value = Boolean(error);
  if (saved && !error) emit("close");
}
</script>

<template>
  <div class="rate-policy-manager-layout">
    <form class="rate-policy-form form-grid modal-form-grid" @submit.prevent="submit">
      <label class="field full">
        <span>策略名称</span>
        <input v-model.trim="form.name" class="input" required maxlength="80" placeholder="例如：公开接口" />
      </label>

      <div class="form-section full"><span>请求频率</span><small>限制单个客户端 IP 的请求速度</small></div>
      <label class="field">
        <span class="rate-policy-field-label">平均请求速率<HelpHint text="同一客户端 IP 长期允许的平均请求数（次/秒）。" /></span>
        <input v-model.number="form.settings.requests_per_second" class="input" type="number" min="1" max="100000" required />
      </label>
      <label class="field">
        <span class="rate-policy-field-label">突发容量<HelpHint text="短时间超过平均速率时，最多额外容纳的请求数。" /></span>
        <input v-model.number="form.settings.burst" class="input" type="number" min="0" max="100000" required />
      </label>
      <fieldset class="rate-policy-mode full">
        <legend>超额请求处理</legend>
        <div>
          <label :class="{ selected: form.settings.no_delay }">
            <input v-model="form.settings.no_delay" type="radio" :value="true" />
            <span><strong>立即处理突发请求</strong><small>容量内不排队，容量耗尽后拒绝</small></span>
          </label>
          <label :class="{ selected: !form.settings.no_delay }">
            <input v-model="form.settings.no_delay" type="radio" :value="false" />
            <span><strong>排队平滑处理</strong><small>容量内排队，队列满后拒绝</small></span>
          </label>
        </div>
      </fieldset>

      <div class="form-section full"><span>连接与带宽</span><small>0 表示不限制对应项目</small></div>
      <label class="field">
        <span class="rate-policy-field-label">单 IP 并发连接<HelpHint text="仅统计已经开始处理请求的连接；0 表示不限制。" /></span>
        <input v-model.number="form.settings.connections" class="input" type="number" min="0" max="100000" />
      </label>
      <label class="field">
        <span class="rate-policy-field-label">单请求限速（KB/s）<HelpHint text="按单个响应计算，并行下载的速度会叠加；0 表示不限制。" /></span>
        <input v-model.number="form.settings.download_kbps" class="input" type="number" min="0" max="1048576" />
      </label>

      <section class="rate-policy-impact full" aria-live="polite">
        <strong>按当前配置，将发生什么？</strong>
        <ul><li>{{ requestOutcome }}</li><li>{{ connectionOutcome }}</li><li>{{ downloadOutcome }}</li></ul>
      </section>
      <p v-if="message" class="rate-policy-message" :class="{ error: messageError }">{{ message }}</p>

      <footer class="rate-policy-footer full">
        <span></span>
        <button type="button" class="button ghost rate-policy-footer-button" :disabled="busy" @click="emit('close')">关闭</button>
        <button type="submit" class="button primary rate-policy-save-button" :disabled="busy">{{ busy ? "保存并应用中…" : "保存并应用" }}</button>
      </footer>
    </form>
  </div>
</template>

<style scoped>
.rate-policy-manager-layout { min-height: 0; max-height: calc(100vh - 84px); overflow: auto; }
.rate-policy-form { min-width: 0; padding: 13px 14px; }
.rate-policy-form .field { display: grid; grid-template-columns: 132px minmax(0, 1fr); align-items: start; column-gap: 10px; row-gap: 4px; }
.rate-policy-form .field > span:first-child { min-height: 34px; display: flex; align-items: center; font-size: 14px; font-weight: 640; }
.rate-policy-form .field:not(.full) > .input { width: 50%; }
.rate-policy-field-label { display: inline-flex; align-items: center; gap: 6px; width: fit-content; }
.rate-policy-form .form-section > small { margin-left: 10px; }
.rate-policy-mode { min-width: 0; margin: 0; padding: 0; border: 0; }.rate-policy-mode legend { margin-bottom: 7px; padding: 0; color: var(--text); font-size: 14px; font-weight: 640; }.rate-policy-mode > div { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }.rate-policy-mode label { min-width: 0; padding: 10px 11px; display: flex; align-items: flex-start; gap: 9px; border: 1px solid var(--line-strong); border-radius: 9px; background: var(--surface-solid); cursor: pointer; transition: .15s ease; }.rate-policy-mode label:hover { border-color: var(--accent); }.rate-policy-mode label.selected { border-color: var(--accent); background: var(--accent-soft); box-shadow: 0 0 0 2px rgba(35,163,122,.08); }.rate-policy-mode input { margin: 2px 0 0; accent-color: var(--accent); }.rate-policy-mode label > span { min-width: 0; display: grid; gap: 3px; }.rate-policy-mode strong { font-size: 13px; line-height: 1.35; }.rate-policy-mode small { color: var(--text-muted); font-size: 12px; font-weight: 400; line-height: 1.4; }
.rate-policy-impact { padding: 11px 13px; border: 1px solid color-mix(in srgb, var(--accent) 28%, var(--line)); border-radius: 10px; background: var(--accent-soft); }.rate-policy-impact > strong { color: var(--accent-dark); font-size: 14px; }.rate-policy-impact ul { margin: 7px 0 0; padding-left: 19px; color: var(--text-muted); font-size: 12px; line-height: 1.55; }
.rate-policy-message { margin: 10px 0 0; color: var(--accent-dark); font-size: 13px; }.rate-policy-message.error { color: var(--danger); }
.rate-policy-footer { display: flex; align-items: center; gap: 10px; margin-top: 14px; padding-top: 14px; border-top: 1px solid var(--line); }.rate-policy-footer > span { flex: 1; }.rate-policy-footer-button { width: 86px; min-height: 34px; height: 34px; padding-inline: 10px; font-size: 13px; }.rate-policy-save-button { min-width: 104px; min-height: 34px; height: 34px; padding-inline: 12px; font-size: 13px; }
@media (max-width: 720px) { .rate-policy-manager-layout { overflow: auto; }.rate-policy-form .field,.rate-policy-mode > div { grid-template-columns: 1fr; }.rate-policy-form .field:not(.full) > .input { width: 100%; } }
</style>
