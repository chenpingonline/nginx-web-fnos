<script setup lang="ts">
import type { LocationSettings, UpstreamPool } from "../types";

const props = defineProps<{
  model: LocationSettings;
  upstreamPools: UpstreamPool[];
  root?: boolean;
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
</script>

<template>
  <div class="location-settings">
    <div class="field">
      <label>处理方式</label>
      <select v-model="model.backend_type" class="select" @change="changeBackend">
        <option value="proxy">HTTP 反向代理</option>
        <option value="static">静态文件</option>
        <option value="return">固定返回 / 跳转</option>
        <option value="grpc">gRPC</option>
        <option value="fastcgi">FastCGI</option>
        <option value="uwsgi">uWSGI</option>
        <option value="scgi">SCGI</option>
        <option value="memcached">Memcached</option>
        <option value="status">连接状态</option>
      </select>
    </div>
    <label v-if="root" class="checkbox-row field">
      <input v-model="model.redirect_to_https" type="checkbox" /> HTTP 永久跳转 HTTPS
    </label>

    <template v-if="!root && ['proxy', 'grpc', 'fastcgi', 'uwsgi', 'scgi', 'memcached'].includes(model.backend_type)">
      <div class="field">
        <label>目标服务池</label>
        <select v-model="model.upstream_pool_id" class="select">
          <option value="">单个服务器</option>
          <option v-for="pool in upstreamPools.filter((item) => item.protocol === 'http')" :key="pool.id" :value="pool.id">{{ pool.name }}</option>
        </select>
      </div>
      <div v-if="model.backend_type === 'proxy' || model.backend_type === 'grpc'" class="field">
        <label>协议</label>
        <select v-model="model.upstream_scheme" class="select"><option value="http">HTTP</option><option value="https">HTTPS</option></select>
      </div>
      <template v-if="!model.upstream_pool_id">
        <div class="field"><label>目标主机</label><input v-model.trim="model.upstream_host" class="input" required /></div>
        <div class="field"><label>目标端口</label><input v-model.number="model.upstream_port" class="input" type="number" min="1" max="65535" required /></div>
      </template>
    </template>

    <template v-if="model.backend_type === 'static'">
      <div class="field full"><label>静态目录</label><input v-model.trim="model.static_path" class="input" placeholder="/vol1/data/www" required /></div>
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

    <details class="advanced-box full">
      <summary>路径重写与访问控制</summary>
      <div class="form-grid compact-grid">
        <div class="field"><label>允许 IP/CIDR</label><textarea class="textarea small" :value="model.allow.join('\n')" @change="setList('allow', $event)"></textarea></div>
        <div class="field"><label>拒绝 IP/CIDR</label><textarea class="textarea small" :value="model.deny.join('\n')" @change="setList('deny', $event)"></textarea></div>
        <div v-for="(item, index) in model.rewrites" :key="index" class="inline-editor full">
          <input v-model="item.pattern" class="input" placeholder="正则" /><input v-model="item.replacement" class="input" placeholder="目标" />
          <select v-model="item.flag" class="select"><option value="last">last</option><option value="break">break</option><option value="redirect">302</option><option value="permanent">301</option></select>
          <button type="button" class="button danger compact" @click="model.rewrites.splice(index, 1)">删除</button>
        </div>
        <button type="button" class="button ghost compact fit" @click="addRewrite">添加 Rewrite</button>
      </div>
    </details>

    <details class="advanced-box full">
      <summary>鉴权、安全与 WebDAV</summary>
      <div class="form-grid compact-grid">
        <label class="checkbox-row field"><input v-model="model.basic_auth" type="checkbox" /> Basic Auth</label>
        <div v-if="model.basic_auth" class="field"><label>认证提示</label><input v-model="model.basic_auth_realm" class="input" /></div>
        <div v-if="model.basic_auth" class="field full"><label>htpasswd 绝对路径</label><input v-model.trim="model.basic_auth_file" class="input" placeholder="/vol1/.../.htpasswd" required /></div>
        <div class="field"><label>Auth Request URI</label><input v-model.trim="model.auth_request" class="input" placeholder="/_auth" /></div>
        <label class="checkbox-row field"><input v-model="model.secure_link.enabled" type="checkbox" /> Secure Link</label>
        <div v-if="model.secure_link.enabled" class="field"><label>签名参数</label><input v-model.trim="model.secure_link.argument" class="input" /></div>
        <div v-if="model.secure_link.enabled" class="field"><label>签名密钥</label><input v-model="model.secure_link.secret" class="input" type="password" required /></div>
        <label v-if="model.backend_type === 'static'" class="checkbox-row field"><input v-model="model.dav.enabled" type="checkbox" /> WebDAV 写入</label>
        <div v-if="model.dav.enabled" class="field"><label>允许方法</label><input class="input" :value="model.dav.methods.join(', ')" @change="setDavMethods" /></div>
        <label v-if="model.dav.enabled" class="checkbox-row field"><input v-model="model.dav.create_full_put_path" type="checkbox" /> 自动创建 PUT 目录</label>
      </div>
    </details>

    <details class="advanced-box full">
      <summary>Header、内容处理与旁路</summary>
      <div class="form-grid compact-grid">
        <div v-for="(item, index) in model.request_headers" :key="`req-${index}`" class="inline-editor full">
          <input v-model.trim="item.name" class="input" placeholder="请求 Header" /><input v-model="item.value" class="input" placeholder="值；空值表示不转发" /><button type="button" class="button danger compact" @click="model.request_headers.splice(index, 1)">删除</button>
        </div>
        <button type="button" class="button ghost compact fit" @click="addHeader('request_headers')">添加请求 Header</button>
        <div v-for="(item, index) in model.response_headers" :key="`resp-${index}`" class="inline-editor full">
          <input v-model.trim="item.name" class="input" placeholder="响应 Header" /><input v-model="item.value" class="input" placeholder="值" /><label class="checkbox-row"><input v-model="item.always" type="checkbox" />always</label><button type="button" class="button danger compact" @click="model.response_headers.splice(index, 1)">删除</button>
        </div>
        <button type="button" class="button ghost compact fit" @click="addHeader('response_headers')">添加响应 Header</button>
        <div v-for="(item, index) in model.sub_filters" :key="`sub-${index}`" class="inline-editor full"><input v-model="item.search" class="input" placeholder="替换前" /><input v-model="item.replacement" class="input" placeholder="替换后" /><button type="button" class="button danger compact" @click="model.sub_filters.splice(index, 1)">删除</button></div>
        <button type="button" class="button ghost compact fit" @click="model.sub_filters.push({ search: '', replacement: '' })">添加内容替换</button>
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
