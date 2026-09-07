<script setup lang="ts">
import AppSelect from "./AppSelect.vue";
import { computed, reactive, ref, toRaw, watch } from "vue";
import {
  PhArrowClockwise,
  PhShareNetwork,
  PhPlusCircle,
  PhX,
} from "@phosphor-icons/vue";
import type {
  CertificateMeta,
  SNIRoute,
  StreamRule,
  StreamRuleInput,
  UpstreamPool,
} from "../types";
const props = defineProps<{
  rules: StreamRule[];
  pools: UpstreamPool[];
  certificates: CertificateMeta[];
  busy: boolean;
}>();
const emit = defineEmits<{
  save: [value: StreamRuleInput, id: string];
  remove: [rule: StreamRule];
  toggle: [rule: StreamRule, enabled: boolean];
  refresh: [];
}>();
const search = ref("");
const protocolFilter = ref<"all" | "tcp" | "udp">("all");
const enabledFilter = ref<"all" | "enabled" | "disabled">("all");
const hasFilters = computed(() => Boolean(search.value || protocolFilter.value !== "all" || enabledFilter.value !== "all"));
const filteredRules = computed(() => {
  const term = search.value.trim().toLowerCase();
  const poolNames = new Map(props.pools.map(pool => [pool.id, pool.name]));
  return props.rules.filter(rule =>
    (protocolFilter.value === "all" || rule.protocol === protocolFilter.value) &&
    (enabledFilter.value === "all" || rule.enabled === (enabledFilter.value === "enabled")) &&
    (!term || [rule.name, `${rule.listen_address}:${rule.listen_port}`, `${rule.upstream_host}:${rule.upstream_port}`, poolNames.get(rule.upstream_pool_id),
      ...(rule.sni_routes ?? []).flatMap(route => [...(route.server_names ?? []), `${route.upstream_host}:${route.upstream_port}`, poolNames.get(route.upstream_pool_id)]),
    ].join(" ").toLowerCase().includes(term)),
  );
});
function resetFilters() {
  search.value = "";
  protocolFilter.value = "all";
  enabledFilter.value = "all";
}
const open = ref(false),
  editing = ref<StreamRule | null>(null),
  trustedText = ref(""),
  allowText = ref(""),
  denyText = ref("");
const blankRoute = (): SNIRoute => ({
  server_names: [],
  upstream_pool_id: "",
  upstream_host: "127.0.0.1",
  upstream_port: 443,
});
const blank = (): StreamRuleInput => ({
  name: "",
  enabled: true,
  protocol: "tcp",
  listen_address: "0.0.0.0",
  listen_port: 10000,
  upstream_pool_id: "",
  upstream_host: "127.0.0.1",
  upstream_port: 8080,
  connect_timeout_seconds: 10,
  proxy_timeout_seconds: 3600,
  udp_responses: 1,
  proxy_protocol: false,
  accept_proxy_protocol: false,
  trusted_proxies: [],
  tls_mode: "off",
  certificate_id: "",
  sni_routes: [],
  access_log: true,
  allow: [],
  deny: [],
  max_connections: 0,
});
const form = reactive<StreamRuleInput>(blank());
const routeNames = ref<string[]>([]);
function show(rule: StreamRule | null = null) {
  editing.value = rule;
  Object.assign(form, blank(), rule ? structuredClone(toRaw(rule)) : {});
  form.trusted_proxies ??= [];
  form.allow ??= [];
  form.deny ??= [];
  form.sni_routes ??= [];
  trustedText.value = form.trusted_proxies.join("\n");
  allowText.value = form.allow.join("\n");
  denyText.value = form.deny.join("\n");
  routeNames.value = form.sni_routes.map((route) =>
    (route.server_names ?? []).join(", "),
  );
  open.value = true;
}
function addRoute() {
  form.sni_routes.push(blankRoute());
  routeNames.value.push("");
}
function removeRoute(index: number) {
  form.sni_routes.splice(index, 1);
  routeNames.value.splice(index, 1);
}
function submit() {
  const value = structuredClone(toRaw(form));
  value.trusted_proxies = trustedText.value.split(/[\s,]+/).filter(Boolean);
  value.allow = allowText.value.split(/[\s,]+/).filter(Boolean);
  value.deny = denyText.value.split(/[\s,]+/).filter(Boolean);
  value.sni_routes = value.sni_routes.map((route, index) => ({
    ...route,
    server_names: (routeNames.value[index] ?? "")
      .split(/[\s,]+/)
      .filter(Boolean),
  }));
  emit("save", value, editing.value?.id ?? "");
}
watch(
  () => props.rules,
  () => {
    if (
      open.value &&
      props.rules.some(
        (rule) =>
          rule.id === editing.value?.id ||
          (!editing.value && rule.name === form.name),
      )
    )
      open.value = false;
  },
);
</script>
<template>
  <div class="toolbar">
    <div class="notice">
      四层代理独立于 HTTP 规则，适用于 SSH、数据库、MQTT、游戏服务和 HTTPS SNI
      透传。
    </div>
    <span class="spacer"></span>
    <button class="button ghost" :disabled="busy" @click="emit('refresh')">
      <PhArrowClockwise :size="16" aria-hidden="true" />刷新
    </button>
    <button class="button primary" @click="show()">
      <PhPlusCircle :size="17" aria-hidden="true" />添加 TCP/UDP 规则
    </button>
  </div>
  <div class="toolbar rule-filters" role="search" aria-label="TCP/UDP 规则筛选">
    <input v-model="search" class="input search-input" type="search" aria-label="搜索 TCP/UDP 规则" placeholder="搜索名称、监听地址、端口或目标" />
    <AppSelect v-model="protocolFilter" class="select" aria-label="TCP/UDP 协议筛选">
      <option value="all">全部协议</option><option value="tcp">TCP</option><option value="udp">UDP</option>
    </AppSelect>
    <AppSelect v-model="enabledFilter" class="select" aria-label="TCP/UDP 启用状态筛选">
      <option value="all">全部状态</option><option value="enabled">已启用</option><option value="disabled">已停用</option>
    </AppSelect>
    <button class="button ghost" :disabled="!hasFilters" @click="resetFilters">重置筛选</button>
    <span class="filter-count">{{ filteredRules.length }} / {{ rules.length }} 条</span>
  </div>
  <article class="card proxy-rule-card">
    <div v-if="filteredRules.length" class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>状态</th>
            <th>名称</th>
            <th>入口</th>
            <th>目标</th>
            <th>TLS / 能力</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="rule in filteredRules" :key="rule.id">
            <td>
              <label class="switch"
                ><input
                  type="checkbox"
                  :checked="rule.enabled"
                  @change="
                    emit(
                      'toggle',
                      rule,
                      ($event.target as HTMLInputElement).checked,
                    )
                  " /><span></span
              ></label>
            </td>
            <td>
              <div class="rule-name">{{ rule.name }}</div>
              <div class="rule-sub">{{ rule.id }}</div>
            </td>
            <td>
              <span class="badge info">{{ rule.protocol.toUpperCase() }}</span>
              {{ rule.listen_address }}:{{ rule.listen_port }}
            </td>
            <td>
              {{
                rule.upstream_pool_id
                  ? "后端服务组"
                  : `${rule.upstream_host}:${rule.upstream_port}`
              }}
            </td>
            <td>
              <div class="domain-list">
                <span class="badge neutral">{{ rule.tls_mode }}</span
                ><span v-if="rule.accept_proxy_protocol" class="badge neutral"
                  >接收 PROXY</span
                ><span v-if="rule.proxy_protocol" class="badge neutral"
                  >发送 PROXY</span
                ><span v-if="rule.sni_routes?.length" class="badge neutral"
                  >SNI {{ rule.sni_routes.length }}</span
                >
              </div>
            </td>
            <td>
              <div class="table-actions">
                <button class="button ghost small" @click="show(rule)">
                  编辑</button
                ><button
                  class="button danger-ghost small"
                  @click="emit('remove', rule)"
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
      <div class="empty-icon">⇆</div>
      <h3>{{ rules.length ? "没有匹配的规则" : "还没有 TCP/UDP 代理" }}</h3>
      <p>{{ rules.length ? "调整关键词、协议或启用状态后重试。" : "创建独立监听端口并转发到单个后端服务或 Stream 后端服务组。" }}</p>
    </div>
  </article>
  <div v-if="open" class="modal-backdrop" @mousedown.self="open = false">
    <section
      class="modal rule-modal stream-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="stream-title"
    >
      <header class="modal-header">
        <PhShareNetwork class="stream-heading-icon" :size="32" aria-hidden="true" />
        <div>
          <h2 id="stream-title">
            {{ editing ? "编辑" : "添加" }} TCP/UDP 规则
          </h2>
          <p>保存后立即应用；其他尚未应用的配置修改也会一并生效。</p>
        </div>
        <div class="rule-header-actions">
          <button type="button" class="button ghost" :disabled="busy" @click="open = false">取消</button>
          <button type="submit" form="stream-rule-form" class="button primary" :disabled="busy">
            {{ busy ? "保存并应用中…" : "保存并应用" }}
          </button>
        </div>
        <button type="button" class="icon-button modal-close" aria-label="关闭" @click="open = false">
          <PhX :size="20" aria-hidden="true" />
        </button>
      </header>
      <div class="modal-body">
        <form id="stream-rule-form" class="form-grid modal-form-grid stream-form" @submit.prevent="submit">
          <div class="stream-basics">
            <div class="field"><label for="stream-name">规则名称</label><input id="stream-name" v-model.trim="form.name" class="input" required maxlength="80" autofocus /></div>
            <div class="field"><label for="stream-protocol">协议</label><AppSelect id="stream-protocol" v-model="form.protocol" class="select"><option value="tcp">TCP</option><option value="udp">UDP</option></AppSelect></div>
            <label class="checkbox-row stream-enabled"><input v-model="form.enabled" type="checkbox" />启用</label>
          </div>
          <div class="form-section"><span>监听设置</span><small>设置接收连接的地址与端口</small></div>
          <section class="stream-settings-panel" aria-label="监听设置">
          <div class="field">
            <label>监听地址</label
            ><input
              v-model.trim="form.listen_address"
              class="input"
              required
              placeholder="0.0.0.0、:: 或指定 IP"
            />
          </div>
          <div class="field">
            <label>监听端口</label
            ><input
              v-model.number="form.listen_port"
              class="input"
              type="number"
              min="1024"
              max="65535"
              required
            />
          </div>
          <div class="field">
            <label>PROXY Protocol</label
            ><label class="checkbox-row"
              ><input
                v-model="form.accept_proxy_protocol"
                type="checkbox"
              />入口接收 PROXY Protocol</label
            ><label class="checkbox-row"
              ><input v-model="form.proxy_protocol" type="checkbox" />向后端服务发送
              PROXY Protocol</label
            >
          </div>
          <div class="field full">
            <label>可信代理 IP / CIDR</label
            ><textarea
              v-model="trustedText"
              class="textarea"
              placeholder="仅在接收 PROXY Protocol 时填写"
            ></textarea>
          </div>
          </section>
          <div class="form-section"><span>后端服务</span><small>选择连接需要转发的位置</small></div>
          <section class="stream-settings-panel" aria-label="后端服务">
          <div class="field full">
            <label>Stream 后端服务组</label
            ><AppSelect v-model="form.upstream_pool_id" class="select">
              <option value="">单个目标</option>
              <option
                v-for="pool in pools.filter(
                  (item) => item.protocol === 'stream',
                )"
                :key="pool.id"
                :value="pool.id"
              >
                {{ pool.name }}
              </option>
            </AppSelect>
          </div>
          <template v-if="!form.upstream_pool_id"
            ><div class="field">
              <label>目标主机</label
              ><input
                v-model.trim="form.upstream_host"
                class="input"
                required
              />
            </div>
            <div class="field">
              <label>目标端口</label
              ><input
                v-model.number="form.upstream_port"
                class="input"
                type="number"
                min="1"
                max="65535"
                required
              /></div
          ></template>
          </section>
          <div class="form-section"><span>连接设置</span><small>调整连接和会话的最长等待时间</small></div>
          <section class="stream-settings-panel" aria-label="连接设置">
          <div class="field">
            <label>连接超时（秒）</label
            ><input
              v-model.number="form.connect_timeout_seconds"
              class="input"
              type="number"
              min="1"
              max="600"
            />
          </div>
          <div class="field">
            <label>会话超时（秒）</label
            ><input
              v-model.number="form.proxy_timeout_seconds"
              class="input"
              type="number"
              min="1"
              max="86400"
            />
          </div>
          <div v-if="form.protocol === 'udp'" class="field">
            <label>UDP 响应次数</label
            ><input
              v-model.number="form.udp_responses"
              class="input"
              type="number"
              min="0"
              max="1000"
            />
          </div>
          </section>
          <div class="form-section"><span>TLS 设置</span><small>配置加密连接与 SNI 透传</small></div>
          <section class="stream-settings-panel" aria-label="TLS 设置">
          <div class="field">
            <label>TLS 模式</label
            ><AppSelect
              v-model="form.tls_mode"
              class="select"
              :disabled="form.protocol === 'udp'"
            >
              <option value="off">关闭</option>
              <option value="terminate">TLS 终止</option>
              <option value="passthrough">SNI 透传</option>
            </AppSelect>
          </div>
          <div v-if="form.tls_mode === 'terminate'" class="field">
            <label>证书</label
            ><AppSelect v-model="form.certificate_id" class="select" required>
              <option value="">请选择</option>
              <option
                v-for="cert in certificates"
                :key="cert.id"
                :value="cert.id"
              >
                {{ cert.name }}
              </option>
            </AppSelect>
          </div>
          <template v-if="form.tls_mode === 'passthrough'"
            ><div class="stream-subheading full">SNI 分流</div>
            <div
              v-for="(route, index) in form.sni_routes"
              :key="index"
              class="full sni-editor"
            >
              <input
                v-model="routeNames[index]"
                class="input"
                required
                placeholder="域名，多个用逗号分隔"
              /><AppSelect v-model="route.upstream_pool_id" class="select">
                <option value="">单个目标</option>
                <option
                  v-for="pool in pools.filter(
                    (item) => item.protocol === 'stream',
                  )"
                  :key="pool.id"
                  :value="pool.id"
                >
                  {{ pool.name }}
                </option></AppSelect
              ><input
                v-if="!route.upstream_pool_id"
                v-model.trim="route.upstream_host"
                class="input"
                required
                placeholder="主机"
              /><input
                v-if="!route.upstream_pool_id"
                v-model.number="route.upstream_port"
                class="input"
                type="number"
                min="1"
                max="65535"
              /><button
                type="button"
                class="button danger-ghost small"
                @click="removeRoute(index)"
              >
                删除
              </button>
            </div>
            <div class="full">
              <button
                type="button"
                class="button ghost small"
                @click="addRoute"
              >
                ＋ 添加 SNI 路由
              </button>
            </div></template
          >
          </section>
          <div class="form-section"><span>安全与日志</span><small>控制访问范围与连接记录</small></div>
          <section class="stream-settings-panel" aria-label="安全与日志">
          <div class="field">
            <label>日志与连接限制</label
            ><label class="checkbox-row"
              ><input v-model="form.access_log" type="checkbox" />记录 Stream
              访问日志</label
            ><label>单 IP 最大连接数</label
            ><input
              v-model.number="form.max_connections"
              class="input"
              type="number"
              min="0"
              max="1000000"
            />
          </div>
          <div class="field">
            <label>允许 IP / CIDR</label
            ><textarea v-model="allowText" class="textarea"></textarea>
          </div>
          <div class="field">
            <label>拒绝 IP / CIDR</label
            ><textarea v-model="denyText" class="textarea"></textarea>
          </div>
          </section>
        </form>
      </div>
    </section>
  </div>
</template>

<style scoped>
.stream-heading-icon { color: var(--accent); flex-shrink: 0; }
.stream-form { gap: 13px 48px; }
.stream-basics { grid-column: 1 / -1; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; align-items: center; gap: 44px; }
#stream-rule-form .stream-basics > .field { grid-template-columns: auto minmax(0, 1fr); gap: 14px; align-items: center; }
.stream-enabled { white-space: nowrap; }
.stream-form > .form-section { position: relative; min-height: 38px; margin-top: 5px; padding: 7px 0 6px 12px; display: flex; align-items: center; gap: 14px; border: 0; color: var(--text); }
.stream-form > .form-section::before { content: ""; position: absolute; left: 0; width: 2px; height: 20px; border-radius: 999px; background: var(--accent); }
.stream-form > .form-section::after { content: ""; height: 1px; flex: 1; background: var(--line); }
.stream-form > .form-section > span { font-size: 18px; font-weight: 660; white-space: nowrap; }
.stream-form > .form-section > small { color: var(--text-muted); font-size: 12px; font-weight: 450; }
.stream-settings-panel { grid-column: 1 / -1; min-width: 0; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px 48px; padding: 20px 24px; border: 1px solid color-mix(in srgb, var(--text) 24%, var(--line)); border-radius: 12px; background: color-mix(in srgb, var(--surface-soft) 55%, var(--surface)); }
#stream-rule-form .field { grid-template-columns: 112px minmax(0, 1fr); gap: 4px 10px; }
.stream-form .field > label:first-child { min-height: 38px; font-size: 15px; font-weight: 620; }
.stream-form .input, .stream-form .select { min-height: 38px; border-radius: 8px; font-size: 15px; }
.stream-form .textarea { min-height: 66px; border-radius: 8px; font-size: 15px; }
.stream-form .input[type="number"] { width: min(100%, 145px); }
.stream-form .checkbox-row { min-height: 38px; gap: 8px; font-size: 15px; }
.stream-form .checkbox-row input { width: 17px; height: 17px; }
.stream-form .field > .checkbox-row { grid-column: 2; }
.stream-subheading { font-size: 15px; font-weight: 620; padding-top: 8px; border-top: 1px solid var(--line); }
@media (max-width: 760px) {
  .stream-basics, .stream-settings-panel { grid-template-columns: minmax(0, 1fr); gap: 14px; }
  .stream-settings-panel { padding: 16px 12px; }
  .stream-form > .form-section { flex-wrap: wrap; gap: 8px; }
  #stream-rule-form .field { grid-template-columns: minmax(0, 1fr); }
  .stream-form .field > .checkbox-row { grid-column: 1; }
}
</style>
