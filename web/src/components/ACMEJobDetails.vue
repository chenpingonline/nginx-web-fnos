<script setup lang="ts">
import { ref, watch } from 'vue';
import type { ACMEJob } from "../types";
const props = defineProps<{ job: ACMEJob; pending?: boolean; feedback?: { type: 'success' | 'error'; message: string } }>();
const emit = defineEmits<{ action: [id: string, action: string] }>();
const names: Record<string, string> = { queued: '等待执行', running: '执行中', ready: '已签发', failed: '失败', letsencrypt: 'Let’s Encrypt', zerossl: 'ZeroSSL', custom: '自定义', cloudflare: 'Cloudflare', alidns: '阿里云', tencentcloud: '腾讯云 DNSPod' };
const date = (value: string) => new Date(value).toLocaleString();
const confirmDelete = ref(false);
watch(() => props.job.id, () => { confirmDelete.value = false; });
function remove() {
  confirmDelete.value = false;
  emit('action', props.job.id, 'delete');
}
</script>
<template>
    <div class="acme-job">
      <div class="acme-heading"><strong>{{ job.name }}</strong><span class="badge" :class="!job.enabled || job.status === 'failed' ? 'danger' : job.status === 'ready' ? 'success' : 'info'">{{ job.enabled ? names[job.status] : '续期已暂停' }}</span></div>
      <div class="field-help">{{ job.domains.join('、') }} · {{ names[job.ca] }} · {{ names[job.provider] || job.provider }}</div>
      <p class="acme-message" :class="job.enabled ? 'renewal-enabled' : 'renewal-paused'" role="status">{{ job.message }}</p>
      <div class="field-help">{{ job.not_after ? `证书到期：${date(job.not_after)} · ` : '' }}{{ job.enabled ? `下次检查：${date(job.next_attempt)}` : '已签发的证书仍可使用' }}</div>
      <div class="acme-actions">
        <button class="button small acme-renew-button" :disabled="pending || job.status === 'running' || !job.enabled" @click="emit('action', job.id, 'retry')">{{ job.certificate_id ? '立即续期 / 重试部署' : '重试' }}</button>
        <button class="button small acme-renewal-toggle" :class="job.enabled ? 'is-pause' : 'is-resume'" :disabled="pending || job.status === 'running'" @click="emit('action', job.id, job.enabled ? 'pause' : 'resume')">{{ job.enabled ? '暂停续期' : '启用续期' }}</button>
        <button class="button danger-ghost small" :disabled="pending || job.status === 'running'" @click="confirmDelete = true">移除任务</button>
        <span v-if="confirmDelete" class="acme-delete-confirm" role="alert">
          确定移除？
          <button type="button" class="button danger small" :disabled="pending" @click="remove">确认</button>
          <button type="button" class="button ghost small" :disabled="pending" @click="confirmDelete = false">取消</button>
        </span>
        <span v-if="feedback" class="acme-action-feedback" :class="feedback.type" :role="feedback.type === 'error' ? 'alert' : 'status'">{{ feedback.type === 'success' ? '✓' : '!' }} {{ feedback.message }}</span>
      </div>
    </div>
</template>
<style scoped>
.acme-heading, .acme-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.acme-heading h3 { margin: 0; font-size: 16px; }
.acme-heading { justify-content: space-between; }
.acme-job { padding-top: 14px; margin-top: 14px; border-top: 1px solid var(--line); overflow-wrap: anywhere; }
.acme-message { font-size: 14px; margin: 8px 0; font-weight: 560; }
.acme-message.renewal-enabled { color: var(--accent-dark); }
.acme-message.renewal-paused { color: var(--danger); }
.acme-actions { margin-top: 10px; }
.acme-renew-button { color: var(--accent-dark); border-color: var(--accent); background: var(--accent-soft); }
.acme-renewal-toggle.is-pause { color: var(--danger); border-color: color-mix(in srgb, var(--danger) 45%, var(--line)); background: var(--danger-soft); }
.acme-renewal-toggle.is-resume { color: white; border-color: var(--accent); background: var(--accent); }
.acme-delete-confirm { display: inline-flex; align-items: center; gap: 6px; color: var(--danger); font-size: 13px; font-weight: 600; }
.acme-action-feedback { display: inline-flex; align-items: center; min-height: 30px; font-size: 13px; font-weight: 600; }
.acme-action-feedback.success { color: var(--accent-dark); }
.acme-action-feedback.error { color: var(--danger); }
</style>
