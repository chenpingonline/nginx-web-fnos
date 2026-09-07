<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { PhX } from "@phosphor-icons/vue";
import type { LocationSettings, ProxyRule, Revision } from "../types";
const props = defineProps<{ revision: Revision; title?: string }>();
const emit = defineEmits<{ close: [] }>();
const dialog = ref<HTMLDialogElement | null>(null);
const snapshot = computed(() => props.revision.state);
const rules = computed(() => snapshot.value?.rules ?? []);
const streams = computed(() => snapshot.value?.stream_rules ?? []);
const enabled = computed(() => [...rules.value, ...streams.value].filter(rule => rule.enabled).length);
let returnFocus: HTMLElement | null = null;
onMounted(() => {
  returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  dialog.value?.showModal();
});
onBeforeUnmount(() => {
  dialog.value?.close();
  returnFocus?.focus();
});
function address(host: string, port: number) {
  return `${host?.includes(":") ? `[${host}]` : host || "所有地址"}:${port}`;
}
function target(rule: { upstream_pool_id: string; upstream_host: string; upstream_port: number }) {
  if (rule.upstream_pool_id) return `服务组 · ${snapshot.value.upstream_pools?.find(pool => pool.id === rule.upstream_pool_id)?.name ?? rule.upstream_pool_id}`;
  return address(rule.upstream_host, rule.upstream_port);
}
function destination(location: LocationSettings | undefined, rule: ProxyRule): string {
  if (!location || !location.backend_type || location.backend_type === "proxy") return rule.upstream_pool_id ? target(rule) : `${rule.upstream_scheme || "http"}://${target(rule)}`;
  if (location.backend_type === "static") return `静态目录 · ${location.static_path || "未设置"}`;
  if (location.backend_type === "return") return location.redirect_to_https ? "跳转到 HTTPS" : `返回 ${location.return_code} · ${location.return_target || "无响应内容"}`;
  if (location.backend_type === "status") return "Nginx 状态页";
  return `${location.backend_type.toUpperCase()} · ${target(location)}`;
}
function features(rule: ProxyRule) {
  return [rule.http2 && "HTTP/2", rule.websocket && "WebSocket", rule.streaming && "流式响应", rule.root_location?.cache?.enabled && "缓存", (rule.rate_limit?.enabled || rule.rate_limit_policy_id) && "限流"].filter(Boolean).join(" · ");
}
const strategies: Record<string, string> = { round_robin: "轮询", least_conn: "最少连接", ip_hash: "按客户端 IP", hash: "哈希", random: "随机" };
</script>

<template>
  <dialog ref="dialog" class="modal preview-dialog" aria-labelledby="preview-title" @cancel.prevent="emit('close')">
    <header class="modal-header">
      <div class="preview-heading">
        <h2 id="preview-title">{{ title || "历史配置预览" }}</h2>
        <p>{{ revision.summary || "配置快照" }} · 只读预览</p>
      </div>
      <button class="icon-button modal-close" aria-label="关闭配置预览" autofocus @click="emit('close')"><PhX :size="20" /></button>
    </header>
    <div class="modal-body preview-body">
      <p v-if="!snapshot" class="notice warning">该版本缺少配置快照，无法预览。</p>
      <template v-else>
        <div class="preview-stats">
          <div><strong>{{ rules.length + streams.length }}</strong><span>代理规则</span></div>
          <div><strong>{{ enabled }}</strong><span>已启用</span></div>
          <div><strong>{{ snapshot.upstream_pools?.length ?? 0 }}</strong><span>后端服务组</span></div>
        </div>
        <section v-if="rules.length" class="preview-section">
          <h3>HTTP(S) 代理</h3>
          <article v-for="rule in rules" :key="rule.id" class="preview-rule">
            <div class="preview-rule-heading"><strong>{{ rule.name || "未命名规则" }}</strong><span class="badge" :class="rule.enabled ? 'success' : 'info'">{{ rule.enabled ? '启用' : '停用' }}</span></div>
            <div class="preview-route"><div><span class="preview-label">访问入口</span><div v-for="domain in rule.domains || []" :key="domain">{{ rule.tls ? 'https' : 'http' }}://{{ address(domain, rule.listen_port) }}</div><div v-if="!rule.domains?.length">端口 {{ rule.listen_port }}</div></div><span class="route-arrow" aria-hidden="true">→</span><div><span class="preview-label">目标服务</span>{{ destination(rule.root_location, rule) }}</div></div>
            <p v-if="features(rule)" class="preview-features">{{ features(rule) }}</p>
            <details v-if="rule.locations?.length" class="preview-paths"><summary>路径规则（{{ rule.locations.length }}）</summary><div v-for="location in rule.locations" :key="location.id" class="preview-path"><span>{{ location.path }} · {{ location.enabled ? '启用' : '停用' }}</span><span>{{ destination(location.settings, { ...rule, ...location.settings }) }}</span></div></details>
          </article>
        </section>
        <section v-if="streams.length" class="preview-section">
          <h3>TCP/UDP 代理</h3>
          <article v-for="rule in streams" :key="rule.id" class="preview-rule">
            <div class="preview-rule-heading"><strong>{{ rule.name || '未命名规则' }}</strong><span class="badge" :class="rule.enabled ? 'success' : 'info'">{{ rule.enabled ? '启用' : '停用' }}</span></div>
            <div class="preview-route"><div><span class="preview-label">监听入口 · {{ rule.protocol.toUpperCase() }}</span>{{ address(rule.listen_address, rule.listen_port) }}</div><span class="route-arrow" aria-hidden="true">→</span><div><span class="preview-label">目标服务</span>{{ rule.tls_mode === 'passthrough' && rule.sni_routes?.length ? `按域名分流（${rule.sni_routes.length} 条）` : target(rule) }}</div></div>
          </article>
        </section>
        <p v-if="!rules.length && !streams.length" class="preview-muted">此配置没有代理规则。</p>
        <section v-if="snapshot.upstream_pools?.length" class="preview-section"><h3>后端服务组</h3><div v-for="pool in snapshot.upstream_pools" :key="pool.id" class="preview-rule"><div class="preview-rule-heading"><strong>{{ pool.name }}</strong><span class="preview-muted">{{ strategies[pool.strategy] || pool.strategy }}</span></div><div v-for="(server, index) in pool.servers" :key="index" class="preview-path"><span>{{ address(server.host, server.port) }}</span><span>{{ server.down ? '停用' : server.backup ? '备用' : '启用' }} · 权重 {{ server.weight }}</span></div></div></section>
        <section v-if="snapshot.rate_limit_policies?.length" class="preview-section"><h3>限流策略</h3><div v-for="policy in snapshot.rate_limit_policies" :key="policy.id" class="preview-rule"><strong>{{ policy.name }}</strong><p class="preview-features">{{ policy.settings.enabled ? '启用' : '停用' }} · 每秒请求 {{ policy.settings.requests_per_second || '不限' }} · 并发连接 {{ policy.settings.connections || '不限' }}</p></div></section>
        <section class="preview-section"><h3>全局设置</h3><dl class="preview-settings"><div><dt>HTTP 端口</dt><dd>{{ snapshot.settings?.default_http_port ?? '未设置' }}</dd></div><div><dt>HTTPS 端口</dt><dd>{{ snapshot.settings?.default_https_port ?? '未设置' }}</dd></div><div><dt>保留历史</dt><dd>{{ snapshot.settings?.revision_limit ?? '未设置' }} 个</dd></div></dl></section>
        <details class="preview-raw"><summary>高级：查看完整原始配置</summary><p class="preview-muted">版本：{{ revision.id }}。包含全部参数，供排查问题时使用。</p><pre class="code-view">{{ JSON.stringify(snapshot, null, 2) }}</pre></details>
      </template>
    </div>
    <footer class="modal-footer"><span class="preview-muted">预览不会修改草稿或运行配置</span><button class="button ghost" @click="emit('close')">关闭</button></footer>
  </dialog>
</template>

<style scoped>
.preview-dialog { position: fixed; inset: 0; margin: auto; padding: 0; width: min(820px, calc(100vw - 32px)); max-height: calc(100dvh - 48px); color: var(--text); }
.preview-dialog:not([open]) { display: none; }
.preview-dialog::backdrop { background: rgba(18,25,31,.48); backdrop-filter: blur(4px); }
.preview-heading { flex: 1; min-width: 0; }
.preview-heading p { overflow-wrap: anywhere; }
.preview-body { padding: 20px; overflow-y: auto; min-height: 0; }
.preview-stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; padding-bottom: 18px; border-bottom: 1px solid var(--line); }
.preview-stats div { display: flex; align-items: baseline; gap: 8px; }
.preview-stats strong { font-size: 24px; }
.preview-stats span, .preview-muted, .preview-label, dt { color: var(--text-muted); }
.preview-section { margin-top: 20px; }
h3 { font-size: 15px; margin: 0 0 10px; }
.preview-rule { border: 1px solid var(--line); border-radius: 9px; padding: 12px; margin-top: 8px; overflow-wrap: anywhere; }
.preview-rule-heading { display: flex; align-items: center; gap: 10px; }
.preview-route { display: grid; grid-template-columns: minmax(0, 1fr) 24px minmax(0, 1fr); align-items: center; gap: 12px; margin-top: 12px; line-height: 1.7; }
.preview-label { display: block; font-size: 13px; }
.route-arrow { color: var(--text-muted); }
.preview-features { margin: 10px 0 0; color: var(--text-muted); font-size: 13px; }
.preview-paths { margin-top: 10px; }
summary { cursor: pointer; line-height: 1.8; }
.preview-path { display: flex; justify-content: space-between; gap: 12px; border-top: 1px solid var(--line); margin-top: 8px; padding-top: 8px; }
.preview-settings { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin: 0; }
dd { margin: 6px 0 0; }
.preview-raw { margin-top: 22px; border-top: 1px solid var(--line); padding-top: 12px; }
.code-view { min-height: 0; max-height: 300px; margin-top: 8px; white-space: pre-wrap; overflow-wrap: anywhere; border-radius: 8px; }
.modal-footer { justify-content: space-between; flex-shrink: 0; }
@media (max-width: 600px) { .preview-body { padding: 14px; } .preview-stats div { flex-direction: column; gap: 2px; } .preview-route { grid-template-columns: 1fr; gap: 6px; } .route-arrow { display: none; } .preview-path { flex-direction: column; gap: 4px; } }
</style>
