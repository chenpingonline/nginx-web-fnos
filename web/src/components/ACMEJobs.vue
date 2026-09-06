<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue';
import { request, errorMessage } from '../api';
import type { ACMEJob } from '../types';
const emit = defineEmits<{ changed: []; action: [id: string, action: string] }>();
const jobs = ref<ACMEJob[]>([]);
const error = ref('');
let timer: ReturnType<typeof setTimeout>;
let stopped = false;
let controller: AbortController | undefined;
let fingerprint = '';
const names: Record<string, string> = { queued: '等待执行', running: '执行中', ready: '已签发', failed: '失败', letsencrypt: 'Let’s Encrypt', zerossl: 'ZeroSSL', staging: 'Let’s Encrypt 测试环境', custom: '自定义', cloudflare: 'Cloudflare', alidns: '阿里云', tencentcloud: '腾讯云 DNSPod' };
const date = (value: string) => new Date(value).toLocaleString();
async function load() {
  controller = new AbortController();
  try {
    jobs.value = await request<ACMEJob[]>('/acme', { signal: controller.signal });
    error.value = '';
    const next = JSON.stringify(jobs.value.map(j => [j.certificate_id, j.not_after, j.status]));
    if (fingerprint && next !== fingerprint) emit('changed');
    fingerprint = next;
  } catch (e) { if (!stopped) error.value = errorMessage(e); }
  finally { if (!stopped) timer = setTimeout(() => void load(), 3000); }
}
onMounted(() => void load());
onBeforeUnmount(() => { stopped = true; clearTimeout(timer); controller?.abort(); });
</script>
<template>
  <article class="card acme-jobs">
    <header class="acme-heading"><h3>ACME 自动申请与续期</h3><span class="field-help">{{ jobs.length }} 个任务 · 自动刷新</span></header>
    <p v-if="error" class="notice warning" role="alert">{{ error }}</p>
    <p v-if="!jobs.length" class="field-help">在“导入证书”的添加方式中选择“ACME 自动申请”。</p>
    <div v-for="job in jobs" :key="job.id" class="acme-job">
      <div class="acme-heading"><strong>{{ job.name }}</strong><span class="badge" :class="job.status === 'failed' ? 'danger' : job.status === 'ready' ? 'success' : 'info'">{{ job.enabled ? names[job.status] : '续期已暂停' }}</span></div>
      <div class="field-help">{{ job.domains.join('、') }} · {{ names[job.ca] }} · {{ names[job.provider] || job.provider }}</div>
      <p class="acme-message" role="status">{{ job.message }}</p>
      <div class="field-help">{{ job.not_after ? `证书到期：${date(job.not_after)} · ` : '' }}{{ job.enabled ? `下次检查：${date(job.next_attempt)}` : '已签发的证书仍可使用' }}</div>
      <div class="acme-actions">
        <button class="button ghost small" :disabled="job.status === 'running' || !job.enabled" @click="emit('action', job.id, 'retry')">{{ job.certificate_id ? '立即续期 / 重试部署' : '重试' }}</button>
        <button class="button ghost small" :disabled="job.status === 'running'" @click="emit('action', job.id, job.enabled ? 'pause' : 'resume')">{{ job.enabled ? '暂停续期' : '启用续期' }}</button>
        <button class="button danger-ghost small" :disabled="job.status === 'running'" @click="emit('action', job.id, 'delete')">移除任务</button>
      </div>
    </div>
  </article>
</template>
<style scoped>
.acme-jobs { padding: 16px; margin-bottom: 16px; }
.acme-heading, .acme-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.acme-heading h3 { margin: 0; font-size: 14px; }
.acme-heading { justify-content: space-between; }
.acme-job { padding-top: 14px; margin-top: 14px; border-top: 1px solid var(--line); overflow-wrap: anywhere; }
.acme-message { font-size: 12px; margin: 8px 0; }
.acme-actions { margin-top: 10px; }
</style>
