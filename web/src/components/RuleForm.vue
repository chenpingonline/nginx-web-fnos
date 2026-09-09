<script setup lang="ts">
import ListenTypePicker from "./ListenTypePicker.vue";
import AppSelect from "./AppSelect.vue";
import { computed, reactive, ref, watch } from "vue";
import type {
  CertificateMeta,
  RuleGroup,
  GroupField,
  ProxyRule,
  ProxyRuleInput,
  RateLimitPolicy,
  Settings,
  UpstreamPool,
} from "../types";
import { formatDate } from "../utils";
import LocationSettingsEditor from "./LocationSettingsEditor.vue";
import type { LocationSettings } from "../types";
const props = defineProps<{
  rule: ProxyRule | null;
  groups: RuleGroup[];
  initialGroup: string;
  settings: Settings;
  certificates: CertificateMeta[];
  upstreamPools: UpstreamPool[];
  rateLimitPolicies: RateLimitPolicy[];
  busy: boolean;
}>();
const emit = defineEmits<{
  save: [value: ProxyRuleInput, applyAfter: boolean];
}>();
const applyAfter = ref(true);
const domains = ref("");
function defaultLocation(): LocationSettings {
  return {
    backend_type: "proxy", upstream_scheme: "http", upstream_pool_id: "", upstream_host: "127.0.0.1", upstream_port: 8080,
    static_path: "/vol1/data/www", static_alias: false, index_files: ["index.html", "index.htm"], autoindex: false, expires: "", try_files: [],
    return_code: 302, return_target: "", redirect_to_https: false, rewrites: [],
    cache: { enabled: false, keys_zone_mb: 10, max_size_mb: 1024, inactive_minutes: 60, valid_seconds: 300, slice_kb: 0, use_stale: true, key: "$scheme$request_method$host$request_uri", bypass: [] },
    allow: [], deny: [], request_headers: [], response_headers: [], basic_auth: false, basic_auth_realm: "Restricted", basic_auth_file: "", auth_request: "",
    secure_link: { enabled: false, secret: "", argument: "md5" }, dav: { enabled: false, methods: ["PUT", "DELETE", "MKCOL", "COPY", "MOVE"], create_full_put_path: true, min_delete_depth: 0 },
    sub_filters: [], sub_filter_once: true, sub_filter_types: ["text/html"], addition_before: "", addition_after: "", mirror: "", mirror_request_body: false,
    ssi: false, valid_referers: [], deny_invalid_referer: false,
  };
}
const form = reactive<ProxyRuleInput>({
  name: "",
  enabled: true,
  listen_port: 9080,
  domains: [],
  tls: false,
  http2: true,
  certificate_id: "",
  upstream_scheme: "http",
  upstream_host: "127.0.0.1",
  upstream_port: 8080,
  upstream_pool_id: "",
  rate_limit_policy_id: "",
  preserve_host: true,
  websocket: true,
  streaming: true,
  verify_upstream_tls: false,
  connect_timeout_seconds: 10,
  read_timeout_seconds: 3600,
  send_timeout_seconds: 3600,
  client_max_body_mb: 0,
  rate_limit: {
    enabled: false,
    requests_per_second: 20,
    burst: 40,
    no_delay: true,
    connections: 0,
    download_kbps: 0,
  },
  root_location: defaultLocation(),
  locations: [],
});
const inheritanceOptions: { key: GroupField; label: string }[] = [
  { key: "listen_type", label: "监听类型" },
  { key: "tls", label: "入口协议" }, { key: "listen_port", label: "监听端口" },
  { key: "certificate_id", label: "证书" }, { key: "http2", label: "HTTP/2" },
];
const selectedGroup = computed(() => props.groups.find(g => g.id === form.group_id));
const inherits = (field: GroupField) => Boolean(selectedGroup.value && form.inherit_fields?.includes(field));
function syncInherited() {
  const group = selectedGroup.value;
  if (!group) return;
  if (inherits("listen_type")) form.listen_type = group.listen_type || "ipv4";
  if (inherits("tls")) form.tls = group.tls;
  if (inherits("listen_port")) form.listen_port = group.listen_port;
  if (inherits("certificate_id")) form.certificate_id = group.certificate_id;
  if (inherits("http2")) form.http2 = group.http2;
  if (!form.tls) form.certificate_id = "";
}
function changeGroup(id: string) {
  if (id === form.group_id) return;
  form.group_id = id;
  form.inherit_fields = form.group_id && !props.rule ? inheritanceOptions.map(o => o.key) : [];
  syncInherited();
}
watch(() => [form.inherit_fields, props.groups], syncInherited, { deep: true });
watch(
  () => props.rule,
  (rule) => {
    Object.assign(
      form,
      { listen_type: "ipv4" },
      rule ?? {
        name: "",
        enabled: true,
        listen_port: props.settings.default_http_port,
        domains: [],
        tls: false,
        http2: true,
        certificate_id: "",
        upstream_scheme: "http",
        upstream_host: "127.0.0.1",
        upstream_port: 8080,
        upstream_pool_id: "",
        rate_limit_policy_id: "",
        preserve_host: true,
        websocket: true,
        streaming: true,
        verify_upstream_tls: false,
        connect_timeout_seconds: 10,
        read_timeout_seconds: 3600,
        send_timeout_seconds: 3600,
        client_max_body_mb: 0,
        rate_limit: {
          enabled: false,
          requests_per_second: 20,
          burst: 40,
          no_delay: true,
          connections: 0,
          download_kbps: 0,
        },
        root_location: defaultLocation(),
        locations: [],
      },
    );
    form.rate_limit = {
      enabled: false,
      requests_per_second: 20,
      burst: 40,
      no_delay: true,
      connections: 0,
      download_kbps: 0,
      ...rule?.rate_limit,
    };
    form.root_location = rule?.root_location
      ? JSON.parse(JSON.stringify(rule.root_location))
      : defaultLocation();
    form.locations = rule?.locations
      ? JSON.parse(JSON.stringify(rule.locations))
      : [];
    form.group_id = rule?.group_id ?? (rule ? "" : props.initialGroup);
    form.inherit_fields = rule ? [...(rule.inherit_fields ?? [])] : form.group_id ? inheritanceOptions.map(o => o.key) : [];
    syncInherited();
    domains.value = (rule?.domains ?? []).join("\n");
  },
  { immediate: true },
);
function changeTLS() {
  if (
    !inherits("listen_port") && (!props.rule ||
    [
      props.settings.default_http_port,
      props.settings.default_https_port,
    ].includes(form.listen_port))
  )
    form.listen_port = form.tls
      ? props.settings.default_https_port
      : props.settings.default_http_port;
  syncInherited();
  if (!form.tls) form.certificate_id = "";
}
function parseBackendAddress(value: string) {
  const raw = value.trim();
  const hasScheme = /^https?:\/\//i.test(raw);
  // Leave plain hosts and unbracketed IPv6 unchanged.
  if (!hasScheme && !/^(?:\[[^\]]+\]|[^:/\s]+):\d+$/.test(raw)) return null;
  try {
    const url = new URL(hasScheme ? raw : `${form.upstream_scheme}://${raw}`);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.pathname !== '/' || url.search || url.hash) return null;
    const port = Number(url.port || (url.protocol === 'https:' ? 443 : 80));
    if (!url.hostname || port < 1 || port > 65535) return null;
    return { upstream_host: url.hostname.replace(/^\[|\]$/g, ''), upstream_port: port, upstream_scheme: url.protocol === 'https:' ? 'https' as const : 'http' as const };
  } catch { return null; }
}
function normalizeBackendAddress() {
  const parsed = parseBackendAddress(form.upstream_host);
  if (parsed) Object.assign(form, parsed);
}
function pasteBackendAddress(event: ClipboardEvent) {
  const parsed = parseBackendAddress(event.clipboardData?.getData('text') || '');
  if (!parsed) return;
  event.preventDefault();
  Object.assign(form, parsed);
}
function submit() {
  if (!form.upstream_pool_id) normalizeBackendAddress();
  Object.assign(form.root_location, {
    upstream_scheme: form.upstream_scheme,
    upstream_pool_id: form.upstream_pool_id,
    upstream_host: form.upstream_host,
    upstream_port: form.upstream_port,
  });
  emit(
    "save",
    { ...form, domains: domains.value.split(/[\s,]+/).filter(Boolean) },
    applyAfter.value,
  );
}
function addLocation() {
  form.locations.push({
    id: crypto.randomUUID().replaceAll("-", "").slice(0, 20),
    name: `路径 ${form.locations.length + 1}`,
    enabled: true,
    path: "/api/",
    match: "prefix",
    settings: defaultLocation(),
  });
}
</script>
<template>
  <form id="proxy-rule-form" class="form-grid modal-form-grid rule-form-grid" @submit.prevent="submit">
    <div class="rule-basics-row">
    <div class="field">
      <label for="rule-group">所属分组</label>
      <AppSelect id="rule-group" :model-value="form.group_id || ''" class="select" @update:model-value="changeGroup">
        <option value="">未分组</option><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
      </AppSelect>

    </div>
    <div class="field">
      <label for="rule-name">规则名称</label
      ><input
        id="rule-name"
        v-model.trim="form.name"
        class="input"
        required
        maxlength="80"
        autofocus
      />
    </div>
    <label class="checkbox-row rule-enabled" title="启用或停用此规则">
      <input v-model="form.enabled" type="checkbox" aria-label="启用此规则" />启用
    </label>
    </div>
      <div v-if="selectedGroup" class="field-help group-inheritance full">
        <span>继承分组设置（取消勾选可自定义）：</span>
        <label v-for="option in inheritanceOptions" :key="option.key" class="checkbox-row"><input v-model="form.inherit_fields" type="checkbox" :value="option.key" />{{ option.label }}</label>
        <span v-if="rule">移动分组时保留原配置，勾选后才使用分组默认值。</span>
      </div>
    <div class="form-section">
      <span>监听设置</span><small>定义访问域名、监听端口与协议</small>
    </div>
    <section class="rule-settings-panel" aria-label="监听设置">
    <div class="field full section-content entry-domain-field">
      <label for="rule-domains">访问域名 / IP</label
      ><textarea
        id="rule-domains"
        v-model="domains"
        class="textarea"
        required
        placeholder="www.example.com&#10;media.example.com"
      ></textarea
      ><span class="field-help"
        >多个域名可用换行、空格或逗号分隔；使用 * 表示该端口的默认站点。</span
      >
    </div>
    <div class="field full section-content"><label>监听类型</label><ListenTypePicker v-model="form.listen_type" :disabled="inherits('listen_type')" /><span class="field-help">IPv6 入口可以转发到 IPv4 后端；需放行对应端口。</span></div>
    <div class="field section-content section-left">
      <label for="listen-port">监听端口</label
      ><input
        :disabled="inherits('listen_port')"
        id="listen-port"
        v-model.number="form.listen_port"
        class="input"
        type="number"
        min="1"
        max="65535"
        required
      /><span class="field-help">支持 80、443 及 1024–65535；80/443 需确保未被 fnOS 或其他服务占用。</span>
    </div>
    <div class="field section-content section-right">
      <label>入口协议</label
      ><label class="checkbox-row"
        ><input v-model="form.tls" :disabled="inherits('tls')" type="checkbox" @change="changeTLS" /> 启用
        HTTPS</label
      >
    </div>
    <div v-if="form.tls" class="field section-content">
      <label for="certificate">SSL/TLS 证书</label
      ><AppSelect
        :disabled="inherits('certificate_id')"
        id="certificate"
        v-model="form.certificate_id"
        class="select"
        required
      >
        <option value="">请选择证书</option>
        <option v-for="cert in certificates" :key="cert.id" :value="cert.id">
          {{ cert.name }} · {{ formatDate(cert.not_after, true) }}
        </option></AppSelect
      ><span class="field-help">没有证书时，请先到“SSL/TLS 证书”页面导入。</span>
    </div>
    <div v-if="form.tls" class="field section-content section-right">
      <label>HTTP/2</label
      ><label class="checkbox-row"
        ><input v-model="form.http2" :disabled="inherits('http2')" type="checkbox" /> 启用 HTTP/2</label
      >
    </div>
    </section>
    <div class="form-section">
      <span>后端服务</span><small>选择请求需要转发到的位置</small>
    </div>
    <section class="rule-settings-panel" aria-label="后端服务">
    <div class="target-service-fields full section-content">
      <div class="field target-pool-field">
        <label for="upstream-pool">后端服务组</label
        ><AppSelect
          id="upstream-pool"
          v-model="form.upstream_pool_id"
          class="select"
        >
          <option value="">单个后端服务</option>
          <option
            v-for="pool in upstreamPools.filter(
              (item) => item.protocol === 'http',
            )"
            :key="pool.id"
            :value="pool.id"
          >
            {{ pool.name }} · {{ pool.servers.length }} 个节点
          </option></AppSelect
        ><span class="field-help"
          >后端服务组支持权重、备份节点、故障恢复和负载均衡。</span
        >
      </div>
      <div class="field">
        <label for="upstream-scheme">后端服务协议</label
        ><AppSelect
          id="upstream-scheme"
          v-model="form.upstream_scheme"
          class="select"
        >
          <option value="http">HTTP</option>
          <option value="https">HTTPS</option>
        </AppSelect>
      </div>
      <div v-if="!form.upstream_pool_id" class="field target-host-field">
        <label for="upstream-host">目标主机</label
        ><input
          id="upstream-host"
          @paste="pasteBackendAddress"
          @blur="normalizeBackendAddress"
          v-model.trim="form.upstream_host"
          class="input"
          required
          placeholder="IP、域名或 http://192.168.1.22:2330"
        /><span class="field-help">粘贴完整地址可自动填写协议和端口；手动输入后移开焦点即可识别。</span>
      </div>
      <div v-if="!form.upstream_pool_id" class="field">
        <label for="upstream-port">目标端口</label
        ><input
          id="upstream-port"
          v-model.number="form.upstream_port"
          class="input"
          type="number"
          min="1"
          max="65535"
          required
        />
      </div>
    </div>
    <div
      v-if="form.upstream_scheme === 'https'"
      class="field section-content section-left"
    >
      <label>后端服务证书校验</label
      ><label class="checkbox-row"
        ><input v-model="form.verify_upstream_tls" type="checkbox" /> 校验后端服务
        SSL/TLS 证书</label
      >
    </div>
    </section>
    <div class="form-section">
      <span>代理能力</span><small>控制请求头、连接升级与传输方式</small>
    </div>
    <section class="rule-settings-panel" aria-label="代理能力">
    <div class="field section-content section-left">
      <label>请求 Host</label
      ><label class="checkbox-row"
        ><input v-model="form.preserve_host" type="checkbox" /> 保留客户端
        Host</label
      >
    </div>
    <div class="field section-content section-right">
      <label>WebSocket</label
      ><label class="checkbox-row"
        ><input v-model="form.websocket" type="checkbox" />
        转发连接升级头</label
      >
    </div>
    <div class="field section-content section-left">
      <label>流式传输</label
      ><label class="checkbox-row"
        ><input v-model="form.streaming" type="checkbox" /> 关闭代理缓冲</label
      >
    </div>
    <div class="field section-content section-right">
      <label for="body-limit">请求体上限（MB）</label
      ><input
        id="body-limit"
        v-model.number="form.client_max_body_mb"
        class="input"
        type="number"
        min="0"
        max="102400"
      /><span class="field-help">0 表示不限制。</span>
    </div>
    </section>
    <div class="form-section">
      <span>超时设置</span><small>调整连接和响应的最长等待时间</small>
    </div>
    <section class="rule-settings-panel" aria-label="超时设置">
    <div class="field section-content section-left">
      <label>连接超时（秒）</label
      ><input
        v-model.number="form.connect_timeout_seconds"
        class="input"
        type="number"
        min="1"
        max="600"
      />
    </div>
    <div class="field section-content section-right">
      <label>读取超时（秒）</label
      ><input
        v-model.number="form.read_timeout_seconds"
        class="input"
        type="number"
        min="1"
        max="86400"
      />
    </div>
    <div class="field section-content section-left">
      <label>发送超时（秒）</label
      ><input
        v-model.number="form.send_timeout_seconds"
        class="input"
        type="number"
        min="1"
        max="86400"
      />
    </div>
    <div class="field section-content section-right">
      <label>保存方式</label
      ><label class="checkbox-row"
        ><input v-model="applyAfter" type="checkbox" /> 保存后立即应用</label
      >
    </div>
    </section>
    <div class="form-section">
      <span>访问限流</span><small>复用限流参数，每条规则独立计算额度</small>
    </div>
    <section class="rule-settings-panel" aria-label="访问限流">
    <div class="field full section-content entry-domain-field">
      <label for="rate-limit-policy">限流策略</label
      ><AppSelect
        id="rate-limit-policy"
        v-model="form.rate_limit_policy_id"
        class="select"
      >
        <option value="">不启用限流</option>
        <option
          v-for="policy in rateLimitPolicies"
          :key="policy.id"
          :value="policy.id"
        >
          {{ policy.name }} · {{ policy.settings.requests_per_second }} 请求/秒
        </option></AppSelect
      ><span class="field-help"
        >同一策略可供多条规则复用，但每条规则分别计数、互不占用额度。</span
      >
    </div>
    </section>
    <div class="form-section">
      <span>根路径与高级能力</span><small>设置缓存、重写、鉴权与内容处理</small>
    </div>
    <section class="rule-settings-panel" aria-label="根路径与高级能力">
    <div class="full location-card section-content">
      <div class="location-card-title"><strong>根路径 /</strong><span>缓存、静态网站、重写、鉴权与内容处理</span></div>
      <LocationSettingsEditor :model="form.root_location" :upstream-pools="upstreamPools" root />
    </div>
    </section>
    <div class="form-section section-actions">
      <span>自定义 Location</span><small>为指定路径覆盖独立代理规则</small
      ><button type="button" class="button ghost compact" @click="addLocation">
        添加路径
      </button>
    </div>
    <section class="rule-settings-panel" aria-label="自定义 Location">
    <div
      v-if="form.locations.length === 0"
      class="empty-inline full section-content"
    >没有额外路径，所有请求使用根路径设置。</div>
    <div v-for="(location, index) in form.locations" :key="location.id" class="location-card full section-content">
      <div class="location-head">
        <input v-model.trim="location.name" class="input" placeholder="名称" required maxlength="80" />
        <AppSelect v-model="location.match" class="select"><option value="prefix">前缀</option><option value="exact">精确</option><option value="regex">正则</option></AppSelect>
        <input v-model="location.path" class="input" placeholder="/api/" required />
        <label class="checkbox-row"><input v-model="location.enabled" type="checkbox" />启用</label>
        <button type="button" class="button danger compact" @click="form.locations.splice(index, 1)">删除</button>
      </div>
      <LocationSettingsEditor :model="location.settings" :upstream-pools="upstreamPools" />
    </div>
    </section>
  </form>
</template>

<style scoped>
.rule-enabled { align-self: center; white-space: nowrap; }
.rule-basics-row { grid-column: 1 / -1; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; gap: 44px; align-items: start; }
#proxy-rule-form .rule-basics-row > .field { grid-template-columns: auto minmax(0, 1fr); column-gap: 14px; align-items: center; }
.rule-form-grid .rule-basics-row > .field > label:first-child { font-size: 15px; font-weight: 620; white-space: nowrap; }
.rule-form-grid .rule-basics-row #rule-name { width: 100%; }
@media (max-width: 760px) { .rule-basics-row { grid-template-columns: minmax(0, 1fr); gap: 12px; } }

.rule-settings-panel {
  grid-column: 1 / -1;
  min-width: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px 48px;
  padding: 20px 24px;
  border: 1px solid color-mix(in srgb, var(--text) 24%, var(--line));
  border-radius: 12px;
  background: color-mix(in srgb, var(--surface-soft) 55%, var(--surface));
}
.rule-settings-panel > .field:not(.standalone-field) > label:first-child { min-height: 38px; font-size: 15px; font-weight: 620; }
@media (max-width: 760px) {
  .rule-settings-panel { grid-template-columns: minmax(0, 1fr); gap: 14px; padding: 16px 12px; }
}

.group-inheritance { display: flex; align-items: center; flex-wrap: wrap; gap: 10px 16px; }
.group-inheritance .checkbox-row { font-size: 14px; font-weight: 400; }
.group-inheritance > span:last-child { flex-basis: 100%; }
</style>
