<script setup lang="ts">
import type { ACMEJob } from "../types";
defineProps<{ job: ACMEJob }>();
const emit = defineEmits<{ action: [id: string, action: string] }>();
const names: Record<string, string> = { queued: '等待执行', running: '执行中', ready: '已签发', failed: '失败', letsencrypt: 'Let’s Encrypt', zerossl: 'ZeroSSL', staging: 'Let’s Encrypt 测试环境', custom: '自定义', cloudflare: 'Cloudflare', alidns: '阿里云', tencentcloud: '腾讯云 DNSPod' };
const date = (value: string) => new Date(value).toLocaleString();
</script>
<template>
    <div class="acme-job">
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
</template>
<style scoped>
.acme-heading, .acme-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.acme-heading h3 { margin: 0; font-size: 16px; }
.acme-heading { justify-content: space-between; }
.acme-job { padding-top: 14px; margin-top: 14px; border-top: 1px solid var(--line); overflow-wrap: anywhere; }
.acme-message { font-size: 14px; margin: 8px 0; }
.acme-actions { margin-top: 10px; }
</style>
