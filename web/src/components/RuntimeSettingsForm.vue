<script setup lang="ts">
import AppSelect from "./AppSelect.vue";
import { computed, reactive, ref, toRaw, watch } from "vue";
import type { Settings } from "../types";
const props = defineProps<{
  settings: Settings;
  busy: boolean;
  dirty: boolean;
}>();
const emit = defineEmits<{
  save: [value: Settings];
  clearCache: [];
  test: [value: Settings];
  apply: [value: Settings];
}>();
const form = reactive<Settings>(structuredClone(toRaw(props.settings)));
const trusted = ref(""),
  gzipTypes = ref("");
watch(
  () => props.settings,
  (value) => {
    Object.assign(form, structuredClone(toRaw(value)));
    trusted.value = value.real_ip.trusted_proxies.join("\n");
    gzipTypes.value = value.gzip.types.join("\n");
  },
  { immediate: true, deep: true },
);
function formValue(): Settings {
  return {
    ...JSON.parse(JSON.stringify(form)),
    real_ip: {
      ...form.real_ip,
      trusted_proxies: trusted.value.split(/[\s,]+/).filter(Boolean),
    },
    gzip: {
      ...form.gzip,
      types: gzipTypes.value.split(/[\s,]+/).filter(Boolean),
    },
  };
}
const hasChanges = computed(() => JSON.stringify(formValue()) !== JSON.stringify(props.settings));
function submit(event: Event) {
  const action = (event as SubmitEvent).submitter?.getAttribute("data-action");
  if (action === "test") emit("test", formValue());
  else if (action === "apply") emit("apply", formValue());
  else emit("save", formValue());
}
function addMap() {
  form.routing.maps.push({ name: "Host 路由", source: "$host", variable: "$backend", hostnames: true, default: "default", entries: [] });
}
function addGeo() {
  form.routing.geos.push({ name: "IP 分组", source: "$remote_addr", variable: "$region", default: "default", entries: [] });
}
function addSplit() {
  form.routing.splits.push({ name: "灰度分流", source: "$request_id", variable: "$variant", entries: [{ key: "10%", value: "canary" }, { key: "*", value: "stable" }] });
}
</script>
<template>
  <form class="settings-stack" @submit.prevent="submit">
    <article class="card">
      <header class="card-header">
        <div>
          <h2>基础设置</h2>
          <p>默认入口和配置历史</p>
        </div>
      </header>
      <div class="card-body form-grid">
        <div class="field">
          <label>默认 HTTP 端口</label
          ><input
            v-model.number="form.default_http_port"
            class="input"
            type="number"
            min="1"
            max="65535"
            required
          />
        </div>
        <div class="field">
          <label>默认 HTTPS 端口</label
          ><input
            v-model.number="form.default_https_port"
            class="input"
            type="number"
            min="1"
            max="65535"
            required
          />
        </div>
        <div class="field">
          <label>配置历史保留数量</label
          ><input
            v-model.number="form.revision_limit"
            class="input"
            type="number"
            min="1"
            max="100"
            required
          />
        </div>
      </div>
    </article>
    <article class="card">
      <header class="card-header"><div><h2>动态路由变量</h2><p>用 Map、Geo 和 Split Clients 生成可在 Header、重写和后端服务配置中引用的变量</p></div></header>
      <div class="card-body settings-stack">
        <details class="advanced-box"><summary>Map（{{ form.routing.maps.length }}）</summary><div class="form-grid compact-grid">
          <div v-for="(item, index) in form.routing.maps" :key="`map-${index}`" class="routing-editor full">
            <input v-model.trim="item.name" class="input" placeholder="名称" /><input v-model.trim="item.source" class="input" placeholder="$host" /><input v-model.trim="item.variable" class="input" placeholder="$backend" /><input v-model="item.default" class="input" placeholder="默认值" />
            <label class="checkbox-row"><input v-model="item.hostnames" type="checkbox" />域名匹配</label><button type="button" class="button danger compact" @click="form.routing.maps.splice(index, 1)">删除</button>
            <div v-for="(entry, entryIndex) in item.entries" :key="entryIndex" class="inline-editor routing-entry"><input v-model="entry.key" class="input" placeholder="匹配值" /><input v-model="entry.value" class="input" placeholder="输出值" /><button type="button" class="button danger compact" @click="item.entries.splice(entryIndex, 1)">−</button></div>
            <button type="button" class="button ghost compact fit" @click="item.entries.push({ key: '', value: '' })">添加条目</button>
          </div><button type="button" class="button ghost compact fit" @click="addMap">添加 Map</button>
        </div></details>
        <details class="advanced-box"><summary>Geo（{{ form.routing.geos.length }}）</summary><div class="form-grid compact-grid">
          <div v-for="(item, index) in form.routing.geos" :key="`geo-${index}`" class="routing-editor full"><input v-model.trim="item.name" class="input" placeholder="名称" /><input v-model.trim="item.source" class="input" placeholder="$remote_addr" /><input v-model.trim="item.variable" class="input" placeholder="$region" /><input v-model="item.default" class="input" placeholder="默认值" /><button type="button" class="button danger compact" @click="form.routing.geos.splice(index, 1)">删除</button><div v-for="(entry, entryIndex) in item.entries" :key="entryIndex" class="inline-editor routing-entry"><input v-model="entry.key" class="input" placeholder="IP / CIDR" /><input v-model="entry.value" class="input" placeholder="输出值" /><button type="button" class="button danger compact" @click="item.entries.splice(entryIndex, 1)">−</button></div><button type="button" class="button ghost compact fit" @click="item.entries.push({ key: '', value: '' })">添加条目</button></div>
          <button type="button" class="button ghost compact fit" @click="addGeo">添加 Geo</button>
        </div></details>
        <details class="advanced-box"><summary>Split Clients（{{ form.routing.splits.length }}）</summary><div class="form-grid compact-grid">
          <div v-for="(item, index) in form.routing.splits" :key="`split-${index}`" class="routing-editor full"><input v-model.trim="item.name" class="input" placeholder="名称" /><input v-model.trim="item.source" class="input" placeholder="$request_id" /><input v-model.trim="item.variable" class="input" placeholder="$variant" /><button type="button" class="button danger compact" @click="form.routing.splits.splice(index, 1)">删除</button><div v-for="(entry, entryIndex) in item.entries" :key="entryIndex" class="inline-editor routing-entry"><input v-model="entry.key" class="input" placeholder="10% 或 *" /><input v-model="entry.value" class="input" placeholder="输出值" /><button type="button" class="button danger compact" @click="item.entries.splice(entryIndex, 1)">−</button></div><button type="button" class="button ghost compact fit" @click="item.entries.splice(Math.max(0, item.entries.length - 1), 0, { key: '10%', value: '' })">添加比例</button></div>
          <button type="button" class="button ghost compact fit" @click="addSplit">添加 Split</button>
        </div></details>
      </div>
    </article>
    <article class="card">
      <header class="card-header">
        <div>
          <h2>Worker 与文件资源</h2>
          <p>自动适配 fnOS 当前文件句柄上限，避免 worker_connections 警告</p>
        </div>
      </header>
      <div class="card-body form-grid">
        <div class="field">
          <label>Worker 数量</label
          ><input
            v-model.number="form.worker_processes"
            class="input"
            type="number"
            min="0"
            max="128"
          /><span class="field-help">0 表示根据 CPU 自动选择。</span>
        </div>
        <div class="field">
          <label>Worker Connections</label
          ><input
            v-model.number="form.worker_connections"
            class="input"
            type="number"
            min="128"
            max="1048576"
          />
        </div>
        <div class="field">
          <label>Worker 文件句柄上限</label
          ><input
            v-model.number="form.worker_rlimit_nofile"
            class="input"
            type="number"
            min="0"
            max="1048576"
          /><span class="field-help">0 表示自动限制到当前系统软上限。</span>
        </div>
        <div class="field">
          <label>事件处理</label
          ><label class="checkbox-row"
            ><input
              v-model="form.multi_accept"
              type="checkbox"
            />一次接受多个连接</label
          ><label class="checkbox-row"
            ><input v-model="form.file_aio" type="checkbox" />启用文件
            AIO</label
          >
        </div>
        <div class="field"><label>线程池线程数</label><input v-model.number="form.thread_pool_threads" class="input" type="number" min="0" max="1024" /><span class="field-help">0 表示关闭；启用后文件 I/O 使用线程池。</span></div>
        <div class="field"><label>线程池队列上限</label><input v-model.number="form.thread_pool_queue" class="input" type="number" min="1" max="1048576" /></div>
      </div>
    </article>
    <article class="card">
      <header class="card-header">
        <div>
          <h2>客户端真实 IP</h2>
          <p>仅信任明确配置的代理地址</p>
        </div>
      </header>
      <div class="card-body form-grid">
        <div class="field">
          <label>状态</label
          ><label class="checkbox-row"
            ><input v-model="form.real_ip.enabled" type="checkbox" />启用 Real
            IP</label
          ><label class="checkbox-row"
            ><input
              v-model="form.real_ip.recursive"
              type="checkbox"
            />递归解析代理链</label
          >
        </div>
        <div class="field">
          <label>来源 Header</label
          ><AppSelect v-model="form.real_ip.header" class="select">
            <option value="X-Forwarded-For">X-Forwarded-For</option>
            <option value="X-Real-IP">X-Real-IP</option>
            <option value="proxy_protocol">PROXY Protocol</option>
          </AppSelect>
        </div>
        <div class="field full">
          <label>可信代理 IP / CIDR</label
          ><textarea
            v-model="trusted"
            class="textarea"
            placeholder="192.168.1.0/24&#10;10.0.0.1"
          ></textarea>
        </div>
      </div>
    </article>
    <article class="card">
      <header class="card-header">
        <div>
          <h2>压缩</h2>
          <p>Gzip、预压缩文件和兼容客户端解压</p>
        </div>
      </header>
      <div class="card-body form-grid">
        <div class="field">
          <label>开关</label
          ><label class="checkbox-row"
            ><input v-model="form.gzip.enabled" type="checkbox" />启用
            Gzip</label
          ><label class="checkbox-row"
            ><input v-model="form.gzip.static" type="checkbox" />优先使用 .gz
            文件</label
          ><label class="checkbox-row"
            ><input v-model="form.gzip.gunzip" type="checkbox" />启用
            Gunzip</label
          >
        </div>
        <div class="field">
          <label>压缩级别</label
          ><input
            v-model.number="form.gzip.level"
            class="input"
            type="number"
            min="1"
            max="9"
          /><label>最小响应大小（字节）</label
          ><input
            v-model.number="form.gzip.min_length"
            class="input"
            type="number"
            min="0"
            max="10485760"
          />
        </div>
        <div class="field full">
          <label>MIME 类型</label
          ><textarea v-model="gzipTypes" class="textarea"></textarea>
        </div>
      </div>
    </article>
    <article class="card">
      <header class="card-header">
        <div>
          <h2>TLS 与日志</h2>
          <p>全局安全协议、会话缓存和日志缓冲</p>
        </div>
      </header>
      <div class="card-body form-grid">
        <div class="field">
          <label>TLS 协议</label
          ><label class="checkbox-row"
            ><input
              v-model="form.tls.protocols"
              type="checkbox"
              value="TLSv1.2"
            />TLS 1.2</label
          ><label class="checkbox-row"
            ><input
              v-model="form.tls.protocols"
              type="checkbox"
              value="TLSv1.3"
            />TLS 1.3</label
          >
        </div>
        <div class="field">
          <label>加密套件（可选）</label
          ><input
            v-model.trim="form.tls.ciphers"
            class="input"
            placeholder="留空使用 OpenSSL 默认值"
          /><label>会话缓存（MB）</label
          ><input
            v-model.number="form.tls.session_cache_mb"
            class="input"
            type="number"
            min="1"
            max="1024"
          /><label>会话超时（分钟）</label
          ><input
            v-model.number="form.tls.session_timeout_minutes"
            class="input"
            type="number"
            min="1"
            max="1440"
          />
          <label class="checkbox-row"><input v-model="form.tls.ocsp_stapling" type="checkbox" />启用 OCSP Stapling</label>
        </div>
        <div class="field">
          <label>客户端证书校验</label><AppSelect v-model="form.tls.client_verify" class="select"><option value="off">关闭</option><option value="on">强制</option><option value="optional">可选并校验 CA</option><option value="optional_no_ca">可选且不校验 CA</option></AppSelect>
          <template v-if="form.tls.client_verify !== 'off'"><label>客户端 CA 文件</label><input v-model.trim="form.tls.client_ca_file" class="input" placeholder="/vol1/.../client-ca.pem" required /><label>校验深度</label><input v-model.number="form.tls.client_verify_depth" class="input" type="number" min="1" max="10" /></template>
        </div>
        <div class="field">
          <label>访问日志</label
          ><label class="checkbox-row"
            ><input
              v-model="form.logging.access_enabled"
              type="checkbox"
            />启用访问日志</label
          ><label>缓冲（KB）</label
          ><input
            v-model.number="form.logging.access_buffer_kb"
            class="input"
            type="number"
            min="0"
            max="1024"
          /><label>刷新间隔（秒）</label
          ><input
            v-model.number="form.logging.access_flush_seconds"
            class="input"
            type="number"
            min="1"
            max="3600"
          />
        </div>
        <div class="field">
          <label>错误日志级别</label
          ><AppSelect v-model="form.logging.error_level" class="select">
            <option
              v-for="level in [
                'debug',
                'info',
                'notice',
                'warn',
                'error',
                'crit',
                'alert',
                'emerg',
              ]"
              :key="level"
            >
              {{ level }}
            </option>
          </AppSelect>
        </div>
        <div class="field full"><label>自定义访问日志格式（可选）</label><input v-model="form.logging.custom_format" class="input" placeholder='$remote_addr [$time_local] "$request" $status' /><span class="field-help">留空使用内置 fnproxy 格式。</span></div>
        <div class="field"><label>日志轮转大小（MB）</label><input v-model.number="form.logging.rotate_size_mb" class="input" type="number" min="1" max="10240" /></div>
        <div class="field"><label>轮转文件保留数</label><input v-model.number="form.logging.rotate_keep" class="input" type="number" min="1" max="100" /></div>
      </div>
    </article>
    <div class="settings-actions-shell"><div class="sticky-actions">
      <button class="button danger-ghost" type="button" :disabled="busy" @click="emit('clearCache')">清理代理缓存</button>
      <span class="spacer"></span>
      <button class="button secondary" type="submit" data-action="test" :disabled="busy" title="校验当前填写的设置，不保存、不应用">校验配置</button>
      <button class="button secondary" type="submit" :disabled="busy || !hasChanges">
        {{ busy ? "处理中…" : "保存为草稿" }}
      </button>
      <button
        v-if="hasChanges || dirty"
        class="button primary"
        type="submit"
        data-action="apply"
        :disabled="busy"
        title="保存当前设置并应用全部草稿配置"
      >
        保存并应用
      </button>
    </div></div>
  </form>
</template>
