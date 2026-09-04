<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import type {
  CertificateMeta,
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
  settings: Settings;
  certificates: CertificateMeta[];
  upstreamPools: UpstreamPool[];
  rateLimitPolicies: RateLimitPolicy[];
  busy: boolean;
}>();
const emit = defineEmits<{
  save: [value: ProxyRuleInput, applyAfter: boolean];
  cancel: [];
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
watch(
  () => props.rule,
  (rule) => {
    Object.assign(
      form,
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
    domains.value = (rule?.domains ?? []).join("\n");
  },
  { immediate: true },
);
function changeTLS() {
  if (
    !props.rule ||
    [
      props.settings.default_http_port,
      props.settings.default_https_port,
    ].includes(form.listen_port)
  )
    form.listen_port = form.tls
      ? props.settings.default_https_port
      : props.settings.default_http_port;
  if (!form.tls) form.certificate_id = "";
}
function submit() {
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
  <form class="form-grid modal-form-grid rule-form-grid" @submit.prevent="submit">
    <div class="field primary-field primary-left">
      <label for="rule-name">规则名称</label
      ><input
        id="rule-name"
        v-model.trim="form.name"
        class="input"
        required
        maxlength="80"
        autofocus
        placeholder="例如：Jellyfin"
      />
    </div>
    <div class="field primary-field primary-right">
      <label>规则状态</label
      ><label class="checkbox-row"
        ><input v-model="form.enabled" type="checkbox" /> 启用此规则</label
      >
    </div>
    <div class="form-section">
      <span>监听设置</span><small>定义访问域名、监听端口与协议</small>
    </div>
    <div class="field full section-content entry-domain-field">
      <label for="rule-domains">访问域名 / IP</label
      ><textarea
        id="rule-domains"
        v-model="domains"
        class="textarea"
        required
        placeholder="jellyfin.example.com&#10;media.example.com"
      ></textarea
      ><span class="field-help"
        >多个域名可用换行、空格或逗号分隔；使用 * 表示该端口的默认站点。</span
      >
    </div>
    <div class="field section-content section-left">
      <label for="listen-port">监听端口</label
      ><input
        id="listen-port"
        v-model.number="form.listen_port"
        class="input"
        type="number"
        min="1024"
        max="65535"
        required
      /><span class="field-help">仅允许非特权端口。</span>
    </div>
    <div class="field section-content section-right">
      <label>入口协议</label
      ><label class="checkbox-row"
        ><input v-model="form.tls" type="checkbox" @change="changeTLS" /> 启用
        HTTPS</label
      >
    </div>
    <div v-if="form.tls" class="field section-content">
      <label for="certificate">HTTPS 证书</label
      ><select
        id="certificate"
        v-model="form.certificate_id"
        class="select"
        required
      >
        <option value="">请选择证书</option>
        <option v-for="cert in certificates" :key="cert.id" :value="cert.id">
          {{ cert.name }} · {{ formatDate(cert.not_after, true) }}
        </option></select
      ><span class="field-help">没有证书时，请先到“HTTPS 证书”页面导入。</span>
    </div>
    <div v-if="form.tls" class="field section-content section-right">
      <label>HTTP/2</label
      ><label class="checkbox-row"
        ><input v-model="form.http2" type="checkbox" /> 启用 HTTP/2</label
      >
    </div>
    <div class="form-section">
      <span>后端服务</span><small>选择请求需要转发到的位置</small>
    </div>
    <div class="target-service-fields full section-content">
      <div class="field target-pool-field">
        <label for="upstream-pool">后端服务池</label
        ><select
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
          </option></select
        ><span class="field-help"
          >服务器池支持权重、备份节点、故障恢复和负载均衡。</span
        >
      </div>
      <div class="field">
        <label for="upstream-scheme">后端服务协议</label
        ><select
          id="upstream-scheme"
          v-model="form.upstream_scheme"
          class="select"
        >
          <option value="http">HTTP</option>
          <option value="https">HTTPS</option>
        </select>
      </div>
      <div v-if="!form.upstream_pool_id" class="field target-host-field">
        <label for="upstream-host">目标主机</label
        ><input
          id="upstream-host"
          v-model.trim="form.upstream_host"
          class="input"
          required
          placeholder="127.0.0.1 或 192.168.1.20"
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
    <div
      v-if="form.upstream_scheme === 'https'"
      class="field section-content section-left"
    >
      <label>后端服务证书校验</label
      ><label class="checkbox-row"
        ><input v-model="form.verify_upstream_tls" type="checkbox" /> 校验后端服务
        HTTPS 证书</label
      >
    </div>
    <div class="form-section">
      <span>代理能力</span><small>控制请求头、连接升级与传输方式</small>
    </div>
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
    <div class="form-section">
      <span>超时设置</span><small>调整连接和响应的最长等待时间</small>
    </div>
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
    <div class="form-section">
      <span>访问限流</span><small>复用限流参数，每条规则独立计算额度</small>
    </div>
    <div class="field full section-content entry-domain-field">
      <label for="rate-limit-policy">限流策略</label
      ><select
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
        </option></select
      ><span class="field-help"
        >同一策略可供多条规则复用，但每条规则分别计数、互不占用额度。</span
      >
    </div>
    <div class="form-section">
      <span>根路径与高级能力</span><small>设置缓存、重写、鉴权与内容处理</small>
    </div>
    <div class="full location-card section-content">
      <div class="location-card-title"><strong>根路径 /</strong><span>缓存、静态网站、重写、鉴权与内容处理</span></div>
      <LocationSettingsEditor :model="form.root_location" :upstream-pools="upstreamPools" root />
    </div>
    <div class="form-section section-actions">
      <span>自定义 Location</span><small>为指定路径覆盖独立代理规则</small
      ><button type="button" class="button ghost compact" @click="addLocation">
        添加路径
      </button>
    </div>
    <div
      v-if="form.locations.length === 0"
      class="empty-inline full section-content"
    >没有额外路径，所有请求使用根路径设置。</div>
    <div v-for="(location, index) in form.locations" :key="location.id" class="location-card full section-content">
      <div class="location-head">
        <input v-model.trim="location.name" class="input" placeholder="名称" required maxlength="80" />
        <select v-model="location.match" class="select"><option value="prefix">前缀</option><option value="exact">精确</option><option value="regex">正则</option></select>
        <input v-model="location.path" class="input" placeholder="/api/" required />
        <label class="checkbox-row"><input v-model="location.enabled" type="checkbox" />启用</label>
        <button type="button" class="button danger compact" @click="form.locations.splice(index, 1)">删除</button>
      </div>
      <LocationSettingsEditor :model="location.settings" :upstream-pools="upstreamPools" />
    </div>
    <footer class="modal-footer full">
      <button
        type="button"
        class="button ghost"
        :disabled="busy"
        @click="emit('cancel')"
      >
        取消</button
      ><button type="submit" class="button primary" :disabled="busy">
        {{ busy ? "处理中…" : rule ? "保存修改" : "创建规则" }}
      </button>
    </footer>
  </form>
</template>
