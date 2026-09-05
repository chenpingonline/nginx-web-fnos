<script setup lang="ts">
import { reactive, ref } from "vue";
import type { CertificateInput } from "../types";
import AppSelect from "./AppSelect.vue";
const props = defineProps<{ busy: boolean }>();
const emit = defineEmits<{ save: [value: CertificateInput]; cancel: [] }>();
const method = ref<"file" | "path" | "pem">("file");
const form = reactive({ name: "", certificate: "", private_key: "", certificate_path: "", private_key_path: "" });
const certificateFile = ref<File>();
const keyFile = ref<File>();
const reading = ref(false);
const error = ref("");
function selectFile(event: Event, kind: "certificate" | "key") {
  const file = (event.target as HTMLInputElement).files?.[0];
  if (kind === "certificate") certificateFile.value = file;
  else keyFile.value = file;
  error.value = "";
}
async function submit() {
  if (props.busy || reading.value) return;
  error.value = "";
  const input: CertificateInput = { name: form.name, method: method.value, certificate: "", private_key: "" };
  if (method.value === "path") {
    input.certificate_path = form.certificate_path;
    input.private_key_path = form.private_key_path;
  } else if (method.value === "pem") {
    input.certificate = form.certificate;
    input.private_key = form.private_key;
  } else {
    const cert = certificateFile.value;
    const key = keyFile.value;
    if (!cert || !key) { error.value = "请选择证书链和私钥文件。"; return; }
    if (!cert.size || !key.size) { error.value = "证书和私钥文件不能为空。"; return; }
    if (cert.size > 2 * 1024 * 1024 || key.size > 512 * 1024) {
      error.value = "证书链不能超过 2 MB，私钥不能超过 512 KB。"; return;
    }
    reading.value = true;
    try {
      [input.certificate, input.private_key] = await Promise.all([cert.text(), key.text()]);
    } catch {
      error.value = "无法读取所选文件，请重新选择。"; return;
    } finally { reading.value = false; }
  }
  emit("save", input);
}
</script>
<template>
  <form class="form-grid certificate-form-grid" @submit.prevent="submit">
    <div class="field full">
      <label for="cert-name">证书名称</label>
      <input id="cert-name" v-model.trim="form.name" class="input" maxlength="80" autofocus placeholder="例如：example.com" :disabled="busy || reading" />
      <span class="field-help">留空时自动使用证书 CN 或第一个域名。</span>
    </div>
    <div class="field full">
      <label for="cert-method">添加方式</label>
      <AppSelect id="cert-method" v-model="method" class="select" :disabled="busy || reading" @change="error = ''">
        <option value="file">文件上传</option>
        <option value="path">服务器路径</option>
        <option value="pem">粘贴 PEM</option>
      </AppSelect>
    </div>
    <template v-if="method === 'file'">
      <div class="field full">
        <label for="cert-file">证书链文件</label>
        <input id="cert-file" type="file" class="input certificate-file-input" accept=".pem,.crt,.cer" required :disabled="busy || reading" @change="selectFile($event, 'certificate')" />
        <span class="field-help">上传 PEM 格式的 .pem、.crt 或 .cer 文件，建议包含完整证书链，最大 2 MB。</span>
      </div>
      <div class="field full">
        <label for="key-file">私钥文件</label>
        <input id="key-file" type="file" class="input certificate-file-input" accept=".pem,.key" required :disabled="busy || reading" @change="selectFile($event, 'key')" />
        <span class="field-help">上传 PEM 格式的 .key 或 .pem 文件，最大 512 KB。</span>
      </div>
    </template>
    <template v-else-if="method === 'path'">
      <div class="field full">
        <label for="cert-path">证书链路径</label>
        <input id="cert-path" v-model.trim="form.certificate_path" class="input" required placeholder="/path/to/fullchain.pem" :disabled="busy" />
      </div>
      <div class="field full">
        <label for="key-path">私钥路径</label>
        <input id="key-path" v-model.trim="form.private_key_path" class="input" required placeholder="/path/to/privkey.pem" :disabled="busy" />
        <span class="field-help">填写运行 nginx-web 的服务器上的绝对路径，应用需要有读取权限。导入后保存独立副本，源文件更新后需重新导入。</span>
      </div>
    </template>
    <template v-else>
      <div class="field full">
        <label for="cert-chain">完整证书链（PEM）</label>
        <textarea id="cert-chain" v-model="form.certificate" class="textarea" required rows="8" spellcheck="false" placeholder="-----BEGIN CERTIFICATE-----" :disabled="busy"></textarea>
      </div>
      <div class="field full">
        <label for="private-key">私钥（PEM）</label>
        <textarea id="private-key" v-model="form.private_key" class="textarea" required rows="8" spellcheck="false" placeholder="-----BEGIN PRIVATE KEY-----" :disabled="busy"></textarea>
      </div>
    </template>
    <span class="field-help full">私钥仅发送到 nginx-web 服务，并以 0600 权限保存。</span>
    <p v-if="error" class="full certificate-import-error" role="alert">{{ error }}</p>
    <footer class="modal-footer full">
      <button type="button" class="button ghost" :disabled="busy || reading" @click="emit('cancel')">取消</button>
      <button type="submit" class="button primary" :disabled="busy || reading">{{ busy || reading ? '处理中…' : '导入证书' }}</button>
    </footer>
  </form>
</template>
<style scoped>
.certificate-file-input { height: auto; min-height: 36px; padding: 6px 10px; }
.certificate-file-input::file-selector-button { margin-right: 12px; padding: 5px 12px; border: 1px solid var(--line); border-radius: 6px; background: var(--surface-soft); color: inherit; cursor: pointer; }
.certificate-import-error { color: var(--danger); margin: 0; }
</style>
