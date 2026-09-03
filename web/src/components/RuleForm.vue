<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import type { CertificateMeta, ProxyRule, ProxyRuleInput, Settings } from "../types";
import { formatDate } from "../utils";
const props=defineProps<{rule:ProxyRule|null;settings:Settings;certificates:CertificateMeta[];busy:boolean}>();
const emit=defineEmits<{save:[value:ProxyRuleInput,applyAfter:boolean];cancel:[]}>();
const applyAfter=ref(true); const domains=ref("");
const form=reactive<ProxyRuleInput>({name:"",enabled:true,listen_port:9080,domains:[],tls:false,http2:true,certificate_id:"",upstream_scheme:"http",upstream_host:"127.0.0.1",upstream_port:8080,preserve_host:true,websocket:true,streaming:true,verify_upstream_tls:false,connect_timeout_seconds:10,read_timeout_seconds:3600,send_timeout_seconds:3600,client_max_body_mb:0});
watch(()=>props.rule,rule=>{Object.assign(form,rule??{name:"",enabled:true,listen_port:props.settings.default_http_port,domains:[],tls:false,http2:true,certificate_id:"",upstream_scheme:"http",upstream_host:"127.0.0.1",upstream_port:8080,preserve_host:true,websocket:true,streaming:true,verify_upstream_tls:false,connect_timeout_seconds:10,read_timeout_seconds:3600,send_timeout_seconds:3600,client_max_body_mb:0});domains.value=(rule?.domains??[]).join("\n")},{immediate:true});
function changeTLS(){if(!props.rule||[props.settings.default_http_port,props.settings.default_https_port].includes(form.listen_port))form.listen_port=form.tls?props.settings.default_https_port:props.settings.default_http_port;if(!form.tls)form.certificate_id=""}
function submit(){emit("save",{...form,domains:domains.value.split(/[\s,]+/).filter(Boolean)},applyAfter.value)}
</script>
<template>
  <form class="form-grid" @submit.prevent="submit">
    <div class="field"><label for="rule-name">规则名称</label><input id="rule-name" v-model.trim="form.name" class="input" required maxlength="80" autofocus placeholder="例如：Jellyfin"></div>
    <div class="field"><label>规则状态</label><label class="checkbox-row"><input v-model="form.enabled" type="checkbox"> 启用此规则</label></div>
    <div class="field full"><label for="rule-domains">访问域名 / IP</label><textarea id="rule-domains" v-model="domains" class="textarea" required placeholder="jellyfin.example.com&#10;media.example.com"></textarea><span class="field-help">多个域名可用换行、空格或逗号分隔；使用 * 表示该端口的默认站点。</span></div>
    <div class="form-section">入口设置</div>
    <div class="field"><label for="listen-port">监听端口</label><input id="listen-port" v-model.number="form.listen_port" class="input" type="number" min="1024" max="65535" required><span class="field-help">仅允许非特权端口。</span></div>
    <div class="field"><label>入口协议</label><label class="checkbox-row"><input v-model="form.tls" type="checkbox" @change="changeTLS"> 启用 HTTPS</label></div>
    <div v-if="form.tls" class="field"><label for="certificate">HTTPS 证书</label><select id="certificate" v-model="form.certificate_id" class="select" required><option value="">请选择证书</option><option v-for="cert in certificates" :key="cert.id" :value="cert.id">{{ cert.name }} · {{ formatDate(cert.not_after,true) }}</option></select><span class="field-help">没有证书时，请先到“HTTPS 证书”页面导入。</span></div>
    <div v-if="form.tls" class="field"><label>HTTP/2</label><label class="checkbox-row"><input v-model="form.http2" type="checkbox"> 启用 HTTP/2</label></div>
    <div class="form-section">上游服务</div>
    <div class="field"><label for="upstream-scheme">上游协议</label><select id="upstream-scheme" v-model="form.upstream_scheme" class="select"><option value="http">HTTP</option><option value="https">HTTPS</option></select></div>
    <div class="field"><label for="upstream-host">上游主机</label><input id="upstream-host" v-model.trim="form.upstream_host" class="input" required placeholder="127.0.0.1 或 192.168.1.20"></div>
    <div class="field"><label for="upstream-port">上游端口</label><input id="upstream-port" v-model.number="form.upstream_port" class="input" type="number" min="1" max="65535" required></div>
    <div v-if="form.upstream_scheme==='https'" class="field"><label>上游证书校验</label><label class="checkbox-row"><input v-model="form.verify_upstream_tls" type="checkbox"> 校验上游 HTTPS 证书</label></div>
    <div class="form-section">代理能力</div>
    <div class="field"><label>请求 Host</label><label class="checkbox-row"><input v-model="form.preserve_host" type="checkbox"> 保留客户端 Host</label></div>
    <div class="field"><label>WebSocket</label><label class="checkbox-row"><input v-model="form.websocket" type="checkbox"> 转发连接升级头</label></div>
    <div class="field"><label>流式传输</label><label class="checkbox-row"><input v-model="form.streaming" type="checkbox"> 关闭代理缓冲</label></div>
    <div class="field"><label for="body-limit">请求体上限（MB）</label><input id="body-limit" v-model.number="form.client_max_body_mb" class="input" type="number" min="0" max="102400"><span class="field-help">0 表示不限制。</span></div>
    <div class="form-section">超时设置</div>
    <div class="field"><label>连接超时（秒）</label><input v-model.number="form.connect_timeout_seconds" class="input" type="number" min="1" max="600"></div>
    <div class="field"><label>读取超时（秒）</label><input v-model.number="form.read_timeout_seconds" class="input" type="number" min="1" max="86400"></div>
    <div class="field"><label>发送超时（秒）</label><input v-model.number="form.send_timeout_seconds" class="input" type="number" min="1" max="86400"></div>
    <div class="field"><label>保存方式</label><label class="checkbox-row"><input v-model="applyAfter" type="checkbox"> 保存后立即应用</label></div>
    <footer class="modal-footer full"><button type="button" class="button ghost" :disabled="busy" @click="emit('cancel')">取消</button><button type="submit" class="button primary" :disabled="busy">{{ busy?'处理中…':rule?'保存修改':'创建规则' }}</button></footer>
  </form>
</template>
