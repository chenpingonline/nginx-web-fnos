<script setup lang="ts">
import { ref } from "vue";
import { PhDownloadSimple, PhUploadSimple, PhArchive } from "@phosphor-icons/vue";
import type { State } from "../types";
export interface BackupFile {
  format: "nginx-web-backup"; version: number; app_version: string; created_at: string;
  state: State; certificate_files: { id: string; certificate: string; private_key: string }[];
}
defineProps<{ state: State; busy: boolean }>();
const emit = defineEmits<{ download: []; restore: [backup: BackupFile]; apply: [] }>();
const fileInput = ref<HTMLInputElement>();
const selected = ref<BackupFile | null>(null), filename = ref(""), error = ref("");
async function selectFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  selected.value = null; filename.value = ""; error.value = "";
  if (!file) return;
  try {
    if (file.size > 64 * 1024 * 1024) throw new Error("备份文件不能超过 64 MB");
    const value = JSON.parse(await file.text());
    if (value?.format !== "nginx-web-backup" || value.version !== 1 || !value.state?.settings ||
      !["rules", "stream_rules", "certificates", "upstream_pools", "rate_limit_policies"].every(key => Array.isArray(value.state[key])) ||
      !Array.isArray(value.certificate_files)) throw new Error("请选择有效的 nginx-web 备份文件");
    selected.value = value; filename.value = file.name;
  } catch (cause) { error.value = cause instanceof Error ? cause.message : "无法读取备份文件"; }
  input.value = "";
}
function date(value: string) {
  const result = new Date(value);
  return Number.isNaN(result.getTime()) ? "未知" : result.toLocaleString("zh-CN");
}
</script>

<template>
  <div class="backup-page">
    <header class="backup-heading"><h1>备份与恢复</h1><p>下载配置副本，或从备份恢复为待应用的草稿</p></header>
    <article class="card">
      <header class="card-header"><PhArchive :size="20" /><div><h2>导出备份</h2><p>备份当前已保存的配置，包括尚未应用的草稿</p></div></header>
      <div class="card-body backup-body">
        <dl class="backup-summary">
          <div><dt>HTTP(S) 代理</dt><dd>{{ state.rules.length }}</dd></div>
          <div><dt>TCP/UDP 代理</dt><dd>{{ state.stream_rules.length }}</dd></div>
          <div><dt>证书</dt><dd>{{ state.certificates.length }}</dd></div>
        </dl>
        <p>包含全局设置、代理规则、后端服务组、限流策略，以及已导入或签发的证书和私钥。</p>
        <p class="backup-note">备份文件包含私钥，请妥善保管。运行日志、流量统计、代理缓存、配置历史和 ACME 账户／续期任务不包含在内；服务器外部引用文件需另行保存。</p>
        <button type="button" class="button primary fit" :disabled="busy" @click="emit('download')"><PhDownloadSimple :size="16" />下载备份</button>
      </div>
    </article>
    <article class="card">
      <header class="card-header"><PhUploadSimple :size="20" /><div><h2>从备份恢复</h2><p>先检查文件，再确认恢复；运行中的 Nginx 配置保持生效</p></div></header>
      <div class="card-body backup-body">
        <div class="backup-file">
          <span>选择备份文件（JSON，最大 64 MB）</span>
          <div class="backup-file-picker">
            <button type="button" class="button secondary" :disabled="busy" @click="fileInput?.click()"><PhUploadSimple :size="16" />选择文件</button>
            <span>{{ filename || "尚未选择文件" }}</span>
          </div>
          <input ref="fileInput" type="file" accept=".json,application/json" hidden :disabled="busy" aria-label="选择备份文件" @change="selectFile" />
        </div>
        <p v-if="error" class="backup-error" role="alert">{{ error }}</p>
        <div v-if="selected" class="backup-preview">
          <strong>{{ filename }}</strong>
          <p>创建时间：{{ date(selected.created_at) }} · nginx-web {{ selected.app_version }}</p>
          <p>{{ selected.state.rules.length }} 条 HTTP(S) 代理 · {{ selected.state.stream_rules.length }} 条 TCP/UDP 代理 · {{ selected.state.certificates.length }} 张证书</p>
          <button type="button" class="button secondary fit" :disabled="busy" @click="emit('restore', selected)">恢复为草稿</button>
        </div>
        <p class="backup-note">恢复会替换当前已保存的设置和代理配置，并在配置历史中保留恢复前的配置。现有证书与 ACME 任务保留；迁移到新设备后，请重新配置自动续期。</p>
      </div>
    </article>
    <div v-if="state.dirty" class="card backup-pending"><span>有待应用的配置，请确认恢复内容后再应用。</span><button type="button" class="button primary" :disabled="busy" @click="emit('apply')">应用草稿</button></div>
  </div>
</template>

<style scoped>
.backup-page, .backup-body { display: grid; gap: 14px; min-width: 0; }
.backup-heading h1 { margin: 0; font-size: 21px; font-weight: 650; }
.backup-heading p { margin: 5px 0 2px; color: var(--text-muted); font-size: 14px; }
.backup-body p { margin: 0; line-height: 1.7; }
.backup-summary { display: flex; flex-wrap: wrap; gap: 24px; margin: 0; }
.backup-summary div { min-width: 100px; }
.backup-summary dt, .backup-note { color: var(--text-muted); font-size: 14px; }
.backup-summary dd { margin: 6px 0 0; font-size: 26px; font-weight: 650; }
.backup-file { display: grid; gap: 10px; font-size: 15px; }
.backup-file-picker { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; }
.backup-file-picker > span { color: var(--text-muted); overflow-wrap: anywhere; min-width: 0; }
.backup-preview { display: grid; gap: 10px; padding: 14px; border: 1px solid var(--line); border-radius: 10px; overflow-wrap: anywhere; }
.backup-error { color: var(--danger); }
.backup-pending { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 14px; flex-wrap: wrap; }
</style>
