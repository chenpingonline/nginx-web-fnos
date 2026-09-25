<script setup lang="ts">
import { onBeforeUnmount, ref } from "vue";
import { TrimApp } from "@trimjs/web-app";
import AppSelect from "./AppSelect.vue";
import type { LocationSettings, UpstreamPool } from "../types";

const props = defineProps<{
  model: LocationSettings;
  upstreamPools: UpstreamPool[];
  root?: boolean;
  ruleAuthEnabled?: boolean;
}>();

const list = (value: string) =>
  value
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean);
const setList = (key: "index_files" | "try_files" | "allow" | "deny" | "sub_filter_types" | "valid_referers", event: Event) => {
  props.model[key] = list((event.target as HTMLInputElement).value);
};
const setCacheBypass = (event: Event) => { props.model.cache.bypass = list((event.target as HTMLInputElement).value); };
const addRewrite = () => props.model.rewrites.push({ pattern: "^/(.*)$", replacement: "/$1", flag: "last" });
const addHeader = (kind: "request_headers" | "response_headers") =>
  props.model[kind].push({ name: "", value: "", always: kind === "response_headers" });
const setDavMethods = (event: Event) => {
  props.model.dav.methods = list((event.target as HTMLInputElement).value).map((item) => item.toUpperCase());
};
const changeBackend = () => { if (props.model.backend_type !== "static") props.model.dav.enabled = false; };
const pickingDirectory = ref(false);
const openingAuthorization = ref(false);
const directoryError = ref("");
let disposed = false;
onBeforeUnmount(() => { disposed = true; });
async function selectDirectory() { await directoryAction("select"); }
async function openAuthorization() { await directoryAction("authorize"); }
async function directoryAction(action: "select" | "authorize") {
  if (pickingDirectory.value || openingAuthorization.value) return;
  const busy = action === "select" ? pickingDirectory : openingAuthorization;
  busy.value = true;
  directoryError.value = "";
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    const sdk = new TrimApp();
    await Promise.race([
      sdk.ready(),
      new Promise<never>((_, reject) => {
        timer = setTimeout(() => reject(new Error("无法连接飞牛，请从飞牛桌面打开应用；也可在应用中心进入 nginx-web 设置。")), 5000);
      }),
    ]);
    clearTimeout(timer);
    if (disposed) return;
    if (sdk.isStandaloneWeb) throw new Error(action === "authorize"
      ? "请在飞牛应用中心打开 nginx-web 的设置，进入“访问权限”添加目录授权。"
      : "请从飞牛桌面打开应用后选择 NAS 目录，或手动填写目录。");
    if (action === "authorize") {
      await sdk.openAppSetting();
      return;
    }
    const paths = await sdk.pickFile({ directory: true, multiple: false, title: "选择静态网站目录", okText: "选择目录" });
    if (disposed || props.model.backend_type !== "static") return;
    const path = paths?.[0];
    if (!path) return; // Cancellation preserves the current directory.
    if (!path.startsWith("/") || /[\0\r\n]/.test(path)) throw new Error("选择器未返回有效的 NAS 绝对路径，请手动填写目录。");
    props.model.static_path = path;
  } catch (error) {
    if (!disposed) directoryError.value = error instanceof Error ? error.message : (action === "authorize" ? "无法打开应用设置，请在飞牛应用中心进入 nginx-web 的访问权限页面。" : "目录选择失败，请重试或手动填写目录。");
  } finally {
    clearTimeout(timer);
    if (!disposed) busy.value = false;
  }
}
</script>

<template>
  <div class="location-settings">
    <div class="field">
      <label>处理方式</label>
      <AppSelect v-model="model.backend_type" class="select" @change="changeBackend">
        <option value="proxy">HTTP 反向代理</option>
        <option value="static">静态文件</option>
        <option value="return">固定返回 / 跳转</option>
        <option value="grpc">gRPC</option>
        <option value="fastcgi">FastCGI</option>
        <option value="uwsgi">uWSGI</option>
        <option value="scgi">SCGI</option>
        <option value="memcached">Memcached</option>
        <option value="status">连接状态</option>
      </AppSelect>
    </div>
    <template v-if="!root && ['proxy', 'grpc', 'fastcgi', 'uwsgi', 'scgi', 'memcached'].includes(model.backend_type)">
      <div class="field">
        <label>转发服务组</label>
        <AppSelect v-model="model.upstream_pool_id" class="select">
          <option value="">单个服务器</option>
          <option v-for="pool in upstreamPools.filter((item) => item.protocol === 'http')" :key="pool.id" :value="pool.id">{{ pool.name }}</option>
        </AppSelect>
      </div>
      <div v-if="model.backend_type === 'proxy' || model.backend_type === 'grpc'" class="field">
        <label>协议</label>
        <AppSelect v-model="model.upstream_scheme" class="select"><option value="http">HTTP</option><option value="https">HTTPS</option></AppSelect>
      </div>
      <template v-if="!model.upstream_pool_id">
        <div class="field target-host-field"><label>目标主机</label><input v-model.trim="model.upstream_host" class="input" required /></div>
        <div class="field"><label>目标端口</label><input v-model.number="model.upstream_port" class="input" type="number" min="1" max="65535" required /></div>
      </template>
    </template>

    <template v-if="model.backend_type === 'static'">
      <div class="field full">
        <label>静态目录</label>
        <div class="static-directory-control">
          <input v-model.trim="model.static_path" class="input" aria-label="静态目录" placeholder="/vol1/data/www" required />
          <button type="button" class="button" :disabled="pickingDirectory || openingAuthorization" @click="selectDirectory">{{ pickingDirectory ? "选择中…" : "选择目录" }}</button>
        </div>
        <span class="field-help">选择 NAS 上的目录；请在应用设置的“访问权限”中授权。<button type="button" class="directory-authorization-link" :disabled="pickingDirectory || openingAuthorization" @click="openAuthorization">{{ openingAuthorization ? "打开中…" : "前往授权" }}</button></span>
        <span v-if="directoryError" class="field-error" role="alert">{{ directoryError }}</span>
      </div>
      <label class="checkbox-row field"><input v-model="model.static_alias" type="checkbox" /> 使用 alias（否则 root）</label>
      <label class="checkbox-row field"><input v-model="model.autoindex" type="checkbox" /> 开启目录浏览</label>
      <div class="field"><label>首页文件</label><input class="input" :value="model.index_files.join(', ')" @change="setList('index_files', $event)" /></div>
      <div class="field"><label>Try Files</label><input class="input" :value="model.try_files.join(', ')" placeholder="$uri, $uri/, /index.html" @change="setList('try_files', $event)" /></div>
      <div class="field"><label>缓存头 Expires</label><input v-model.trim="model.expires" class="input" placeholder="7d / max / off" /></div>
    </template>
    <template v-if="model.backend_type === 'return'">
      <div class="field"><label>状态码</label><input v-model.number="model.return_code" class="input" type="number" min="200" max="599" /></div>
      <div class="field"><label>返回目标 / 内容</label><input v-model="model.return_target" class="input" placeholder="https://example.com$request_uri" /></div>
    </template>

    <details v-if="model.backend_type === 'proxy'" class="advanced-box full">
      <summary>缓存与大文件切片</summary>
      <div class="form-grid compact-grid">
        <label class="checkbox-row field full"><input v-model="model.cache.enabled" type="checkbox" /> 启用代理缓存</label>
        <template v-if="model.cache.enabled">
          <div class="field"><label>缓存区（MB）</label><input v-model.number="model.cache.keys_zone_mb" class="input" type="number" min="1" /></div>
          <div class="field"><label>磁盘上限（MB）</label><input v-model.number="model.cache.max_size_mb" class="input" type="number" min="1" /></div>
          <div class="field"><label>未访问失效（分钟）</label><input v-model.number="model.cache.inactive_minutes" class="input" type="number" min="1" /></div>
          <div class="field"><label>响应有效期（秒）</label><input v-model.number="model.cache.valid_seconds" class="input" type="number" min="1" /></div>
          <div class="field"><label>切片大小（KB）</label><input v-model.number="model.cache.slice_kb" class="input" type="number" min="0" /><span class="field-help">0 表示不开启切片。</span></div>
          <label class="checkbox-row field"><input v-model="model.cache.use_stale" type="checkbox" /> 故障时使用过期缓存</label>
          <div class="field full"><label>缓存 Key</label><input v-model="model.cache.key" class="input" /></div>
          <div class="field full"><label>绕过缓存变量</label><input class="input" :value="model.cache.bypass.join(', ')" placeholder="$cookie_nocache, $arg_nocache" @change="setCacheBypass" /></div>
        </template>
      </div>
    </details>

    <details class="advanced-box full access-control-box">
      <summary>路径重写与访问控制</summary>
      <div class="form-grid compact-grid">
        <div class="field"><label>允许 IP/CIDR</label><textarea class="textarea small access-list-textarea" :value="model.allow.join('\n')" @change="setList('allow', $event)"></textarea></div>
        <div class="field"><label>拒绝 IP/CIDR</label><textarea class="textarea small access-list-textarea" :value="model.deny.join('\n')" @change="setList('deny', $event)"></textarea></div>
        <div v-for="(item, index) in model.rewrites" :key="index" class="inline-editor full">
          <input v-model="item.pattern" class="input" placeholder="正则" /><input v-model="item.replacement" class="input" placeholder="目标" />
          <AppSelect v-model="item.flag" class="select rewrite-flag-select"><option value="last">last</option><option value="break">break</option><option value="redirect">302</option><option value="permanent">301</option></AppSelect>
          <button type="button" class="button danger compact" @click="model.rewrites.splice(index, 1)">删除</button>
        </div>
        <button type="button" class="button ghost compact fit" @click="addRewrite">添加 Rewrite</button>
      </div>
    </details>

    <details class="advanced-box full security-settings-box">
      <summary>鉴权、安全与 WebDAV</summary>
      <div class="form-grid compact-grid">
        <p v-if="ruleAuthEnabled" class="field-help full auth-mode-note">已启用规则级“访问认证”，外部 htpasswd 和 Auth Request 已停用。</p>
        <div class="security-toggle-row full">
          <label class="checkbox-row"><input v-model="model.basic_auth" type="checkbox" :disabled="ruleAuthEnabled" /> 外部 htpasswd（兼容）</label>
          <label class="checkbox-row"><input v-model="model.secure_link.enabled" type="checkbox" /> Secure Link</label>
        </div>
        <div class="field full auth-request-field"><label>外部 Auth Request URI</label><input v-model.trim="model.auth_request" class="input" placeholder="/_auth" :disabled="ruleAuthEnabled" /><span class="field-help">仅在已有外部鉴权服务时使用；Nginx 会先请求该 URI，并根据返回状态决定是否放行。普通用户名密码认证请使用上方“访问认证”。</span></div>
        <div v-if="model.basic_auth" class="field"><label>认证提示</label><input v-model="model.basic_auth_realm" class="input" :disabled="ruleAuthEnabled" /></div>
        <div v-if="model.basic_auth" class="field full"><label>htpasswd 绝对路径</label><input v-model.trim="model.basic_auth_file" class="input" placeholder="/vol1/.../.htpasswd" :disabled="ruleAuthEnabled" required /></div>
        <div v-if="model.secure_link.enabled" class="field"><label>签名参数</label><input v-model.trim="model.secure_link.argument" class="input" /></div>
        <div v-if="model.secure_link.enabled" class="field"><label>签名密钥</label><input v-model="model.secure_link.secret" class="input" type="password" required /></div>
        <label v-if="model.backend_type === 'static'" class="checkbox-row field"><input v-model="model.dav.enabled" type="checkbox" /> WebDAV 写入</label>
        <div v-if="model.dav.enabled" class="field"><label>允许方法</label><input class="input" :value="model.dav.methods.join(', ')" @change="setDavMethods" /></div>
        <label v-if="model.dav.enabled" class="checkbox-row field"><input v-model="model.dav.create_full_put_path" type="checkbox" /> 自动创建 PUT 目录</label>
      </div>
    </details>

    <details class="advanced-box full header-processing-box">
      <summary>Header、内容处理与旁路</summary>
      <div class="form-grid compact-grid">
        <section class="header-editor-group full" aria-label="请求 Header">
          <header class="editor-group-header">
            <div><strong>请求 Header</strong><span>控制转发给后端的请求头</span></div>
            <button type="button" class="button compact header-add-button" @click="addHeader('request_headers')">添加请求 Header</button>
          </header>
          <p v-if="model.request_headers.length === 0" class="editor-empty">未配置请求 Header。</p>
          <div v-for="(item, index) in model.request_headers" :key="`req-${index}`" class="inline-editor header-editor-row">
            <input v-model.trim="item.name" class="input" aria-label="请求 Header 名称" placeholder="Header 名称" /><input v-model="item.value" class="input" aria-label="请求 Header 值" placeholder="值；空值表示不转发" /><button type="button" class="button danger-ghost compact" @click="model.request_headers.splice(index, 1)">删除</button>
          </div>
        </section>
        <section class="header-editor-group full" aria-label="响应 Header">
          <header class="editor-group-header">
            <div><strong>响应 Header</strong><span>添加返回给客户端的响应头</span></div>
            <button type="button" class="button compact header-add-button" @click="addHeader('response_headers')">添加响应 Header</button>
          </header>
          <p v-if="model.response_headers.length === 0" class="editor-empty">未配置响应 Header。</p>
          <div v-for="(item, index) in model.response_headers" :key="`resp-${index}`" class="inline-editor header-editor-row response-header-row">
            <input v-model.trim="item.name" class="input" aria-label="响应 Header 名称" placeholder="Header 名称" /><input v-model="item.value" class="input" aria-label="响应 Header 值" placeholder="值" /><label class="checkbox-row"><input v-model="item.always" type="checkbox" />always</label><button type="button" class="button danger-ghost compact" @click="model.response_headers.splice(index, 1)">删除</button>
          </div>
        </section>
        <section class="header-editor-group full" aria-label="内容替换">
          <header class="editor-group-header">
            <div><strong>内容替换</strong><span>替换响应正文中的指定内容</span></div>
            <button type="button" class="button compact header-add-button" @click="model.sub_filters.push({ search: '', replacement: '' })">添加内容替换</button>
          </header>
          <p v-if="model.sub_filters.length === 0" class="editor-empty">未配置内容替换。</p>
          <div v-for="(item, index) in model.sub_filters" :key="`sub-${index}`" class="inline-editor header-editor-row">
            <input v-model="item.search" class="input" aria-label="替换前内容" placeholder="替换前" /><input v-model="item.replacement" class="input" aria-label="替换后内容" placeholder="替换后" /><button type="button" class="button danger-ghost compact" @click="model.sub_filters.splice(index, 1)">删除</button>
          </div>
        </section>
        <div class="editor-group-divider full">
          <strong>旁路与响应处理</strong><span>配置追加内容、镜像、SSI 与 Referer 校验</span>
        </div>
        <div class="field"><label>响应前追加 URI</label><input v-model.trim="model.addition_before" class="input" placeholder="/_before" /></div>
        <div class="field"><label>响应后追加 URI</label><input v-model.trim="model.addition_after" class="input" placeholder="/_after" /></div>
        <div class="field"><label>镜像 URI</label><input v-model.trim="model.mirror" class="input" placeholder="/_mirror" /></div>
        <label class="checkbox-row field"><input v-model="model.mirror_request_body" type="checkbox" /> 镜像请求体</label>
        <label class="checkbox-row field"><input v-model="model.ssi" type="checkbox" /> 开启 SSI</label>
        <div class="field"><label>允许 Referer</label><input class="input" :value="model.valid_referers.join(', ')" placeholder="none, blocked, server_names" @change="setList('valid_referers', $event)" /></div>
        <label class="checkbox-row field"><input v-model="model.deny_invalid_referer" type="checkbox" /> 拒绝无效 Referer</label>
      </div>
    </details>
  </div>
</template>

<style scoped>
.directory-authorization-link { border: 0; background: transparent; color: var(--accent, #168354); font: inherit; padding: 2px 4px; margin-left: 4px; cursor: pointer; text-decoration: underline; text-underline-offset: 3px; }
.directory-authorization-link:disabled { opacity: .55; cursor: wait; }
.directory-authorization-link:focus-visible { outline: 2px solid currentColor; outline-offset: 2px; border-radius: 3px; }
.static-directory-control { display: flex; align-items: center; gap: 10px; min-width: 0; }
.rule-modal .rule-form-grid .static-directory-control .input { width: 50%; min-width: 0; }
.static-directory-control .button { flex: none; white-space: nowrap; min-height: 34px; }
@media (max-width: 760px) {
  .rule-modal .rule-form-grid .static-directory-control .input { flex: 1; width: 0; }
}
.auth-mode-note { margin: 0; color: var(--warning); }
.header-editor-group { display: grid; gap: 7px; padding: 10px; border: 1px solid var(--line); border-radius: 9px; background: color-mix(in srgb, var(--surface-soft) 72%, var(--surface)); }
.editor-group-header { display: flex; align-items: center; justify-content: space-between; gap: 14px; }
.editor-group-header > div { min-width: 0; display: flex; align-items: baseline; gap: 10px; }
.editor-group-header strong, .editor-group-divider strong { font-size: 13px; color: var(--text); }
.editor-group-header span, .editor-group-divider span, .editor-empty { color: var(--text-muted); font-size: 12px; }
.header-editor-row { grid-template-columns: minmax(120px, .8fr) minmax(180px, 1.4fr) auto; }
.response-header-row { grid-template-columns: minmax(120px, .8fr) minmax(180px, 1.4fr) auto auto; }
.editor-empty { margin: 0; padding: 3px 2px; }
.editor-group-divider { display: flex; align-items: baseline; gap: 10px; padding: 8px 2px 0; border-top: 1px solid var(--line); }
@media (max-width: 760px) {
  .editor-group-header, .editor-group-header > div, .editor-group-divider { align-items: flex-start; }
  .editor-group-header > div, .editor-group-divider { flex-direction: column; gap: 2px; }
  .header-editor-row, .response-header-row { grid-template-columns: minmax(0, 1fr); }
}
</style>
