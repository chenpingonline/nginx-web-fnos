<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue';
import AppSelect from './AppSelect.vue';
import { dnsProviderLink } from '../dnsProviderLinks';
import { request, errorMessage } from '../api';
import type { ACMEInput, DNSCatalog, DNSProviderField } from '../types';
const props = defineProps<{ modelValue: ACMEInput; busy: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: ACMEInput]; ready: [value: boolean] }>();
function set<K extends keyof ACMEInput>(key: K, value: ACMEInput[K]) { emit('update:modelValue', { ...props.modelValue, [key]: value }); }
function credential(key: keyof ACMEInput['credentials'], value: string) { set('credentials', { ...props.modelValue.credentials, [key]: value }); }
const domains = computed({ get: () => props.modelValue.domains.join('\n'), set: (value: string) => set('domains', value.split('\n')) });

const catalog = ref<DNSCatalog>();
const loading = ref(false);
const loadError = ref('');
const controller = new AbortController();
async function loadProviders() {
 loading.value = true; loadError.value = '';
 try { catalog.value = await request<DNSCatalog>('/acme/providers', { signal: controller.signal }); }
 catch(e) { if (!controller.signal.aborted) loadError.value = errorMessage(e); }
 finally { loading.value = false; }
}
onMounted(loadProviders);
onBeforeUnmount(() => controller.abort());
const selected = computed(() => catalog.value?.providers.find(p => p.code === props.modelValue.provider));
watch(selected, value => emit('ready', !!value), { immediate: true });
function selectProvider(value: string) { emit('update:modelValue', { ...props.modelValue, provider: value, dns_config: {} }); }
function dnsValue(key: string, value: string) { set('dns_config', { ...props.modelValue.dns_config, [key]: value }); }
const alidnsFields: Record<string, { label: string; description: string; advanced: boolean }> = {
 ALICLOUD_ACCESS_KEY: { label: 'ID', description: '', advanced: false },
 ALICLOUD_SECRET_KEY: { label: 'Secret', description: '', advanced: false },
 ALICLOUD_RAM_ROLE: { label: '实例 RAM 角色（可选）', description: '仅在阿里云 ECS 上使用实例角色认证时填写；此方式无需填写 AccessKey ID 和 Secret。', advanced: true },
 ALICLOUD_SECURITY_TOKEN: { label: 'STS 临时令牌（可选）', description: '仅使用 STS 临时凭证时填写，并配套填写临时 AccessKey ID 和 Secret；普通 AccessKey 认证请留空。', advanced: true },
};
// Keep provider-specific credential names while hiding environment variable prefixes.
const domesticLabels: Record<string, string> = {
 TENCENTCLOUD_SECRET_ID: 'ID', TENCENTCLOUD_SECRET_KEY: 'Secret',
 BAIDUCLOUD_ACCESS_KEY_ID: 'ID', BAIDUCLOUD_SECRET_ACCESS_KEY: 'Secret',
 COM35_USERNAME: '用户名', COM35_PASSWORD: 'API 密码',
 DNS51_API_KEY: 'API Key', DNS51_API_SECRET: 'Secret',
 DNSLA_API_ID: 'ID', DNSLA_API_SECRET: 'Secret',
 EDGEONE_SECRET_ID: 'ID', EDGEONE_SECRET_KEY: 'Secret',
 HUAWEICLOUD_ACCESS_KEY_ID: 'ID', HUAWEICLOUD_SECRET_ACCESS_KEY: 'Secret', HUAWEICLOUD_REGION: '区域',
 JDCLOUD_ACCESS_KEY_ID: 'ID', JDCLOUD_ACCESS_KEY_SECRET: 'Secret',
 RAINYUN_API_KEY: 'API Key',
 TODAYNIC_AUTH_USER_ID: '账户 ID', TODAYNIC_API_KEY: 'API Key',
 UCLOUD_PUBLIC_KEY: '公钥', UCLOUD_PRIVATE_KEY: '私钥',
 VOLC_ACCESSKEY: 'ID', VOLC_SECRETKEY: 'Secret',
 WESTCN_USERNAME: '用户名', WESTCN_PASSWORD: 'API 密码',
 XINNET_SECRET: '应用密钥', XINNET_AGENT_ID: '代理商 ID',
};
function fieldPresentation(field: DNSProviderField) {
 if (props.modelValue.provider === 'alidns') return alidnsFields[field.key];
 if (props.modelValue.provider === 'aliesa') return alidnsFields[field.key.replace(/^ALIESA_/, 'ALICLOUD_')];
 if (selected.value?.group !== '国内') return undefined;
 const label = domesticLabels[field.key];
 return label ? { label, description: '', advanced: field.advanced } : undefined;
}
function isAdvanced(field: DNSProviderField) {
 const presentation = fieldPresentation(field);
 if (presentation) return presentation.advanced;
 return field.advanced || field.description.startsWith('Alias') || (props.modelValue.provider === 'cloudflare' && field.key !== 'CF_DNS_API_TOKEN');
}
const primaryFields = computed(() => selected.value?.fields.filter(f => !isAdvanced(f)) ?? []);
const providerLink = computed(() => selected.value && dnsProviderLink(selected.value.code, selected.value.name));
</script>
<template>
  <div class="field full"><label for="acme-ca">证书颁发机构</label><AppSelect id="acme-ca" :model-value="modelValue.ca" class="select" :disabled="busy" @update:model-value="set('ca', $event)"><option value="letsencrypt">Let’s Encrypt</option><option value="zerossl">ZeroSSL</option><option value="staging">Let’s Encrypt 测试环境</option><option value="custom">自定义 ACME 服务</option></AppSelect><span v-if="modelValue.ca === 'staging'" class="field-help">仅供测试，浏览器不会信任测试证书。</span></div>
  <div v-if="modelValue.ca === 'custom'" class="field full"><label for="acme-url">ACME Directory URL</label><input id="acme-url" class="input" type="url" required placeholder="https://ca.example.com/directory" :value="modelValue.directory_url" :disabled="busy" @input="set('directory_url', ($event.target as HTMLInputElement).value)" /></div>
  <template v-if="modelValue.ca === 'zerossl' || modelValue.ca === 'custom'">
    <div class="field full"><label for="acme-kid">EAB KID</label><input id="acme-kid" class="input" :required="modelValue.ca === 'zerossl'" :value="modelValue.credentials.eab_kid" :disabled="busy" autocomplete="off" @input="credential('eab_kid', ($event.target as HTMLInputElement).value)" /></div>
    <div class="field full"><label for="acme-hmac">EAB HMAC Key</label><input id="acme-hmac" class="input" type="password" :required="modelValue.ca === 'zerossl'" :value="modelValue.credentials.eab_hmac" :disabled="busy" autocomplete="new-password" @input="credential('eab_hmac', ($event.target as HTMLInputElement).value)" /><span class="field-help">{{ modelValue.ca === 'zerossl' ? '在 ZeroSSL 控制台生成 EAB 凭据，不能用 DNS Token 代替。' : '仅在证书机构要求 EAB 时填写，两项需同时提供。' }}</span></div>
  </template>

  <div class="field full"><label for="acme-provider">DNS 验证服务商</label><AppSelect id="acme-provider" :model-value="modelValue.provider" class="select" :disabled="busy || loading" @update:model-value="selectProvider"><option v-for="provider in catalog?.providers ?? []" :key="provider.code" :value="provider.code">{{ provider.name }}</option></AppSelect></div>
  <div v-if="loadError" class="full notice warning" role="alert">服务商列表加载失败：{{ loadError }} <button type="button" class="button ghost small" @click="loadProviders">重试</button></div>
  <div v-if="selected" class="full dns-provider-fields">
    <p v-if="providerLink" class="field-help"><a class="dns-credential-link" :href="providerLink.url" target="_blank" rel="noopener noreferrer">{{ providerLink.label }}</a></p>
    <div v-for="field in primaryFields" :key="field.key" class="field full dns-provider-field">
      <label :for="`dns-${field.key}`">{{ fieldPresentation(field)?.label ?? field.key }}</label>
      <textarea v-if="field.multiline" :id="`dns-${field.key}`" class="textarea" rows="3" :value="modelValue.dns_config?.[field.key] ?? ''" :disabled="busy" autocomplete="off" spellcheck="false" @input="dnsValue(field.key, ($event.target as HTMLTextAreaElement).value)" />
      <input v-else :id="`dns-${field.key}`" :type="field.secret ? 'password' : 'text'" class="input" :value="modelValue.dns_config?.[field.key] ?? ''" :disabled="busy" autocomplete="new-password" spellcheck="false" @input="dnsValue(field.key, ($event.target as HTMLInputElement).value)" />
      <span v-if="fieldPresentation(field)?.description ?? field.description" class="field-help">{{ fieldPresentation(field)?.description ?? field.description }}</span>
    </div>
  </div>
  <div class="field full"><label for="acme-domains">域名列表</label><textarea id="acme-domains" v-model="domains" class="textarea" rows="3" required spellcheck="false" :disabled="busy" placeholder="example.com&#10;*.example.com"></textarea><span class="field-help">每行一个域名，支持通配符；DNS 验证不支持 IP 地址证书。</span></div>
  <div class="field full"><label for="acme-email">电子邮箱</label><input id="acme-email" class="input" type="email" required :value="modelValue.email" :disabled="busy" @input="set('email', ($event.target as HTMLInputElement).value)" /></div>
  <div class="field full"><label for="acme-key-type">密钥算法</label><AppSelect id="acme-key-type" :model-value="modelValue.key_type" class="select" :disabled="busy" @update:model-value="set('key_type', $event)"><option value="rsa2048">RSA 2048</option><option value="rsa4096">RSA 4096</option><option value="ec256">ECDSA P-256</option><option value="ec384">ECDSA P-384</option></AppSelect></div>
  <div class="field full"><label for="acme-dns-timeout">DNS 等待时间（秒）</label><input id="acme-dns-timeout" class="input" type="number" min="30" max="1800" required :value="modelValue.propagation_seconds" :disabled="busy" @input="set('propagation_seconds', Number(($event.target as HTMLInputElement).value))" /></div>
  <label class="checkbox-row full"><input type="checkbox" :checked="modelValue.rotate_key" :disabled="busy" @change="set('rotate_key', ($event.target as HTMLInputElement).checked)" />每次续期更换私钥</label>
  <label class="checkbox-row full"><input type="checkbox" required :checked="modelValue.accept_terms" :disabled="busy" @change="set('accept_terms', ($event.target as HTMLInputElement).checked)" />我同意所选证书机构的服务条款，并授权自动申请和续期</label>
  <span class="field-help full"><a v-if="modelValue.ca === 'letsencrypt' || modelValue.ca === 'staging'" href="https://letsencrypt.org/repository/" target="_blank" rel="noopener noreferrer">查看 Let’s Encrypt 服务条款</a><a v-else-if="modelValue.ca === 'zerossl'" href="https://zerossl.com/terms/" target="_blank" rel="noopener noreferrer">查看 ZeroSSL 服务条款</a><template v-else>请向自定义证书机构确认服务条款。</template> 申请在后台执行；签发后可绑定到代理规则，后续自动续期。</span>
</template>

<style scoped>
.dns-provider-fields { min-width: 0; display: grid; grid-template-columns: subgrid; row-gap: 10px; }
.dns-provider-fields > p { margin: 0; grid-column: 1 / -1; }
.dns-provider-field { min-width: 0; }
.dns-provider-field > label { max-width: 132px; overflow-wrap: anywhere; }
.dns-provider-field .field-help { overflow-wrap: anywhere; }
</style>
