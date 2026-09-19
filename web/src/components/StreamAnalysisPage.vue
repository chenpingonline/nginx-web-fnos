<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import AppSelect from './AppSelect.vue';
import TrafficChart from './TrafficChart.vue';
import { useDashboardData } from '../composables/useDashboardData';
import { trafficRanges } from './traffic-ranges';
import type { MetricPoint, Overview } from '../types';
import type { TrafficSeries } from './traffic-chart';
const props = defineProps<{ updatedAt: string; initialMinutes: number }>();
const emit = defineEmits<{ overview: [value: Overview] }>();
const minutes = ref(props.initialMinutes), selected = ref('');
const { data, currentData, error, fetching, rules, load } = useDashboardData({ minutes, selected, updatedAt: () => props.updatedAt, onOverview: v => emit('overview', v) });
const stats = computed(() => currentData.value?.stream_metrics);
const counts = computed(() => stats.value && (stats.value.sessions > 0 || (stats.value.ready && (selected.value ? stats.value.logging_rules.includes(selected.value) : stats.value.logging_rules.length > 0))) ? stats.value : undefined);
const streamRules = computed(() => rules.value.filter(r => r.protocol === 'TCP' || r.protocol === 'UDP'));
const names = computed(() => new Map(streamRules.value.map(r => [r.id, r.name])));
const ruleName = (id: string) => names.value.get(id) ?? `历史规则 ${id}`;
const search = ref(''), filter = ref('all'), mode = ref('sessions'), tab = ref('overview'), expanded = ref(''), page = ref(1);
watch([minutes, selected, search, filter], () => { page.value = 1; expanded.value = ''; });
const notice = computed(() => {
 if (!stats.value) return '';
 if (!stats.value.ready) return 'TCP/UDP 分析尚未启用，请保存并应用配置后开始采集；旧日志无法回溯统计。';
 if (selected.value && !stats.value.logging_rules.includes(selected.value)) return '所选规则未启用访问日志或尚未应用，下方仅展示已采集的历史记录。';
 if (!stats.value.logging_rules.length) return '暂无启用访问日志的 TCP/UDP 规则，请在规则中开启访问日志并应用。';
 if (!data.value?.overview.nginx.running) return 'Nginx 已停止，下方展示已采集的历史记录。';
 return stats.value.issue ?? '';
});
const unlogged = computed(() => streamRules.value.filter(r => r.enabled && !stats.value?.logging_rules.includes(r.id)).length);
const recent = computed(() => (stats.value?.recent ?? []).filter(s => {
 const term = search.value.trim().toLowerCase();
 return (filter.value === 'all' || (filter.value === 'errors' && s.status >= 400) || (filter.value === 'rejected' && s.limit === 'REJECTED')) && (!term || [ruleName(s.rule), s.client, s.upstream, s.protocol].join(' ').toLowerCase().includes(term));
}));
const ranked = computed(() => stats.value?.rules ?? []);
const pages = computed(() => Math.max(1, Math.ceil(ranked.value.length / 8)));
watch(pages, n => { page.value = Math.min(page.value, n); });
const chartSeries = computed<TrafficSeries[]>(() => mode.value === 'traffic' ? [
 { key: 'received', label: '客户端上行', color: '#3b82f6', unit: 'B' },
 { key: 'sent', label: '客户端下行', color: 'var(--accent)', unit: 'B' },
] : [
 { key: 'sessions', label: '已结束会话', color: 'var(--accent)', unit: '次', area: true },
 { key: 'errors', label: '异常会话', color: 'var(--danger)', unit: '次' },
]);
const points = computed<MetricPoint[]>(() => (counts.value?.points ?? []).map(p => ({ ...p, rps:null,response_rps:null,connections:null,error_rate:null,client_error_rate:null,server_error_rate:null,requests:null,average_request_time_ms:null,average_upstream_header_time_ms:null,average_upstream_time_ms:null,limit_request_rejected:null,limit_connection_rejected:null,limit_delayed:null })));
function number(n?: number) { return n == null ? '—' : n.toLocaleString('zh-CN'); }
function bytes(n?: number) { if(n == null) return '—'; const units=['B','KB','MB','GB','TB']; let i=0; while(n>=1024 && i<4) {n/=1024;i++} return `${n.toFixed(i ? 1 : 0)} ${units[i]}`; }
function duration(n?: number) { return n == null ? '—' : n < 1 ? `${Math.round(n*1000)} ms` : `${n.toFixed(2)} s`; }
function dateTime(value:string) { return new Date(value).toLocaleString('zh-CN',{year:'numeric',month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit',second:'2-digit',fractionalSecondDigits:3,hour12:false}); }
function statusLabel(status:number) { return ({200:'正常结束',400:'协议异常',403:'访问拒绝',500:'内部异常',502:'后端异常',503:'服务不可用'} as Record<number,string>)[status] ?? '会话异常'; }
function inspect(rule:string) { selected.value=rule; tab.value='sessions'; }
</script>
<template>
 <div class="stream-analysis">
  <p v-if="error" class="details-notice error-notice" role="alert">数据更新失败：{{ error }} · 请刷新重试</p>
  <p v-if="notice" class="details-notice" role="status">{{ notice }}</p>
  <section class="analysis-toolbar">
   <div class="details-controls">
    <AppSelect v-model="selected" class="select details-rule-select" aria-label="TCP/UDP 分析规则"><option value="">全部 TCP / UDP</option><option v-if="selected && !names.has(selected)" :value="selected">历史规则 {{ selected }}</option><option v-for="r in streamRules" :key="r.id" :value="r.id">{{ r.name }} · {{ r.protocol }}</option></AppSelect>
    <div class="range-buttons" aria-label="TCP/UDP 分析时间范围"><button v-for="r in trafficRanges" :key="r.value" :class="{ active: minutes === r.value }" :aria-pressed="minutes === r.value" @click="minutes=r.value">{{ r.label }}</button></div>
   </div>
   <button class="button ghost small" :disabled="fetching" @click="load()">{{ fetching ? '更新中…' : '刷新' }}</button>
  </section>
  <nav class="analysis-tabs" aria-label="TCP/UDP 分析视图"><button :class="{active:tab==='overview'}" :aria-pressed="tab==='overview'" @click="tab='overview'">概览</button><button :class="{active:tab==='sessions'}" :aria-pressed="tab==='sessions'" @click="tab='sessions'">会话排查</button></nav>
  <p class="stream-note">会话结束后计入统计；TCP 长连接和 UDP 会话可能延后出现，不代表实时连接数或数据包数。<template v-if="unlogged">有 {{ unlogged }} 条启用规则尚未开启采集。</template></p>
  <template v-if="tab==='overview'">
   <div class="analysis-metrics">
    <div class="card analysis-metric"><span>当前 TCP 连接</span><strong>{{ number(stats?.active_connections ?? undefined) }}</strong><small>每 5 秒刷新，不包含 UDP</small></div>
    <div class="card analysis-metric"><span>已结束会话</span><strong>{{ number(counts?.sessions) }}</strong><small>仅统计启用访问日志的规则</small></div>
    <div class="card analysis-metric"><span>异常会话</span><strong>{{ number(counts?.errors) }}</strong><small>其中并发拦截 {{ number(counts?.rejected) }} 次</small></div>
    <div class="card analysis-metric"><span>收发流量</span><strong>{{ bytes(counts ? counts.sent + counts.received : undefined) }}</strong><small>上行 {{ bytes(counts?.received) }} · 下行 {{ bytes(counts?.sent) }}</small></div>
    <div class="card analysis-metric"><span>平均会话时长</span><strong>{{ duration(stats?.sessions ? stats.duration / stats.sessions : undefined) }}</strong><small>会话持续时间，不是响应耗时</small></div>
   </div>
   <section class="card"><header class="card-header"><h2>会话趋势</h2><div class="range-buttons"><button :class="{active:mode==='sessions'}" @click="mode='sessions'">会话</button><button :class="{active:mode==='traffic'}" @click="mode='traffic'">流量</button></div></header><TrafficChart :points="points" :series="chartSeries" :loading="fetching" /></section>
   <section class="card"><header class="card-header"><h2>规则表现</h2></header><div class="table-wrap"><table class="table stream-table"><thead><tr><th>协议 / 规则</th><th>会话</th><th>异常</th><th>上行 / 下行</th><th>平均时长</th><th></th></tr></thead><tbody><tr v-for="r in ranked.slice((page-1)*8,page*8)" :key="`${r.rule}/${r.protocol}`"><td><span class="stream-rule-inline"><small>{{ r.protocol }}</small><span>{{ ruleName(r.rule) }}</span></span></td><td>{{ number(r.sessions) }}</td><td>{{ number(r.errors) }}</td><td>{{ bytes(r.received) }} / {{ bytes(r.sent) }}</td><td>{{ duration(r.duration/r.sessions) }}</td><td><button class="button ghost small" @click="inspect(r.rule)">查看会话</button></td></tr></tbody></table></div><p v-if="!ranked.length" class="empty-state">本时段暂无已结束会话</p><footer v-if="pages>1" class="stream-footer"><button class="button ghost small" :disabled="page<=1" @click="page--">上一页</button>{{ page }} / {{ pages }}<button class="button ghost small" :disabled="page>=pages" @click="page++">下一页</button></footer></section>
  </template>
  <section v-else class="card">
   <header class="card-header"><div><h2>会话样本</h2><p class="stream-note">最近最多 50 条样本，非完整会话日志。状态码为 Stream 会话状态。</p></div></header>
   <div class="stream-filters"><input v-model="search" class="input" placeholder="搜索规则、客户端或后端" aria-label="搜索会话" /><AppSelect v-model="filter" class="select" aria-label="会话筛选"><option value="all">全部会话</option><option value="errors">异常会话</option><option value="rejected">并发拦截</option></AppSelect></div>
   <div class="table-wrap"><table class="table stream-table"><thead><tr><th>结束时间</th><th>协议 / 规则</th><th>后端</th><th>会话状态</th><th>持续时间</th><th></th></tr></thead><tbody><template v-for="(s,i) in recent" :key="`${s.time}/${s.rule}/${i}`"><tr><td>{{ dateTime(s.time) }}</td><td><span class="stream-rule-inline"><small>{{ s.protocol }}</small><span>{{ ruleName(s.rule) }}</span></span></td><td>{{ s.upstream || '—' }}</td><td><span :class="s.status>=400 ? 'server-text' : 'success-text'">{{ s.status }} · {{ s.limit==='REJECTED' ? '并发拦截' : statusLabel(s.status) }}</span></td><td>{{ duration(s.duration) }}</td><td><button class="button ghost small" :aria-expanded="expanded===`${s.time}/${i}`" @click="expanded=expanded===`${s.time}/${i}` ? '' : `${s.time}/${i}`">{{ expanded===`${s.time}/${i}` ? '收起' : '展开' }}</button></td></tr><tr v-if="expanded===`${s.time}/${i}`"><td colspan="6" class="stream-detail">客户端 {{ s.client }} · 上行 {{ bytes(s.received) }} · 下行 {{ bytes(s.sent) }} · 后端 {{ s.upstream || '未建立后端连接' }}</td></tr></template></tbody></table></div>
   <p v-if="!recent.length" class="empty-state">暂无符合条件的会话样本</p>
  </section>
 </div>
</template>
<style scoped>
.stream-analysis { display:grid; grid-template-columns:minmax(0,1fr); gap:18px; min-width:0; }
.stream-analysis > * { min-width:0; }
.table-wrap { margin:0 18px; max-width:calc(100% - 36px); overflow-x:auto; }
.stream-table { min-width:760px; }
.stream-table td { font-variant-numeric:tabular-nums; }
.stream-table td:first-child { white-space:nowrap; }
.server-text { color:var(--danger); }
.success-text { color:var(--accent); }
.stream-note { margin:0; color:var(--text-muted); font-size:13px; line-height:1.6; }
.stream-rule-inline { display:inline-flex; align-items:baseline; gap:8px; white-space:nowrap; }
.stream-rule-inline small { color:var(--text-muted); font-size:12px; }
.stream-filters { display:flex; gap:12px; padding:16px 20px; }
.stream-filters .input { max-width:380px; }
.stream-filters .select { width:160px; }
.stream-detail { background:var(--surface-soft); white-space:normal; overflow-wrap:anywhere; }
.stream-footer { display:flex; justify-content:flex-end; gap:12px; align-items:center; padding:16px; }
.stream-analysis .analysis-metric strong { font-size:26px; }
.stream-analysis .analysis-metrics { grid-template-columns:repeat(5,minmax(0,1fr)); }
@media(max-width:1100px) { .stream-analysis .analysis-metrics { grid-template-columns:repeat(3,minmax(0,1fr)); } }
@media(max-width:700px) { .stream-filters { flex-wrap:wrap; } .stream-analysis .analysis-metrics { grid-template-columns:repeat(2,minmax(0,1fr)); } }
</style>

<style scoped src="./analysis-common.css"></style>
