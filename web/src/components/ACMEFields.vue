<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue';
import AppSelect from './AppSelect.vue';
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
const search = ref('');
const advanced = ref(false);
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
const filtered = computed(() => catalog.value?.providers.filter(p => p.code === props.modelValue.provider || `${p.group} ${p.name} ${p.code}`.toLowerCase().includes(search.value.trim().toLowerCase())) ?? []);
function selectProvider(value: string) { advanced.value = false; emit('update:modelValue', { ...props.modelValue, provider: value, dns_config: {} }); }
function dnsValue(key: string, value: string) { set('dns_config', { ...props.modelValue.dns_config, [key]: value }); }
function isAdvanced(field: DNSProviderField) {
 return field.advanced || field.description.startsWith('Alias') || (props.modelValue.provider === 'cloudflare' && field.key !== 'CF_DNS_API_TOKEN');
}
const primaryFields = computed(() => selected.value?.fields.filter(f => !isAdvanced(f)) ?? []);
const extraFields = computed(() => selected.value?.fields.filter(isAdvanced) ?? []);
</script>
<template>
  <div class="field full"><label for="acme-ca">证书颁发机构</label><AppSelect id="acme-ca" :model-value="modelValue.ca" class="select" :disabled="busy" @update:model-value="set('ca', $event)"><option value="letsencrypt">Let’s Encrypt</option><option value="zerossl">ZeroSSL</option><option value="staging">Let’s Encrypt 测试环境</option><option value="custom">自定义 ACME 服务</option></AppSelect><span v-if="modelValue.ca === 'staging'" class="field-help">仅供测试，浏览器不会信任测试证书。</span></div>
  <div v-if="modelValue.ca === 'custom'" class="field full"><label for="acme-url">ACME Directory URL</label><input id="acme-url" class="input" type="url" required placeholder="https://ca.example.com/directory" :value="modelValue.directory_url" :disabled="busy" @input="set('directory_url', ($event.target as HTMLInputElement).value)" /></div>
  <template v-if="modelValue.ca === 'zerossl' || modelValue.ca === 'custom'">
    <div class="field full"><label for="acme-kid">EAB KID</label><input id="acme-kid" class="input" :required="modelValue.ca === 'zerossl'" :value="modelValue.credentials.eab_kid" :disabled="busy" autocomplete="off" @input="credential('eab_kid', ($event.target as HTMLInputElement).value)" /></div>
    <div class="field full"><label for="acme-hmac">EAB HMAC Key</label><input id="acme-hmac" class="input" type="password" :required="modelValue.ca === 'zerossl'" :value="modelValue.credentials.eab_hmac" :disabled="busy" autocomplete="new-password" @input="credential('eab_hmac', ($event.target as HTMLInputElement).value)" /><span class="field-help">{{ modelValue.ca === 'zerossl' ? '在 ZeroSSL 控制台生成 EAB 凭据，不能用 DNS Token 代替。' : '仅在证书机构要求 EAB 时填写，两项需同时提供。' }}</span></div>
  </template>

  <div class="field full"><label for="acme-provider-search">搜索服务商</label><input id="acme-provider-search" v-model="search" class="input" type="search" placeholder="名称或代码，例如 华为云、dnsla" :disabled="busy || loading" /></div>
  <div class="field full"><label for="acme-provider">DNS 验证服务商</label><AppSelect id="acme-provider" :model-value="modelValue.provider" class="select" :disabled="busy || loading" @update:model-value="selectProvider"><option v-for="provider in filtered" :key="provider.code" :value="provider.code">{{ provider.name }}</option></AppSelect><span class="field-help">{{ catalog?.providers.length ?? 0 }} 个原生 DNS 适配器。使用 DNS-01 验证，无需开放公网 80 端口。</span></div>
  <div v-if="loadError" class="full notice warning" role="alert">服务商列表加载失败：{{ loadError }} <button type="button" class="button ghost small" @click="loadProviders">重试</button></div>
  <div v-if="selected" class="full dns-provider-fields">
    <p class="field-help">按所用认证方式填写；可选字段可留空。<a :href="selected.url" target="_blank" rel="noopener noreferrer">查看 {{ selected.name }} 配置说明</a></p>
    <div v-for="field in primaryFields" :key="field.key" class="dns-provider-field">
      <label :for="`dns-${field.key}`">{{ field.key }}</label>
      <textarea v-if="field.multiline" :id="`dns-${field.key}`" class="textarea" rows="3" :value="modelValue.dns_config?.[field.key] ?? ''" :disabled="busy" autocomplete="off" spellcheck="false" @input="dnsValue(field.key, ($event.target as HTMLTextAreaElement).value)" />
      <input v-else :id="`dns-${field.key}`" :type="field.secret ? 'password' : 'text'" class="input" :value="modelValue.dns_config?.[field.key] ?? ''" :disabled="busy" autocomplete="new-password" spellcheck="false" @input="dnsValue(field.key, ($event.target as HTMLInputElement).value)" />
      <span class="field-help">{{ field.description }}</span>
    </div>
    <button v-if="extraFields.length" type="button" class="button ghost small" :aria-expanded="advanced" @click="advanced = !advanced">{{ advanced ? '收起' : '展开' }}其他认证方式与高级设置（{{ extraFields.length }}）</button>
    <template v-if="advanced"><div v-for="field in extraFields" :key="field.key" class="dns-provider-field">
      <label :for="`dns-${field.key}`">{{ field.key }}</label>
      <textarea v-if="field.multiline" :id="`dns-${field.key}`" class="textarea" rows="3" :value="modelValue.dns_config?.[field.key] ?? ''" :disabled="busy" autocomplete="off" spellcheck="false" @input="dnsValue(field.key, ($event.target as HTMLTextAreaElement).value)" />
      <input v-else :id="`dns-${field.key}`" :type="field.secret ? 'password' : 'text'" class="input" :value="modelValue.dns_config?.[field.key] ?? ''" :disabled="busy" autocomplete="new-password" spellcheck="false" @input="dnsValue(field.key, ($event.target as HTMLInputElement).value)" />
      <span class="field-help">{{ field.description }}</span>
    </div></template>
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
.dns-provider-fields { min-width: 0; display: grid; gap: 10px; }
.dns-provider-fields > p { margin: 0; }
.dns-provider-fields > button { justify-self: start; max-width: 100%; white-space: normal; }
.dns-provider-field { display: grid; gap: 4px; min-width: 0; }
.dns-provider-field > label { font-size: 11px; font-weight: 600; overflow-wrap: anywhere; }
.dns-provider-field .field-help { overflow-wrap: anywhere; }
</style>
