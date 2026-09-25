<script setup lang="ts">
import ListenTypePicker from "./ListenTypePicker.vue";
import AppSelect from "./AppSelect.vue";
import { computed, nextTick, reactive, ref, watch } from "vue";
import { PhPlusCircle } from "@phosphor-icons/vue";
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
import AuthProfileManager from "./AuthProfileManager.vue";
import HelpHint from "./HelpHint.vue";
import type { AuthProfile, LocationSettings } from "../types";
const props = defineProps<{
  rule: ProxyRule | null;
  groups: RuleGroup[];
  initialGroup: string;
  settings: Settings;
  certificates: CertificateMeta[];
  upstreamPools: UpstreamPool[];
  rateLimitPolicies: RateLimitPolicy[];
  authProfiles: AuthProfile[];
  busy: boolean;
}>();
const emit = defineEmits<{
  save: [value: ProxyRuleInput, applyAfter: boolean];
  authProfilesChanged: [profiles: AuthProfile[]];
  manageRateLimits: [createNew?: boolean];
}>();
const addRateLimitValue = "__add_rate_limit_policy__";
const addAuthProfileValue = "__add_auth_profile__";
const authProfileManager = ref<InstanceType<typeof AuthProfileManager>>();
const domains = ref("");
const authError = ref("");
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
  redirect_to_https: false,
  redirect_https_port: 443,
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
  authentication: { enabled: false, mode: "basic", profile_id: "", forward_authorization: false },
  root_location: defaultLocation(),
  locations: [],
});
const activePaths = computed(() => [
  { ...form.root_location, upstream_scheme: form.upstream_scheme },
  ...form.locations.filter(location => location.enabled).map(location => location.settings),
]);
const hasHTTPProxy = computed(() => activePaths.value.some(path => path.backend_type === "proxy"));
const hasHTTPSProxy = computed(() => activePaths.value.some(path => path.backend_type === "proxy" && path.upstream_scheme === "https"));
function locationSummary(settings: LocationSettings) {
  if (settings.backend_type === "static") return `静态文件 → ${settings.static_path || "未选择目录"}`;
  if (settings.backend_type === "return") return `返回 ${settings.return_code} → ${settings.return_target || "未填写内容"}`;
  if (settings.backend_type === "status") return "连接状态";
  const pool = props.upstreamPools.find(pool => pool.id === settings.upstream_pool_id);
  const target = settings.upstream_pool_id ? (pool?.name || "转发服务组") : `${settings.upstream_host}:${settings.upstream_port}`;
  return `${settings.backend_type === "proxy" ? "反向代理" : settings.backend_type.toUpperCase()} → ${target}`;
}
const inheritanceOptions: { key: GroupField; label: string }[] = [
  { key: "listen_type", label: "监听类型" },
  { key: "tls", label: "入口协议" }, { key: "listen_port", label: "监听端口" },
  { key: "certificate_id", label: "证书" }, { key: "http2", label: "HTTP/2" },
];
const selectedGroup = computed(() => props.groups.find(g => g.id === form.group_id));
const hasExternalAuthentication = computed(() =>
  [form.root_location, ...form.locations.map(location => location.settings)]
    .some(settings => settings.basic_auth || Boolean(settings.auth_request?.trim())),
);
const rateLimitPolicySelection = computed({
  get: () => form.rate_limit_policy_id,
  set: (value: string) => {
    if (value === addRateLimitValue) {
      nextTick(() => emit("manageRateLimits", true));
      return;
    }
    form.rate_limit_policy_id = value;
  },
});
const authProfileSelection = computed({
  get: () => form.authentication.profile_id,
  set: (value: string) => {
    if (value === addAuthProfileValue) {
      nextTick(() => authProfileManager.value?.showNew());
      return;
    }
    form.authentication.profile_id = value;
    authError.value = "";
  },
});
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
        redirect_to_https: false,
        redirect_https_port: 443,
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
        authentication: { enabled: false, mode: "basic", profile_id: "", forward_authorization: false },
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
    form.authentication = {
      enabled: false,
      mode: "basic",
      profile_id: "",
      forward_authorization: false,
      ...rule?.authentication,
    };
    form.redirect_to_https = rule?.redirect_to_https ?? rule?.root_location?.redirect_to_https ?? false;
    form.redirect_https_port = rule?.redirect_https_port || 443;
    form.root_location = rule?.root_location
      ? JSON.parse(JSON.stringify(rule.root_location))
      : defaultLocation();
    form.root_location.redirect_to_https = false;
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
watch(() => form.tls, (tls) => {
  if (tls) form.redirect_to_https = false;
});
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
  if (form.authentication.enabled && !form.authentication.profile_id) {
    authError.value = "启用认证时必须选择认证策略。";
    return;
  }
  authError.value = "";
  form.root_location.redirect_to_https = false;
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
    true,
  );
}
function createLocationID() {
  if (typeof crypto !== "undefined" && typeof crypto.getRandomValues === "function") {
    const bytes = crypto.getRandomValues(new Uint8Array(10));
    return Array.from(bytes, byte => byte.toString(16).padStart(2, "0")).join("");
  }
  return `${Date.now().toString(16)}${Math.random().toString(16).slice(2)}`
    .replaceAll(".", "")
    .slice(0, 20)
    .padEnd(20, "0");
}
async function addLocation() {
  const id = createLocationID();
  form.locations.push({
    id,
    name: `路径 ${form.locations.length + 1}`,
    enabled: true,
    path: "/api/",
    match: "prefix",
    settings: defaultLocation(),
  });
  await nextTick();
  document.getElementById(`location-${id}`)?.scrollIntoView({ behavior: "smooth", block: "nearest" });
}
</script>
<template>
  <form id="proxy-rule-form" class="form-grid modal-form-grid rule-form-grid" @submit.prevent="submit">
    <div class="rule-basics-row">
    <div class="field">
      <label>所属分组</label>
      <AppSelect id="rule-group" :model-value="form.group_id || ''" class="select" aria-label="所属分组" @update:model-value="changeGroup">
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
    <div class="field full section-content"><label class="field-label-with-help">监听类型<HelpHint text="IPv6 入口可以转发到 IPv4 后端；需放行对应端口。" /></label><ListenTypePicker v-model="form.listen_type" :disabled="inherits('listen_type')" /></div>
    <div class="field full section-content entry-domain-field">
      <label for="rule-domains">访问域名 / IP</label
      ><textarea
        id="rule-domains"
        v-model="domains"
        class="textarea"
        required
        placeholder="多个域名可用换行、空格或逗号分隔；使用 * 表示该端口的默认站点。&#10;例如：www.example.com、media.example.com"
      ></textarea>
    </div>
    <div class="field section-content section-left">
      <label for="listen-port" class="field-label-with-help">监听端口<HelpHint text="仅允许非特权端口，范围 1024–65535。" /></label
      ><input
        :disabled="inherits('listen_port')"
        id="listen-port"
        v-model.number="form.listen_port"
        class="input"
        type="number"
        min="1024"
        max="65535"
        required
      />
    </div>
    <div class="field section-content section-right">
      <label>入口协议</label
      ><label class="checkbox-row"
        ><input v-model="form.tls" :disabled="inherits('tls')" type="checkbox" @change="changeTLS" /> 启用
        HTTPS</label
      >
    </div>
    <div v-if="!form.tls" class="field full section-content redirect-https-field">
      <label class="field-label-with-help">HTTPS 跳转<HelpHint text="使用 308 保留请求路径；需另有相同域名的 HTTPS 规则监听目标端口。" /></label>
      <div class="redirect-https-controls">
        <label class="checkbox-row"><input v-model="form.redirect_to_https" type="checkbox" />HTTP 自动跳转 HTTPS</label>
        <label v-if="form.redirect_to_https" class="redirect-port-control" for="redirect-https-port">
          <span>目标端口</span>
          <input id="redirect-https-port" v-model.number="form.redirect_https_port" class="input" type="number" min="1" max="65535" required />
        </label>
      </div>

    </div>
    <div v-if="form.tls" class="field section-content">
      <label class="field-label-with-help">证书<HelpHint text="没有证书时，请先到“SSL/TLS 证书”页面导入。" /></label
      ><AppSelect
        :disabled="inherits('certificate_id')"
        id="certificate"
        v-model="form.certificate_id"
        class="select"
        aria-label="SSL/TLS 证书"
        required
      >
        <option value="">请选择证书</option>
        <option v-for="cert in certificates" :key="cert.id" :value="cert.id">
          {{ cert.name }} · {{ formatDate(cert.not_after, true) }}
        </option></AppSelect
      >
    </div>
    <div v-if="form.tls" class="field section-content section-right">
      <label>HTTP/2</label
      ><label class="checkbox-row"
        ><input v-model="form.http2" :disabled="inherits('http2')" type="checkbox" /> 启用 HTTP/2</label
      >
    </div>
    </section>
    <div class="form-section">
      <span>默认路径 /</span><small>未匹配自定义路径的请求在这里处理</small>
    </div>
    <section class="rule-settings-panel" aria-label="默认路径">
      <div class="full section-content">
        <LocationSettingsEditor :model="form.root_location" :upstream-pools="upstreamPools" :rule-auth-enabled="form.authentication.enabled" root>
          <template #upstream>
    <div class="target-service-fields full section-content">
      <div class="field target-pool-field">
        <label class="field-label-with-help">转发服务组<HelpHint text="转发服务组支持权重、备份节点、故障恢复和负载均衡。" /></label
        ><AppSelect
          id="upstream-pool"
          v-model="form.upstream_pool_id"
          class="select"
          aria-label="转发服务组"
        >
          <option value="">单节点服务</option>
          <option
            v-for="pool in upstreamPools.filter(
              (item) => item.protocol === 'http',
            )"
            :key="pool.id"
            :value="pool.id"
          >
            {{ pool.name }} · {{ pool.servers.length }} 个节点
          </option></AppSelect
        >
      </div>
      <div v-if="['proxy', 'grpc'].includes(form.root_location.backend_type)" class="field">
        <label>转发服务协议</label
        ><AppSelect
          id="upstream-scheme"
          v-model="form.upstream_scheme"
          class="select"
          aria-label="转发服务协议"
        >
          <option value="http">HTTP</option>
          <option value="https">HTTPS</option>
        </AppSelect>
      </div>
      <div v-if="!form.upstream_pool_id" class="field target-host-field">
        <label for="upstream-host" class="field-label-with-help">目标主机<HelpHint text="粘贴完整地址可自动填写协议和端口。" /></label
        ><input
          id="upstream-host"
          @paste="pasteBackendAddress"
          @blur="normalizeBackendAddress"
          v-model.trim="form.upstream_host"
          class="input"
          required
          placeholder="IP、域名或 http://192.168.1.22:2330"
        />
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
          </template>
        </LocationSettingsEditor>
      </div>
    </section>
    <div class="form-section section-actions">
      <span>自定义路径</span><small>例如 /api/ 转发 Java，其他请求使用默认路径</small
      ><button type="button" class="button compact add-location-button" @click="addLocation">
        <PhPlusCircle :size="15" aria-hidden="true" />添加路径
      </button>
    </div>
    <section class="rule-settings-panel" aria-label="自定义路径">
    <div
      v-if="form.locations.length === 0"
      class="empty-inline full section-content"
    >尚未添加自定义路径，所有请求使用默认路径 /。</div>
    <div v-for="(location, index) in form.locations" :id="`location-${location.id}`" :key="location.id" class="location-card full section-content">
      <div class="path-summary"><strong>{{ location.path || "未填写路径" }}</strong><span>{{ locationSummary(location.settings) }}</span></div>
      <div class="location-head">
        <input v-model.trim="location.name" class="input" placeholder="名称" required maxlength="80" />
        <AppSelect v-model="location.match" class="select"><option value="prefix">前缀</option><option value="exact">精确</option><option value="regex">正则</option></AppSelect>
        <input v-model="location.path" class="input" placeholder="/api/" aria-label="自定义路径" required />
        <label class="checkbox-row"><input v-model="location.enabled" type="checkbox" />启用</label>
        <button type="button" class="button danger compact" @click="form.locations.splice(index, 1)">删除</button>
      </div>
      <LocationSettingsEditor :model="location.settings" :upstream-pools="upstreamPools" :rule-auth-enabled="form.authentication.enabled" />
    </div>
    </section>
    <div class="form-section"><span>规则公共设置</span><small>默认路径与自定义路径共用</small></div>
    <section class="rule-settings-panel" aria-label="规则公共设置">
    <div class="field section-content section-right body-limit-field">
      <label for="body-limit">请求体上限（MB）</label
      ><div class="body-limit-control"><input
        id="body-limit"
        v-model.number="form.client_max_body_mb"
        class="input"
        type="number"
        min="0"
        max="102400"
        :class="{ 'has-unlimited-hint': form.client_max_body_mb === 0 }"
        aria-description="0 表示不限制"
      /><span v-if="form.client_max_body_mb === 0" class="body-limit-hint" aria-hidden="true">0 表示不限制</span></div>
    </div>
      <div class="settings-subheading full">访问认证</div>

      <div class="field section-content section-left">
        <label>访问认证</label>
        <label class="checkbox-row"><input v-model="form.authentication.enabled" type="checkbox" :disabled="hasExternalAuthentication && !form.authentication.enabled" />启用用户名密码认证</label>
        <span v-if="hasExternalAuthentication && !form.authentication.enabled" class="field-help auth-mode-warning">根路径或自定义 Location 已配置外部认证，请先清除后再启用。</span>
      </div>
      <div class="field section-content section-right">
        <label class="field-label-with-help">认证策略<HelpHint text="请从下拉菜单添加认证策略。" /></label>
        <div class="auth-policy-controls">
          <AppSelect id="auth-profile" v-model="authProfileSelection" class="select" aria-label="认证策略" :disabled="busy">
            <option value="">请选择认证策略</option>
            <option v-for="profile in authProfiles" :key="profile.id" :value="profile.id">{{ profile.name }} · {{ profile.users.filter(user => user.enabled).length }} 个用户</option>
            <option :value="addAuthProfileValue" data-action>＋ 添加认证策略</option>
          </AppSelect>
          <AuthProfileManager ref="authProfileManager" :profiles="authProfiles" :disabled="busy" hide-trigger @changed="emit('authProfilesChanged', $event)" />
        </div>

        <span v-if="authError" class="field-help auth-error">{{ authError }}</span>
      </div>
      <div v-if="form.authentication.enabled" class="auth-forward-row full section-content">
        <label class="checkbox-row"><input v-model="form.authentication.forward_authorization" type="checkbox" /><span>将 Authorization 请求头继续转发给后端</span></label>
        <span class="field-help">默认不转发，避免代理认证密码泄露给后端。仅在后端明确需要该请求头时开启。</span>
      </div>
    </section>
    <div v-if="hasHTTPProxy" class="form-section"><span>反向代理设置</span><small>适用于本规则中使用 HTTP 反向代理的路径（含 HTTPS 后端）</small></div>
    <section v-if="hasHTTPProxy" class="rule-settings-panel" aria-label="反向代理设置">
    <div class="field section-content section-left">
      <label>Host 请求头</label
      ><label class="checkbox-row"
        ><input v-model="form.preserve_host" type="checkbox" /> 保留客户端
        Host</label
      >
    </div>
    <div class="field section-content section-right">
      <label>WebSocket</label
      ><label class="checkbox-row"
        ><input v-model="form.websocket" type="checkbox" />
        启用 WebSocket 支持</label
      >
    </div>
    <div class="field section-content section-left">
      <label>流式传输</label
      ><label class="checkbox-row"
        ><input v-model="form.streaming" type="checkbox" /> 关闭请求与响应缓冲</label
      >
    </div>
    <div
      v-if="hasHTTPSProxy"
      class="field section-content section-left"
    >
      <label>转发服务证书校验</label
      ><label class="checkbox-row"
        ><input v-model="form.verify_upstream_tls" type="checkbox" /> 校验转发服务
        SSL/TLS 证书</label
      >
    </div>
      <div class="settings-subheading full">后端超时</div>

    <div class="field section-content section-left">
      <label class="field-label-with-help">后端连接超时（秒）<HelpHint text="与后端服务器建立连接的等待时间。" /></label
      ><input
        v-model.number="form.connect_timeout_seconds"
        class="input"
        type="number"
        min="1"
        max="600"
      />
    </div>
    <div class="field section-content section-right">
      <label class="field-label-with-help">后端读取超时（秒）<HelpHint text="从后端读取响应时，相邻两次读取之间的最长等待时间，不是整个请求的总耗时。" /></label
      ><input
        v-model.number="form.read_timeout_seconds"
        class="input"
        type="number"
        min="1"
        max="86400"
      />
    </div>
    <div class="field section-content section-left">
      <label class="field-label-with-help">后端发送超时（秒）<HelpHint text="向后端发送请求时，相邻两次写入之间的最长等待时间，不是整个请求的总耗时。" /></label
      ><input
        v-model.number="form.send_timeout_seconds"
        class="input"
        type="number"
        min="1"
        max="86400"
      />
    </div>
      <div class="settings-subheading full">访问限流</div>

    <div class="field full section-content entry-domain-field">
      <label class="field-label-with-help">限流策略<HelpHint text="同一规则下的 HTTP 反向代理路径共享按客户端 IP 统计的请求与连接额度；不同规则独立计数。静态文件路径不受此策略控制。" /></label
      ><div class="rate-limit-policy-controls">
        <AppSelect
          id="rate-limit-policy"
          v-model="rateLimitPolicySelection"
          class="select"
          aria-label="限流策略"
        >
          <option value="">不启用限流</option>
          <option
            v-for="policy in rateLimitPolicies"
            :key="policy.id"
            :value="policy.id"
          >
            {{ policy.name }} · {{ policy.settings.requests_per_second }} 请求/秒
          </option>
          <option :value="addRateLimitValue" data-action>＋ 添加限流策略</option>
        </AppSelect>
      </div>
    </div>
    </section>
  </form>
</template>

<style scoped>
.rule-modal .rule-form-grid .rule-settings-panel[aria-label="反向代理设置"] > .field .input { width: 100%; }
.settings-subheading { border-top: 1px solid var(--line); padding-top: 12px; margin-top: 4px; font-size: 13px; font-weight: 600; color: var(--text-muted); }
.rule-modal .rule-form-grid .rule-settings-panel[aria-label="反向代理设置"] > .field:not(.standalone-field) { grid-template-columns: 168px minmax(0, 1fr); }
.body-limit-control { position: relative; width: 100%; min-width: 0; }
.rule-modal .rule-form-grid .body-limit-control .input { width: 100%; }
.rule-modal .rule-form-grid .body-limit-control .input.has-unlimited-hint { padding-right: 105px; }
.body-limit-hint { position: absolute; right: 10px; top: 50%; transform: translateY(-50%); color: var(--text-muted); font-size: 12px; pointer-events: none; }
.path-summary { display: flex; flex-wrap: wrap; gap: 8px 14px; padding: 2px 0 12px; align-items: baseline; }
.path-summary strong { color: var(--accent); }
.path-summary span { color: var(--text-muted); font-size: 13px; overflow-wrap: anywhere; }
.rule-enabled { align-self: center; white-space: nowrap; }
.field-label-with-help { gap: 6px; width: fit-content; }
.rule-basics-row { grid-column: 1 / -1; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; gap: 44px; align-items: start; }
#proxy-rule-form .rule-basics-row > .field { grid-template-columns: auto minmax(0, 1fr); column-gap: 14px; align-items: center; }
.rule-form-grid .rule-basics-row > .field > label:first-child { font-size: 15px; font-weight: 620; white-space: nowrap; }
.rule-form-grid .rule-basics-row :is(#rule-group, #rule-name) { height: 34px; min-height: 34px; }
.rule-form-grid .rule-basics-row #rule-name { width: 100%; }
@media (max-width: 760px) { .rule-basics-row { grid-template-columns: minmax(0, 1fr); gap: 12px; } }

.rule-settings-panel {
  grid-column: 1 / -1;
  min-width: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 48px;
  padding: 16px 24px;
  border: 1px solid color-mix(in srgb, var(--text) 24%, var(--line));
  border-radius: 12px;
  background: color-mix(in srgb, var(--surface-soft) 55%, var(--surface));
}
.rule-settings-panel > .field:not(.standalone-field) > label:first-child { min-height: 34px; font-size: 15px; font-weight: 620; }
@media (max-width: 760px) {
  .rule-settings-panel { grid-template-columns: minmax(0, 1fr); gap: 14px; padding: 16px 12px; }
}

.group-inheritance { display: flex; align-items: center; flex-wrap: wrap; gap: 10px 16px; }
.group-inheritance .checkbox-row { font-size: 14px; font-weight: 400; }
.group-inheritance > span:last-child { flex-basis: 100%; }
#proxy-rule-form .body-limit-field { grid-template-columns: max-content minmax(0, 1fr); }
.body-limit-field > label:first-child { white-space: nowrap; }
.add-location-button {
  min-height: 34px;
  padding-inline: 12px;
  border-color: color-mix(in srgb, var(--accent) 34%, var(--line));
  background: var(--accent-soft);
  color: var(--accent-dark);
  box-shadow: none;
}

.add-location-button:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--accent) 52%, var(--line));
  background: color-mix(in srgb, var(--accent) 14%, var(--surface));
}
.redirect-https-controls { display: flex; align-items: center; flex-wrap: wrap; gap: 10px 24px; min-height: 34px; }
.redirect-port-control { display: flex; align-items: center; gap: 9px; color: var(--text-muted); font-size: 13px; }
#proxy-rule-form .redirect-port-control .input { width: 110px; }
.rate-limit-policy-controls { width: 33.333%; min-width: 220px; display: grid; grid-template-columns: minmax(0, 1fr); gap: 10px; align-items: center; }
.auth-policy-controls { min-width: 0; display: grid; grid-template-columns: minmax(0, 1fr); gap: 10px; align-items: center; }
.auth-forward-row { display: grid; gap: 5px; padding: 12px 14px; border: 1px solid var(--line); border-radius: 9px; background: color-mix(in srgb, var(--surface-soft) 70%, var(--surface)); }
.auth-forward-row .checkbox-row { min-height: auto; font-weight: 620; }
.auth-forward-row .field-help { padding-left: 25px; }
.auth-mode-warning { color: var(--warning); }
.auth-error { color: var(--danger); }
@media (max-width: 760px) {
  .rate-limit-policy-controls { width: 100%; min-width: 0; }
  .rate-limit-policy-controls,
  .auth-policy-controls { grid-template-columns: minmax(0, 1fr); }
  .auth-forward-row .field-help { padding-left: 0; }
}
</style>
