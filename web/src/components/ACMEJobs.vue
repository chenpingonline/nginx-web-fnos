<script setup lang="ts">
import { computed, nextTick, ref, onMounted, onBeforeUnmount } from 'vue';
import ACMEJobDetails from './ACMEJobDetails.vue';
import { request, errorMessage, jsonBody } from '../api';
import type { ACMEJob, CertificateMeta } from '../types';
const props = defineProps<{ certificates: CertificateMeta[] }>();
const emit = defineEmits<{ changed: []; action: [id: string, action: string] }>();
const jobs = ref<ACMEJob[]>([]);
const error = ref('');
let timer: ReturnType<typeof setTimeout>;
let stopped = false;
let controller: AbortController | undefined;
let fingerprint = '';

const selectedId = ref('');
const editing = ref(false);
const domainText = ref('');
const saving = ref(false);
const editError = ref('');
const editMessage = ref('');
const editJobId = ref('');
const editJob = computed(() => jobs.value.find(j => j.id === editJobId.value));
async function showEdit(id: string) {
  await showDetails(id);
  editing.value = true;
  saving.value = true;
  try {
    jobs.value = await request<ACMEJob[]>('/acme');
    const job = jobs.value.find(j => j.certificate_id === id);
    editJobId.value = job?.id || '';
    domainText.value = (job?.domains ?? selected.value?.dns_names ?? []).join('\n');
  } catch (e) { editError.value = errorMessage(e); }
  finally { saving.value = false; }

}
async function reissue() {
  if (saving.value || !editJob.value) return;
  editError.value = ''; editMessage.value = '';
  const domains = domainText.value.split(/[\s,，]+/).filter(Boolean);
  if (!domains.length) { editError.value = '请至少填写一个域名'; return; }
  saving.value = true;
  try {
    const job = await request<ACMEJob>(`/acme/${editJobId.value}/reissue`, { method: 'POST', body: jsonBody({ domains }) });
    jobs.value = jobs.value.map(j => j.id === job.id ? job : j);
    editing.value = false;
    editMessage.value = '修改已保存，正在后台重新签发。成功后自动替换原证书，代理规则无需重新选择证书。';
    emit('changed');
  } catch (e) { editError.value = errorMessage(e); }
  finally { saving.value = false; }
}

const detail = ref<HTMLDialogElement>();
let returnFocus: HTMLElement | null = null;
const selected = computed(() => props.certificates.find(c => c.id === selectedId.value));
const pendingJobs = computed(() => jobs.value.filter(j => !props.certificates.some(c => c.id === j.certificate_id)));
const selectedJobs = computed(() => jobs.value.filter(j => j.certificate_id === selectedId.value));
async function showDetails(id: string) {
  editing.value = false; editError.value = ''; editMessage.value = ''; editJobId.value = '';
  if (!detail.value?.open) returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  selectedId.value = id;
  await nextTick();
  detail.value?.showModal();
}
function closeDetails() { if (saving.value) return; detail.value?.close(); selectedId.value = ''; returnFocus?.focus(); }
function detailAction(id: string, action: string) {
  if (action === 'delete') closeDetails();
  emit('action', id, action);
}
defineExpose({ showDetails, showEdit });
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
  <article v-if="pendingJobs.length || error" class="card acme-jobs">
    <header class="acme-heading"><h3>ACME 自动申请与续期</h3><span class="field-help">{{ pendingJobs.length }} 个任务 · 自动刷新</span></header>
    <p v-if="error" class="notice warning" role="alert">{{ error }}</p>
    <ACMEJobDetails v-for="job in pendingJobs" :key="job.id" :job="job" @action="(id, action) => emit('action', id, action)" />
  </article>
  <dialog ref="detail" class="modal acme-detail-dialog" aria-labelledby="acme-detail-title" @cancel.prevent="closeDetails">
    <header class="modal-header"><div><h2 id="acme-detail-title">{{ editing ? '修改证书域名' : '证书详情' }}</h2><p>{{ selected?.name }}</p></div><button type="button" class="icon-button" aria-label="关闭证书详情" autofocus @click="closeDetails">×</button></header>
    <div v-if="selected" class="modal-body">
      <dl class="certificate-details">
        <dt>域名 / IP</dt><dd>{{ [...(selected.dns_names ?? []), ...(selected.ip_addresses ?? [])].join('、') || '—' }}</dd>
        <dt>主体</dt><dd>{{ selected.subject || '—' }}</dd>
        <dt>有效期</dt><dd>{{ date(selected.not_before) }} ～ {{ date(selected.not_after) }}</dd>
        <dt>序列号</dt><dd>{{ selected.serial_number }}</dd>
        <dt>指纹</dt><dd>{{ selected.fingerprint }}</dd>
      </dl>
      <p v-if="editMessage" class="notice success" role="status">{{ editMessage }}</p>
      <p v-if="editError" class="notice danger" role="alert">{{ editError }}</p>
      <form v-if="editing && editJob" id="certificate-reissue" @submit.prevent="reissue">
        <label for="certificate-domains">签发域名</label>
        <textarea id="certificate-domains" v-model="domainText" class="textarea" rows="4" required :disabled="saving" placeholder="nascp.cn&#10;*.nascp.cn"></textarea>
        <p class="field-help">每行一个域名，也可用逗号或空格分隔。*.nascp.cn 覆盖一级子域名，但不包含 nascp.cn；需要时请同时填写。</p>
        <p class="field-help">沿用原来的证书机构和 DNS 验证配置。修改后立即重新签发并启用自动续期；签发失败时继续使用旧证书。</p>
      </form>
      <p v-if="editing && !editJob && !saving && !editError" class="notice warning">此证书未关联 ACME 申请配置，无法直接重新签发。请通过“导入证书”中的 ACME 申请填写域名和 DNS 验证配置。</p>
      <h3>ACME 自动申请与续期</h3>
      <p v-if="error" class="notice warning" role="alert">{{ error }}</p>
      <ACMEJobDetails v-for="job in selectedJobs" :key="job.id" :job="job" @action="detailAction" />
      <p v-if="!selectedJobs.length && !error" class="field-help">此证书未关联 ACME 自动续期任务。</p>
    </div>
    <footer class="modal-footer">
      <button type="button" class="button ghost" :disabled="saving" @click="closeDetails">关闭</button>
      <button v-if="!editing && selectedJobs.length" type="button" class="button primary" @click="showEdit(selectedId)">修改域名</button>
      <button v-if="editing && editJob" form="certificate-reissue" type="submit" class="button primary" :disabled="saving || editJob.status === 'running' || editJob.status === 'queued'">{{ saving ? '提交中…' : editJob.status === 'running' || editJob.status === 'queued' ? '请等待当前任务完成' : '修改并重新签发' }}</button>
    </footer>
  </dialog>
</template>
<style scoped>
.acme-jobs { padding: 16px; margin-bottom: 16px; }
.acme-heading, .acme-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.acme-heading h3 { margin: 0; font-size: 16px; }
.acme-heading { justify-content: space-between; }
.acme-job { padding-top: 14px; margin-top: 14px; border-top: 1px solid var(--line); overflow-wrap: anywhere; }
.acme-message { font-size: 14px; margin: 8px 0; }
.acme-actions { margin-top: 10px; }
.acme-detail-dialog { position: fixed; inset: 0; margin: auto; padding: 0; width: min(680px, calc(100vw - 32px)); max-height: calc(100dvh - 48px); color: var(--text); }
.acme-detail-dialog:not([open]) { display: none; }
.acme-detail-dialog::backdrop { background: rgba(18,25,31,.48); backdrop-filter: blur(4px); }
.certificate-details { display: grid; grid-template-columns: 80px minmax(0, 1fr); gap: 10px 14px; margin: 0; }
.certificate-details dt { color: var(--text-muted); }
.certificate-details dd { margin: 0; overflow-wrap: anywhere; }
</style>
