<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import type { Settings } from "../types";
const props = defineProps<{ settings: Settings; busy: boolean }>();
const emit = defineEmits<{ save: [value: Settings] }>();
const form = reactive<Settings>(structuredClone(props.settings));
const trusted = ref(""),
  gzipTypes = ref("");
watch(
  () => props.settings,
  (value) => {
    Object.assign(form, structuredClone(value));
    trusted.value = value.real_ip.trusted_proxies.join("\n");
    gzipTypes.value = value.gzip.types.join("\n");
  },
  { immediate: true, deep: true },
);
function submit() {
  emit("save", {
    ...structuredClone(form),
    real_ip: {
      ...form.real_ip,
      trusted_proxies: trusted.value.split(/[\s,]+/).filter(Boolean),
    },
    gzip: {
      ...form.gzip,
      types: gzipTypes.value.split(/[\s,]+/).filter(Boolean),
    },
  });
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
            min="1024"
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
            min="1024"
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
          ><select v-model="form.real_ip.header" class="select">
            <option value="X-Forwarded-For">X-Forwarded-For</option>
            <option value="X-Real-IP">X-Real-IP</option>
            <option value="proxy_protocol">PROXY Protocol</option>
          </select>
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
          ><select v-model="form.logging.error_level" class="select">
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
          </select>
        </div>
      </div>
    </article>
    <div class="sticky-actions">
      <button class="button primary" type="submit" :disabled="busy">
        {{ busy ? "处理中…" : "保存全局设置" }}
      </button>
    </div>
  </form>
</template>
