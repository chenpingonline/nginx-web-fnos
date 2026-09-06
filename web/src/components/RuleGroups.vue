<script setup lang="ts">
import { computed, nextTick, reactive, ref } from "vue";
import { PhX, PhPencilSimple } from "@phosphor-icons/vue";
import AppSelect from "./AppSelect.vue";
import { errorMessage } from "../api";
import type { RuleGroup, ProxyRule, CertificateMeta } from "../types";
const props = defineProps<{
 groups: RuleGroup[]; rules: ProxyRule[]; certificates: CertificateMeta[]; selected: string; busy: boolean;
 defaultPort: number;
 save: (value: RuleGroup) => Promise<{ group: RuleGroup; error?: string }>;
 remove: (id: string) => Promise<boolean>;
}>();
const emit = defineEmits<{ select: [id: string] }>();
const current = computed(() => props.groups.find(g => g.id === props.selected));
const open = ref(false), error = ref("");
const nameInput = ref<HTMLInputElement>();
const form = reactive<RuleGroup>({ id: "", name: "", listen_port: 9080, tls: false, http2: true, certificate_id: "" });
const count = (id: string) => props.rules.filter(r => (r.group_id || "") === id).length;
const affected = computed(() => props.rules.filter(r => r.group_id === form.id && r.inherit_fields?.length).length);
const certName = (id: string) => props.certificates.find(c => c.id === id)?.name || "未选择证书";
function show(group?: RuleGroup) {
 Object.assign(form, group ?? { id: "", name: "", listen_port: props.defaultPort, tls: false, http2: true, certificate_id: "" });
 error.value = ""; open.value = true;
 void nextTick(() => nameInput.value?.focus());
}
defineExpose({ show });
async function submit() {
 if (props.busy) return;
 error.value = "";
 try {
  const result = await props.save({ ...form, certificate_id: form.tls ? form.certificate_id : "" });
  form.id = result.group.id;
  emit("select", form.id);
  if (result.error) { error.value = result.error; return; }
  open.value = false;
 } catch (e) { error.value = errorMessage(e); }
}
async function remove() {
 error.value = "";
 try { if (await props.remove(form.id)) { open.value = false; emit("select", "all"); } }
 catch (e) { error.value = errorMessage(e); }
}
</script>
<template>
 <div class="rule-groups">
  <nav class="group-tabs" aria-label="代理分组">
   <button type="button" :class="{ active: selected === 'all' }" :aria-pressed="selected === 'all'" @click="emit('select', 'all')">全部 <span>{{ rules.length }}</span></button>
   <button type="button" :class="{ active: selected === '' }" :aria-pressed="selected === ''" @click="emit('select', '')">未分组 <span>{{ count('') }}</span></button>
   <button v-for="group in groups" :key="group.id" type="button" :class="{ active: selected === group.id }" :aria-pressed="selected === group.id" @click="emit('select', group.id)">{{ group.name }} <span>{{ count(group.id) }}</span></button>
  </nav>
  <div v-if="current" class="group-summary">
   <span class="group-summary-name">{{ current.name }}</span>
   <span>{{ current.tls ? 'HTTPS' : 'HTTP' }} · {{ current.listen_port }}<template v-if="current.tls"> · {{ certName(current.certificate_id) }} · HTTP/2 {{ current.http2 ? '开启' : '关闭' }}</template></span>
   <button class="button ghost small" :disabled="busy" @click="show(current)"><PhPencilSimple :size="14" />编辑分组</button>
  </div>
 </div>
 <Teleport to="body">
  <div v-if="open" class="modal-backdrop" @mousedown.self="!busy && (open = false)">
   <section class="modal group-modal" role="dialog" aria-modal="true" aria-labelledby="group-title" @keydown.esc="!busy && (open = false)">
    <header class="modal-header">
     <div><h2 id="group-title">{{ form.id ? '编辑分组' : '新建分组' }}</h2><p>设置组内规则默认使用的访问方式。</p></div>
     <div class="rule-header-actions"><button class="button ghost" :disabled="busy" @click="open = false">取消</button><button form="rule-group-form" type="submit" class="button primary" :disabled="busy">{{ busy ? '保存并应用中…' : '保存并应用' }}</button></div>
     <button class="icon-button modal-close" aria-label="关闭" :disabled="busy" @click="open = false"><PhX :size="20" /></button>
    </header>
    <div class="modal-body">
     <div v-if="error" class="notice danger" role="alert">{{ error }}</div>
     <form id="rule-group-form" class="group-form" @submit.prevent="submit">
      <label for="group-name">分组名称</label><input ref="nameInput" id="group-name" v-model.trim="form.name" class="input" maxlength="80" required autofocus />
      <label for="group-protocol">默认协议</label><AppSelect id="group-protocol" :model-value="form.tls ? 'https' : 'http'" class="select" @update:model-value="form.tls = $event === 'https'"><option value="http">HTTP</option><option value="https">HTTPS</option></AppSelect>
      <label for="group-port">访问端口</label><input id="group-port" v-model.number="form.listen_port" class="input" type="number" min="1024" max="65535" required />
      <template v-if="form.tls">
       <label for="group-cert">默认证书</label><div><AppSelect id="group-cert" v-model="form.certificate_id" class="select" required><option value="">请选择证书</option><option v-for="cert in certificates" :key="cert.id" :value="cert.id">{{ cert.name }}</option></AppSelect><span class="field-help">证书应覆盖组内规则的访问域名。</span></div>
       <label for="group-http2">HTTP/2</label><label class="checkbox-row"><input id="group-http2" v-model="form.http2" type="checkbox" />启用 HTTP/2</label>
      </template>
      <p class="group-form-note">{{ form.id ? `此分组有 ${count(form.id)} 条规则，其中 ${affected} 条使用分组默认值。` : '新建规则默认继承分组配置，每项均可单独自定义。' }}保存后立即应用，其他尚未应用的修改也会一并生效。</p>
      <button v-if="form.id" type="button" class="button danger-ghost group-delete" :disabled="busy" @click="remove">删除分组</button>
     </form>
    </div>
   </section>
  </div>
 </Teleport>
</template>
<style scoped>
.rule-groups { margin-bottom: 14px; }
.group-tabs { display: flex; flex-wrap: wrap; gap: 7px; }
.group-tabs button { display: inline-flex; align-items: center; gap: 8px; padding: 7px 13px; border: 1px solid var(--line); border-radius: 8px; background: transparent; color: var(--text-muted); font: inherit; font-size: 13px; cursor: pointer; }
.group-tabs button.active { background: var(--accent-soft); border-color: var(--accent); color: var(--accent); }
.group-tabs button span { font-size: 12px; opacity: .8; }
.group-summary { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; margin-top: 10px; color: var(--text-muted); font-size: 12px; }
.group-summary-name { font-weight: 600; color: var(--text); }
.group-summary .button { margin-left: auto; }
.group-modal { max-width: 740px; }
.group-modal .rule-header-actions { display: flex; gap: 8px; flex-shrink: 0; margin-right: 0; }
.group-form { display: grid; grid-template-columns: 100px minmax(0, 1fr); gap: 20px 24px; align-items: center; }
.group-form > label { font-weight: 600; }
.group-form .input, .group-form .select { width: 100%; }
.group-form-note { grid-column: 1 / -1; margin: 0; color: var(--text-muted); font-size: 13px; line-height: 1.8; }
.group-delete { grid-column: 1 / -1; justify-self: start; }
@media (max-width: 600px) { .group-modal .modal-header { flex-wrap: wrap; } .group-modal .modal-header > div:first-child { flex-basis: 100%; padding-right: 30px; } .group-modal .rule-header-actions { flex-wrap: nowrap; margin-left: 0; } .group-form { grid-template-columns: 1fr; gap: 12px; } .group-summary .button { margin-left: 0; } }
</style>
