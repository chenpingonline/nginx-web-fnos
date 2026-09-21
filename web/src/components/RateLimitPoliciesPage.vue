<script setup lang="ts">
import { computed, reactive, ref, toRaw } from "vue";
import {
  PhArrowClockwise,
  PhPlusCircle,
} from "@phosphor-icons/vue";
import HelpHint from "./HelpHint.vue";
import type {
  ProxyRule,
  RateLimitPolicy,
  RateLimitPolicyInput,
} from "../types";

const props = defineProps<{
  policies: RateLimitPolicy[];
  rules: ProxyRule[];
  busy: boolean;
}>();
const emit = defineEmits<{
  save: [value: RateLimitPolicyInput, id: string, done: (saved?: RateLimitPolicy, error?: string) => void];
  remove: [policy: RateLimitPolicy];
  refresh: [];
  inspect: [];
}>();
const editing = ref<RateLimitPolicy | null>(null);
const open = ref(false);
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
const form = reactive<RateLimitPolicyInput>(blank());
const requestsPerSecond = computed(() => Number(form.settings.requests_per_second) || 0);
const burst = computed(() => Number(form.settings.burst) || 0);
const connections = computed(() => Number(form.settings.connections) || 0);
const downloadKBps = computed(() => Number(form.settings.download_kbps) || 0);
const requestOutcome = computed(() => {
  const rate = requestsPerSecond.value;
  const capacity = burst.value;
  if (capacity <= 0) {
    return `同一客户端 IP 平均每秒处理 ${rate} 个请求；超出平均速率的请求将返回 503。`;
  }
  return form.settings.no_delay
    ? `同一客户端 IP 平均每秒处理 ${rate} 个请求，可额外容纳 ${capacity} 个突发请求；突发容量内立即处理，容量耗尽后返回 503。`
    : `同一客户端 IP 平均每秒处理 ${rate} 个请求；超出平均速率的请求会在 ${capacity} 个突发容量内排队，队列满后返回 503。`;
});
const connectionOutcome = computed(() => connections.value > 0
  ? `同一客户端 IP 最多允许 ${connections.value} 个正在处理的并发请求，超过后返回 503。`
  : "不限制单个客户端 IP 的并发请求数。");
const downloadOutcome = computed(() => downloadKBps.value > 0
  ? `每个响应的下载速度限制为 ${downloadKBps.value} KB/s；多个并行下载的速度会叠加。`
  : "不限制单个响应的下载速度。");

function show(policy: RateLimitPolicy | null = null) {
  saveError.value = "";
  editing.value = policy;
  Object.assign(form, policy ? structuredClone(toRaw(policy)) : blank());
  open.value = true;
}
function submit() {
  if (props.busy) return;
  saveError.value = "";
  form.settings.enabled = true;
  emit("save", structuredClone(toRaw(form)), editing.value?.id ?? "", savedResult);
}
function usageCount(id: string) {
  return props.rules.filter((rule) => rule.rate_limit_policy_id === id).length;
}
function limitSummary(policy: RateLimitPolicy) {
  const items = [`${policy.settings.requests_per_second} 请求/秒`];
  if (policy.settings.burst > 0) items.push(`突发 ${policy.settings.burst}`);
  if (policy.settings.connections > 0)
    items.push(`单 IP ${policy.settings.connections} 连接`);
  if (policy.settings.download_kbps > 0)
    items.push(`${policy.settings.download_kbps} KB/s`);
  return items.join(" · ");
}
const saveError = ref("");
function savedResult(saved?: RateLimitPolicy, error?: string) {
  if (saved) editing.value = saved;
  saveError.value = error ?? "";
  if (saved && !error) open.value = false;
}
</script>

<template>
  <div class="toolbar">
    <div class="notice">
      策略统一维护限流参数；代理规则复用策略，但请求与连接额度分别计算。
    </div>
    <span class="spacer"></span>
    <button class="button ghost" :disabled="busy" @click="emit('refresh')">
      <PhArrowClockwise :size="16" aria-hidden="true" />刷新
    </button>
    <button class="button ghost" @click="emit('inspect')">查看请求分析</button>
    <button class="button primary" @click="show()">
      <PhPlusCircle :size="17" aria-hidden="true" />添加限流策略
    </button>
  </div>
  <article class="card">
    <div v-if="policies.length" class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>策略名称</th>
            <th>请求速率</th>
            <th>其他限制</th>
            <th>使用规则</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="policy in policies" :key="policy.id">
            <td>
              <div class="rule-name">{{ policy.name }}</div>
              <div class="rule-sub">{{ policy.id }}</div>
            </td>
            <td>
              {{ policy.settings.requests_per_second }} 请求/秒
              <div class="rule-sub">
                突发 {{ policy.settings.burst }} ·
                {{ policy.settings.no_delay ? "立即处理" : "排队处理" }}
              </div>
            </td>
            <td>{{ limitSummary(policy) }}</td>
            <td>{{ usageCount(policy.id) }} 条规则</td>
            <td>
              <div class="table-actions">
                <button class="button ghost small" @click="show(policy)">
                  编辑</button
                ><button
                  class="button danger-ghost small"
                  @click="emit('remove', policy)"
                >
                  删除
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="empty-state">
      <div class="empty-icon">⏱</div>
      <h3>还没有限流策略</h3>
      <p>先创建一组限流参数，再在代理规则中选择。未选择策略的规则不受限流影响。</p>
    </div>
  </article>

  <Teleport to="body">
  <div v-if="open" class="modal-backdrop" @mousedown.self="!busy && (open = false)">
    <section
      class="modal policy-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="policy-title"
    >
      <header class="modal-header">
        <div>
          <h2 id="policy-title">{{ editing ? "编辑" : "添加" }}限流策略</h2>
          <p>保存后立即应用，其他尚未应用的配置修改也会一并生效。</p>
        </div>
        <div class="rule-header-actions">
          <button type="button" class="button ghost" :disabled="busy" @click="open = false">取消</button>
          <button type="submit" form="policy-form" class="button primary" :disabled="busy">
            {{ busy ? "保存并应用中…" : "保存并应用" }}
          </button>
        </div>
        <button class="icon-button modal-close" aria-label="关闭" :disabled="busy" @click="open = false">
          ×
        </button>
      </header>
      <div class="modal-body">
        <div v-if="saveError" class="notice danger" role="alert">{{ saveError }}</div>
        <form id="policy-form" class="form-grid modal-form-grid" @submit.prevent="submit">
          <div class="field full">
            <label for="policy-name">策略名称</label
            ><input
              id="policy-name"
              v-model.trim="form.name"
              class="input"
              required
              maxlength="80"
              autofocus
              placeholder="例如：公开接口"
            />
          </div>
          <div class="form-section full">
            <span>请求频率</span><small>限制单个客户端 IP 的请求速度</small>
          </div>
          <div class="field">
            <label for="policy-rps" class="policy-field-label">平均请求速率<HelpHint text="同一客户端 IP 长期允许的平均请求数（次/秒）。" /></label
            ><input
              id="policy-rps"
              v-model.number="form.settings.requests_per_second"
              class="input"
              type="number"
              min="1"
              max="100000"
              required
            />
          </div>
          <div class="field">
            <label for="policy-burst" class="policy-field-label">突发容量<HelpHint text="短时间超过平均速率时，最多额外容纳的请求数。" /></label
            ><input
              id="policy-burst"
              v-model.number="form.settings.burst"
              class="input"
              type="number"
              min="0"
              max="100000"
              required
            />
          </div>
          <fieldset class="policy-mode-field full">
            <legend>超额请求处理</legend>
            <div class="policy-mode-options">
              <label class="policy-mode-option" :class="{ selected: form.settings.no_delay }">
                <input v-model="form.settings.no_delay" type="radio" :value="true" />
                <span><strong>立即处理突发请求</strong><small>容量内不排队，容量耗尽后拒绝</small></span>
              </label>
              <label class="policy-mode-option" :class="{ selected: !form.settings.no_delay }">
                <input v-model="form.settings.no_delay" type="radio" :value="false" />
                <span><strong>排队平滑处理</strong><small>容量内排队，队列满后拒绝</small></span>
              </label>
            </div>
          </fieldset>
          <div class="form-section full">
            <span>连接与带宽</span><small>0 表示不限制对应项目</small>
          </div>
          <div class="field">
            <label for="policy-connections" class="policy-field-label">单 IP 并发连接<HelpHint text="仅统计已经开始处理请求的连接；0 表示不限制。" /></label
            ><input
              id="policy-connections"
              v-model.number="form.settings.connections"
              class="input"
              type="number"
              min="0"
              max="100000"
            />
          </div>
          <div class="field">
            <label for="policy-download" class="policy-field-label">单请求限速（KB/s）<HelpHint text="按单个响应计算，并行下载的速度会叠加；0 表示不限制。" /></label
            ><input
              id="policy-download"
              v-model.number="form.settings.download_kbps"
              class="input"
              type="number"
              min="0"
              max="1048576"
            />
          </div>
          <section class="policy-impact full" aria-live="polite" aria-atomic="true">
            <strong>按当前配置，将发生什么？</strong>
            <ul>
              <li>{{ requestOutcome }}</li>
              <li>{{ connectionOutcome }}</li>
              <li>{{ downloadOutcome }}</li>
            </ul>
          </section>
        </form>
      </div>
    </section>
  </div>
  </Teleport>
</template>

<style scoped>
.policy-field-label { display: inline-flex; align-items: center; gap: 6px; width: fit-content; }
.policy-modal .field:not(.full) > .input { width: 50%; }
.policy-modal .form-section > small { margin-left: 10px; }
.policy-mode-field { min-width: 0; margin: 0; padding: 0; border: 0; }
.policy-mode-field legend { margin-bottom: 7px; padding: 0; color: var(--text); font-size: 14px; font-weight: 640; }
.policy-mode-options { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.policy-mode-option { min-width: 0; padding: 10px 11px; display: flex; align-items: flex-start; gap: 9px; border: 1px solid var(--line-strong); border-radius: 9px; background: var(--surface-solid); cursor: pointer; transition: .15s ease; }
.policy-mode-option:hover { border-color: var(--accent); }
.policy-mode-option.selected { border-color: var(--accent); background: var(--accent-soft); box-shadow: 0 0 0 2px rgba(35,163,122,.08); }
.policy-mode-option input { margin: 2px 0 0; accent-color: var(--accent); }
.policy-mode-option span { min-width: 0; display: grid; gap: 3px; }
.policy-mode-option strong { font-size: 13px; line-height: 1.35; }
.policy-mode-option small { color: var(--text-muted); font-size: 12px; font-weight: 400; line-height: 1.4; }
.policy-impact { padding: 11px 13px; border: 1px solid color-mix(in srgb, var(--accent) 28%, var(--line)); border-radius: 10px; background: var(--accent-soft); }
.policy-impact > strong { color: var(--accent-dark); font-size: 14px; }
.policy-impact ul { margin: 7px 0 0; padding-left: 19px; color: var(--text-muted); font-size: 12px; line-height: 1.55; }
@media (max-width: 760px) {
  .policy-mode-options { grid-template-columns: 1fr; }
}
</style>
