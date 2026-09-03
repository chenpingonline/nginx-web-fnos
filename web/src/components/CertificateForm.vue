<script setup lang="ts">
import { reactive } from "vue"; import type { CertificateInput } from "../types";
defineProps<{busy:boolean}>(); const emit=defineEmits<{save:[value:CertificateInput];cancel:[]}>();
const form=reactive<CertificateInput>({name:"",certificate:"",private_key:""});
</script>
<template><form class="form-grid certificate-form-grid" @submit.prevent="emit('save',{...form})">
  <div class="field full"><label for="cert-name">证书名称</label><input id="cert-name" v-model.trim="form.name" class="input" maxlength="80" autofocus placeholder="例如：example.com"><span class="field-help">留空时自动使用证书 CN 或第一个域名。</span></div>
  <div class="field full"><label for="cert-chain">完整证书链（PEM）</label><textarea id="cert-chain" v-model="form.certificate" class="textarea" required rows="10" spellcheck="false" placeholder="-----BEGIN CERTIFICATE-----"></textarea><span class="field-help">建议包含站点证书及中间证书。</span></div>
  <div class="field full"><label for="private-key">私钥（PEM）</label><textarea id="private-key" v-model="form.private_key" class="textarea" required rows="10" spellcheck="false" placeholder="-----BEGIN PRIVATE KEY-----"></textarea><span class="field-help">私钥只发送给本机 nginx-web，并以 0600 权限保存。</span></div>
  <footer class="modal-footer full"><button type="button" class="button ghost" :disabled="busy" @click="emit('cancel')">取消</button><button type="submit" class="button primary" :disabled="busy">{{ busy?'处理中…':'导入证书' }}</button></footer>
</form></template>
