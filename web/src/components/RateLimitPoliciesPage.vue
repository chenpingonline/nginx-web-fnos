<script setup lang="ts">
import { reactive, ref, toRaw } from "vue";
import {
  PhArrowClockwise,
  PhPlusCircle,
} from "@phosphor-icons/vue";
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
            <label for="policy-rps">每秒请求数</label
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
            <label for="policy-burst">突发请求数</label
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
          <div class="field full standalone-field">
            <label class="checkbox-row"
              ><input v-model="form.settings.no_delay" type="checkbox" />
              不延迟突发请求</label
            ><span class="field-help policy-burst-help"
              >关闭后，突发请求会在允许范围内排队处理。</span
            >
          </div>
          <div class="form-section full">
            <span>连接与带宽</span><small>0 表示不限制对应项目</small>
          </div>
          <div class="field">
            <label for="policy-connections">单 IP 并发连接</label
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
            <label for="policy-download">下载限速（KB/s）</label
            ><input
              id="policy-download"
              v-model.number="form.settings.download_kbps"
              class="input"
              type="number"
              min="0"
              max="1048576"
            />
          </div>

        </form>
      </div>
    </section>
  </div>
  </Teleport>
</template>
