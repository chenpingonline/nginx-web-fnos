<script setup lang="ts">
import AppSelect from "./AppSelect.vue";
import { computed, ref, watch } from "vue";
import { PhArrowRight, PhCheckCircle, PhMagnifyingGlass, PhWarningCircle } from "@phosphor-icons/vue";
import { useDashboardData } from "../composables/useDashboardData";
import type { MetricCounts, Overview, RequestAnalysis, RequestSample, LimitAnalysis, LimitPolicySnapshot } from "../types";
import type { TrafficSeries } from "./traffic-chart";
import TrafficChart from "./TrafficChart.vue";
import { trafficPeriod, trafficRanges } from "./traffic-ranges";

const props = defineProps<{
  overview: Overview; busy: boolean; updatedAt: string; initialMinutes: number; initialRule: string;
}>();
const emit = defineEmits<{ overview: [value: Overview]; edit: [id: string] }>();
const minutes = ref(props.initialMinutes), selected = ref(props.initialRule);
const view = ref<"overview" | "limits" | "requests">("overview");
const dimension = ref<"rules" | "paths" | "backends">("rules");
const expandedRequest = ref("");
const chartMode = ref<"traffic" | "errors" | "latency">("traffic");
const requestSearch = ref("");
const requestFilter = ref<"all" | "4xx" | "5xx" | "slow" | "upgrade" | "limited" | "delayed">("all");
const page = ref(1);
const { data, error, fetching, stats, rules, load } = useDashboardData({
  minutes, selected, updatedAt: () => props.updatedAt,
  onOverview: (value) => emit("overview", value),
});
const emptyAnalysis: RequestAnalysis = {
  average_request_time_ms: null, average_upstream_header_time_ms: null, average_upstream_time_ms: null, max_request_time_ms: null,
  slow_requests: 0, status_codes: [], methods: [], paths: [], backends: [], recent: [],
};
const analysis = computed(() => stats.value?.analysis ?? emptyAnalysis);
const limits = computed<LimitAnalysis>(() => analysis.value.limits ?? { covered: 0, rejected: 0, request_rejected: 0, connection_rejected: 0, delayed: 0, total: stats.value?.counts.requests ?? 0, rate: null, affected_rules: 0, rules: [], recent: [] });
const limitCoverage = computed(() => !limits.value.covered
 ? '限流统计尚无有效样本。应用新配置并产生请求后开始统计；旧日志无法回溯识别。'
 : limits.value.covered < limits.value.total ? `仅覆盖部分请求：${number(limits.value.covered)} / ${number(limits.value.total)} 次；拦截占比按已覆盖请求计算。` : '已覆盖本时段全部完成请求；限流拦截可能同时计入 4xx / 5xx，请勿相加。');
const limitSeries: TrafficSeries[] = [
 { key: 'limit_request_rejected', label: '请求速率限制', color: 'var(--warning)', unit: '次' },
 { key: 'limit_connection_rejected', label: '并发限制', color: 'var(--danger)', unit: '次' },
 { key: 'limit_delayed', label: '延迟处理', color: 'var(--accent)', unit: '次' },
];
const limitPage = ref(1);
const limitPageCount = computed(() => Math.max(1, Math.ceil(limits.value.rules.length / 8)));
const visibleLimitRules = computed(() => limits.value.rules.slice((limitPage.value - 1) * 8, limitPage.value * 8));
const recentSource = computed(() => ['limited', 'delayed'].includes(requestFilter.value) ? limits.value.recent : analysis.value.recent);
function limitReason(request: RequestSample) {
 const reasons = [];
 if (request.limit_req_status === 'REJECTED') reasons.push('请求速率限制');
 if (request.limit_conn_status === 'REJECTED') reasons.push('并发限制');
 if (!reasons.length && request.limit_req_status === 'DELAYED') reasons.push('延迟处理');
 return reasons.join('、');
}
function policySummary(policy?: LimitPolicySnapshot) {
 if (!policy) return '未记录策略快照';
 const s = policy.settings;
 return `每秒 ${s.requests_per_second} 次 · 突发容量 ${s.burst} · ${s.no_delay ? '突发不延迟' : '超额排队'}${s.connections ? ` · 并发上限 ${s.connections}` : ''}`;
}
function inspectLimitRule(rule: string, rejected: number) { selected.value = rule; inspectRequests(rejected ? 'limited' : 'delayed'); }
const httpRules = computed(() => rules.value.filter((rule) => rule.protocol === "HTTP" || rule.protocol === "HTTPS"));
const scopedRules = computed(() => httpRules.value.filter((rule) => !selected.value || rule.id === selected.value));
const ruleNames = computed(() => new Map(httpRules.value.map((rule) => [rule.id, rule.name])));
const ruleRows = computed(() => scopedRules.value
  .filter((rule) => rule.counts && rule.counts.requests > 0)
  .sort((a, b) => (b.counts?.requests ?? 0) - (a.counts?.requests ?? 0)));
const pageCount = computed(() => Math.max(1, Math.ceil(ruleRows.value.length / 8)));
const visibleRules = computed(() => ruleRows.value.slice((page.value - 1) * 8, page.value * 8));
const period = computed(() => trafficPeriod(minutes.value));
const scopeName = computed(() => selected.value
  ? (httpRules.value.find((rule) => rule.id === selected.value)?.name ?? "所选规则")
  : "全部 HTTP / HTTPS");
const hasCoverage = computed(() => (stats.value?.observed_seconds ?? 0) > 0 || (stats.value?.counts.requests ?? 0) > 0);
const counts = computed(() => hasCoverage.value ? stats.value?.counts : undefined);
const incomplete = computed(() => hasCoverage.value && counts.value?.client_errors == null);
const issue = computed(() => {
  if (!data.value) return "";
  if (!data.value.monitoring_ready) return "请求分析尚未启用，返回总览保存并应用配置后开始采集；当前草稿也会一并应用。";
  if (!data.value.access_logging) return "访问日志已关闭，请求分析暂停采集；下方保留已采集的历史记录。";
  if (!(data.value.overview ?? props.overview).nginx.running) return "Nginx 已停止，下方展示已采集的历史记录。";
  return stats.value?.issue || stats.value?.history_issue || "";
});
const chartSeries = computed<TrafficSeries[]>(() => {
  if (chartMode.value === "errors") return [
    { key: "client_error_rate", label: "4xx 错误率", color: "var(--warning)", unit: "%", area: true },
    { key: "server_error_rate", label: "5xx 错误率", color: "var(--danger)", unit: "%" },
  ];
  if (chartMode.value === "latency") return [
    { key: "average_request_time_ms", label: "平均响应", color: "var(--accent)", unit: "ms", area: true },
    { key: "average_upstream_header_time_ms", label: "后端首包", color: "#3b82f6", unit: "ms" },
    { key: "average_upstream_time_ms", label: "后端完成", color: "#8b5cf6", unit: "ms" },
  ];
  return [
    { key: "requests", label: "完成请求", color: "var(--accent)", unit: "次", area: true },
    { key: "error_rate", label: "错误率", color: "var(--warning)", unit: "%", axis: "right" },
  ];
});
const filteredRecent = computed(() => {
  const term = requestSearch.value.trim().toLowerCase();
  return recentSource.value.filter((request) => {
    const matches = requestFilter.value === "all"
      || (requestFilter.value === "limited" && (request.limit_req_status === 'REJECTED' || request.limit_conn_status === 'REJECTED'))
      || (requestFilter.value === "delayed" && request.limit_req_status === 'DELAYED' && request.limit_conn_status !== 'REJECTED')
      || (requestFilter.value === "4xx" && request.status >= 400 && request.status < 500)
      || (requestFilter.value === "5xx" && request.status >= 500)
      || (requestFilter.value === "slow" && request.status !== 101 && (request.request_time_ms ?? 0) >= 1000)
      || (requestFilter.value === "upgrade" && request.status === 101);
    const haystack = `${request.method ?? ""} ${request.uri ?? ""} ${request.upstream ?? ""} ${ruleName(request)}`.toLowerCase();
    return matches && (!term || haystack.includes(term));
  });
});
watch([minutes, selected], () => { page.value = 1; limitPage.value = 1; expandedRequest.value = ""; });
watch([requestSearch, requestFilter], () => { expandedRequest.value = ""; });
watch(limitPageCount, (value) => { limitPage.value = Math.min(limitPage.value, value); });
watch(pageCount, (value) => { page.value = Math.min(page.value, value); });

function requestKey(request: RequestSample, index: number) { return `${request.time}-${request.rule}-${request.method}-${request.uri}-${request.status}-${request.upstream}-${index}`; }
function inspectRequests(filter: typeof requestFilter.value) { requestFilter.value = filter; requestSearch.value = ""; view.value = "requests"; }
const rankedItems = computed(() => dimension.value === "backends" ? analysis.value.backends : analysis.value.paths);
const statusTotal = computed(() => analysis.value.status_codes.reduce((sum, item) => sum + item.count, 0));
function number(value: number | null | undefined, digits = 0) {
  return value == null ? "—" : value.toLocaleString("zh-CN", { maximumFractionDigits: digits });
}
function percent(value: number | null | undefined) { return value == null ? "—" : `${number(value, 2)}%`; }
function duration(value: number | null | undefined) {
  if (value == null) return "—";
  return value >= 1000 ? `${number(value / 1000, 2)} s` : `${number(value, value < 10 ? 2 : 0)} ms`;
}
function bytes(value: number | null | undefined) {
  if (value == null) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let amount = value, unit = 0;
  while (amount >= 1024 && unit < units.length - 1) { amount /= 1024; unit++; }
  return `${number(amount, amount < 10 && unit > 0 ? 2 : 1)} ${units[unit]}`;
}
function date(value: string | null | undefined) {
  return value ? new Date(value).toLocaleString("zh-CN", {
    month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit",
  }) : "—";
}
function totalErrors(value: MetricCounts | null | undefined) {
  return value?.client_errors == null ? null : value.client_errors + value.errors;
}
function errorRate(value: MetricCounts | null | undefined) {
  const errors = totalErrors(value);
  return errors == null || !value?.requests ? null : (errors / value.requests) * 100;
}
function averageRequest(value: MetricCounts | null | undefined) {
  return value?.timed_requests ? (value.request_time_ms ?? 0) / value.timed_requests : null;
}
function ruleName(request: RequestSample) {
  return request.rule === "default" ? "默认站点" : (ruleNames.value.get(request.rule) ?? request.limit_policy?.rule_name ?? "已删除规则");
}
function statusClass(status: number | string) {
  const value = Number(status);
  if (value === 101) return "upgrade";
  if (value >= 500) return "server";
  if (value >= 400) return "client";
  if (value >= 300) return "redirect";
  return "success";
}
</script>

<template>
  <div class="request-analysis-page">
    <div v-if="error" class="details-notice error-notice" role="alert">
      <PhWarningCircle :size="18" /><span>数据更新失败：{{ error }}<template v-if="data"> · 当前显示上次成功获取的数据</template></span>
      <button class="button ghost small" :disabled="fetching" @click="load()">重试</button>
    </div>
    <p v-if="issue" class="details-notice" role="status"><PhWarningCircle :size="18" /><span>{{ issue }}</span></p>

    <section class="analysis-toolbar" aria-label="全局分析筛选">
        <div class="details-controls">
          <AppSelect v-model="selected" class="select details-rule-select" aria-label="请求分析规则">
            <option value="">全部 HTTP / HTTPS</option><option v-if="selected && !httpRules.some(rule => rule.id === selected)" :value="selected">历史规则</option><option v-for="rule in httpRules" :key="rule.id" :value="rule.id">{{ rule.name }}</option>
          </AppSelect>
          <div class="range-buttons" aria-label="请求分析时间范围">
            <button v-for="range in trafficRanges" :key="range.value" :class="{ active: minutes === range.value }" :aria-pressed="minutes === range.value" @click="minutes = range.value">{{ range.label }}</button>
          </div>
        </div>

      <button class="button ghost small" :disabled="fetching" @click="load()">{{ fetching ? '更新中…' : '刷新' }}</button>
    </section>
    <nav class="analysis-tabs" aria-label="分析视图">
      <button :class="{ active: view === 'overview' }" :aria-pressed="view === 'overview'" @click="view = 'overview'">概览</button>
      <button :class="{ active: view === 'limits' }" :aria-pressed="view === 'limits'" @click="view = 'limits'">限流分析</button>
      <button :class="{ active: view === 'requests' }" :aria-pressed="view === 'requests'" @click="view = 'requests'">请求排查</button>
      <span>{{ scopeName }} · {{ period }}</span>
    </nav>
    <template v-if="view === 'overview'">
      <div class="analysis-metrics">
        <div class="card analysis-metric"><span>完成请求</span><strong>{{ number(counts?.requests) }}</strong><small>响应流量 {{ bytes(counts?.bytes) }}</small></div>
        <div class="card analysis-metric"><span>错误率</span><strong>{{ percent(errorRate(counts)) }}</strong><small><span class="client-text">4xx {{ number(counts?.client_errors) }}</span> · <span class="server-text">5xx {{ number(counts?.errors) }}</span><template v-if="incomplete"> · 历史统计不完整</template></small><button v-if="limits.covered" class="limit-link" @click="view = 'limits'">限流拦截 {{ number(limits.rejected) }} 次</button></div>
        <div class="card analysis-metric"><span>平均响应</span><strong>{{ duration(analysis.average_request_time_ms) }}</strong><small>不包含 101 长连接</small></div>
        <div class="card analysis-metric"><span>慢请求</span><strong>{{ hasCoverage ? number(analysis.slow_requests) : '—' }}</strong><small>响应耗时 ≥ 1 秒</small></div>
      </div>
    <section class="card request-trend" :aria-busy="fetching">
      <header class="card-header analysis-header"><h2>请求趋势</h2>
      <div class="chart-mode" aria-label="趋势指标">
        <button :class="{ active: chartMode === 'traffic' }" :aria-pressed="chartMode === 'traffic'" @click="chartMode = 'traffic'">请求与错误</button>
        <button :class="{ active: chartMode === 'errors' }" :aria-pressed="chartMode === 'errors'" @click="chartMode = 'errors'">错误分类</button>
        <button :class="{ active: chartMode === 'latency' }" :aria-pressed="chartMode === 'latency'" @click="chartMode = 'latency'">响应耗时</button>
      </div>
      </header>
      <TrafficChart :points="stats?.points ?? []" :series="chartSeries" :loading="fetching && !stats" :height="180" connect-gaps />
      <footer class="trend-footer"><span>滚动时间窗口内的已完成请求，非累计总量</span><span>最近采集 {{ date(stats?.sampled_at) }}</span></footer>
    </section>

    <section class="card distribution-card">
      <header class="card-header"><h2>响应分布</h2><div class="method-list"><span v-for="item in analysis.methods" :key="item.key">{{ item.key }} <strong>{{ number(item.count) }}</strong></span></div></header>
      <div v-if="analysis.status_codes.length" class="status-distribution">
        <div v-for="item in analysis.status_codes" :key="item.key" class="status-item">
          <span class="status-code" :class="statusClass(item.key)">{{ item.key }}</span>
          <div class="status-track"><div :class="statusClass(item.key)" :style="{ width: `${statusTotal ? item.count / statusTotal * 100 : 0}%` }"></div></div>
          <strong>{{ number(item.count) }}</strong><small>{{ percent(statusTotal ? item.count / statusTotal * 100 : 0) }}</small>
        </div>
      </div><div v-else class="compact-empty">本时段暂无状态码分布</div>
    </section>
    <div class="dimension-heading"><div class="chart-mode" aria-label="分析维度">
      <button :class="{ active: dimension === 'rules' }" :aria-pressed="dimension === 'rules'" @click="dimension = 'rules'">按规则</button>
      <button :class="{ active: dimension === 'paths' }" :aria-pressed="dimension === 'paths'" @click="dimension = 'paths'">按路径</button>
      <button :class="{ active: dimension === 'backends' }" :aria-pressed="dimension === 'backends'" @click="dimension = 'backends'">按后端</button>
    </div><span>按完成请求数排序</span><button class="button ghost small" @click="inspectRequests('all')">查看请求样本<PhArrowRight :size="14" /></button></div>
    <section v-if="dimension !== 'rules'" class="card ranking-card">
      <div v-if="rankedItems.length" class="table-wrap"><table class="table"><thead><tr><th>{{ dimension === 'paths' ? '请求路径' : '后端地址' }}</th><th>完成请求</th><th>错误数</th><th>平均响应</th><th v-if="dimension === 'backends'">后端首包</th></tr></thead>
        <tbody><tr v-for="item in rankedItems" :key="item.key"><td><code class="ranking-path">{{ item.key }}</code></td><td>{{ number(item.requests) }}</td><td>{{ number(item.client_errors + item.errors) }}</td><td>{{ duration(item.average_request_time_ms) }}</td><td v-if="dimension === 'backends'">{{ duration(item.average_upstream_header_time_ms) }}</td></tr></tbody>
      </table></div><div v-else class="compact-empty">本时段暂无{{ dimension === 'paths' ? '路径' : '后端' }}统计</div>
    </section>
    <section v-if="dimension === 'rules'" class="card rule-performance">
      <header class="card-header"><div><h2>规则表现 <span class="affected-count">{{ ruleRows.length }}</span></h2><p>按完成请求数排序 · {{ period }}</p></div></header>
      <div v-if="visibleRules.length" class="table-wrap"><table class="table rule-table">
        <thead><tr><th>名称 / 入口</th><th>完成请求</th><th>响应流量</th><th>平均响应</th><th>错误 / 错误率</th><th>最近访问</th><th>操作</th></tr></thead>
        <tbody><tr v-for="rule in visibleRules" :key="rule.id">
          <td><button class="rule-name" @click="selected = rule.id">{{ rule.name }}</button><div class="rule-entry" :title="rule.entry">{{ rule.entry }}</div></td><td>{{ number(rule.counts?.requests) }}</td><td>{{ bytes(rule.counts?.bytes) }}</td><td>{{ duration(averageRequest(rule.counts)) }}</td>
          <td :class="{ 'error-count': (totalErrors(rule.counts) ?? 0) > 0 }">{{ number(totalErrors(rule.counts)) }} / {{ percent(errorRate(rule.counts)) }}</td><td>{{ date(rule.counts?.last_seen) }}</td>
          <td><button class="button ghost small" :disabled="busy || rule.config_state === 'pending_delete'" @click="emit('edit', rule.id)">管理规则<PhArrowRight :size="13" /></button></td>
        </tr></tbody>
      </table></div>
      <div v-else-if="fetching && !stats" class="empty-state"><h3>正在读取请求统计…</h3><p>获取所选范围内的规则数据</p></div>
      <div v-else class="empty-state clear-state"><PhCheckCircle :size="24" /><h3>本时段暂无规则请求</h3><p>产生 HTTP 请求后，这里会展示各规则的表现</p></div>
      <footer v-if="pageCount > 1" class="rules-footer"><span>共 {{ ruleRows.length }} 条</span><div class="pagination"><button class="button ghost small" :disabled="page <= 1" @click="page--">上一页</button><span>{{ page }} / {{ pageCount }}</span><button class="button ghost small" :disabled="page >= pageCount" @click="page++">下一页</button></div></footer>
    </section>    </template>
    <template v-else-if="view === 'limits'">
      <p class="details-notice" role="status"><PhWarningCircle :size="18" /><span>{{ limitCoverage }}</span></p>
      <div class="analysis-metrics">
        <div class="card analysis-metric"><span>拦截请求</span><strong class="client-text">{{ limits.covered ? number(limits.rejected) : '—' }}</strong><small>同一请求仅计一次</small></div>
        <div class="card analysis-metric"><span>拦截占比</span><strong>{{ percent(limits.rate) }}</strong><small>占已覆盖完成请求</small></div>
        <div class="card analysis-metric"><span>延迟处理</span><strong>{{ limits.covered ? number(limits.delayed) : '—' }}</strong><small>排队等待，不计入拦截</small></div>
        <div class="card analysis-metric"><span>触发规则</span><strong>{{ limits.covered ? number(limits.affected_rules) : '—' }}</strong><small>发生拦截或延迟的规则</small></div>
      </div>
      <section class="card"><header class="card-header"><h2>限流趋势</h2><button class="button ghost small" @click="inspectRequests('limited')">查看拦截样本<PhArrowRight :size="14" /></button></header>
        <TrafficChart :points="stats?.points ?? []" :series="limitSeries" :loading="fetching && !stats" :height="180" />
        <footer class="trend-footer"><span>按实际限流结果统计，不根据 429 / 503 状态码推测</span><span>下载限速不计入拦截</span></footer>
      </section>
      <section class="card"><header class="card-header"><div><h2>限流规则排行</h2><p>按拦截次数排序 · 同一策略在不同规则中独立执行</p></div></header>
        <div v-if="visibleLimitRules.length" class="table-wrap"><table class="table limit-table"><thead><tr><th>规则 / 当时策略</th><th>速率拦截</th><th>并发拦截</th><th>延迟处理</th><th>最近触发</th><th>操作</th></tr></thead>
          <tbody><tr v-for="row in visibleLimitRules" :key="row.rule"><td>{{ row.rule === 'default' ? '默认站点' : ruleNames.get(row.rule) ?? row.policy?.rule_name ?? '已删除规则' }}<small>{{ row.policy?.name ?? '未记录策略' }}<template v-if="row.policy_changed"> · 多个配置版本</template></small></td><td>{{ number(row.request_rejected) }}</td><td>{{ number(row.connection_rejected) }}</td><td>{{ number(row.delayed) }}</td><td>{{ date(row.last_seen) }}</td><td><button class="button ghost small" @click="inspectLimitRule(row.rule, row.rejected)">查看记录</button></td></tr></tbody>
        </table></div><div v-else class="compact-empty">{{ limits.covered ? '本时段未记录限流拦截或延迟处理' : '暂无可分析的限流数据' }}</div>
        <footer v-if="limitPageCount > 1" class="rules-footer"><span>共 {{ limits.rules.length }} 条</span><div class="pagination"><button class="button ghost small" :disabled="limitPage <= 1" @click="limitPage--">上一页</button><span>{{ limitPage }} / {{ limitPageCount }}</span><button class="button ghost small" :disabled="limitPage >= limitPageCount" @click="limitPage++">下一页</button></div></footer>
      </section>
    </template>
    <section v-else class="card recent-requests">
      <header class="card-header recent-heading">
        <div><h2>请求样本</h2><p>最近最多 50 条样本；限流筛选使用独立保留的限流样本，非完整访问日志</p></div>
        <div class="recent-filters" role="search" aria-label="筛选异常请求">
          <label class="request-search"><PhMagnifyingGlass :size="16" /><input v-model="requestSearch" type="search" placeholder="搜索路径、规则或后端" aria-label="搜索异常请求" /></label>
          <AppSelect v-model="requestFilter" class="select" aria-label="异常请求类型"><option value="all">全部记录</option><option value="limited">限流拦截</option><option value="delayed">限流延迟</option><option value="4xx">4xx</option><option value="5xx">5xx</option><option value="slow">慢请求</option><option value="upgrade">101 长连接</option></AppSelect>
        </div>
      </header>
      <div v-if="filteredRecent.length" class="table-wrap"><table class="table request-table">
        <thead><tr><th>时间</th><th>规则</th><th>方法 / 路径</th><th>状态码</th><th>耗时</th><th>详情</th></tr></thead>
        <tbody><template v-for="(request, index) in filteredRecent" :key="requestKey(request, index)"><tr>
          <td>{{ date(request.time) }}</td><td>{{ ruleName(request) }}</td><td><code :title="`${request.method ?? ''} ${request.uri ?? ''}`">{{ request.method || "—" }} {{ request.uri || "—" }}</code></td>
          <td><span class="status-code" :class="statusClass(request.status)">{{ request.status }}</span><small v-if="limitReason(request)" class="limit-reason">{{ limitReason(request) }}</small></td>
          <td><small v-if="request.status === 101" class="duration-kind">连接持续时间</small>{{ duration(request.request_time_ms) }}</td>
          <td><button class="button ghost small" :aria-expanded="expandedRequest === requestKey(request, index)" @click="expandedRequest = expandedRequest === requestKey(request, index) ? '' : requestKey(request, index)">{{ expandedRequest === requestKey(request, index) ? '收起' : '展开' }}</button></td>
        </tr><tr v-if="expandedRequest === requestKey(request, index)" class="request-detail-row"><td colspan="6"><dl class="request-detail">
          <template v-if="limitReason(request)"><div><dt>触发类型</dt><dd>{{ limitReason(request) }}</dd></div><div><dt>执行结果</dt><dd>{{ request.limit_req_status === 'REJECTED' || request.limit_conn_status === 'REJECTED' ? '已拒绝' : '延迟处理' }}</dd></div><div><dt>当时策略</dt><dd>{{ request.limit_policy?.name ?? '未记录策略快照' }}</dd></div><div class="full-path"><dt>当时配置</dt><dd>{{ policySummary(request.limit_policy) }}</dd></div></template>
          <div class="full-path"><dt>完整路径（不含查询参数）</dt><dd>{{ request.uri || '—' }}</dd></div>
          <div><dt>后端地址</dt><dd>{{ request.upstream || '—' }}</dd></div><div><dt>后端状态</dt><dd>{{ request.upstream_status || '—' }}</dd></div>
          <div><dt>后端首包</dt><dd>{{ duration(request.upstream_header_time_ms) }}</dd></div><div><dt>{{ request.status === 101 ? '后端连接时间' : '后端完成耗时' }}</dt><dd>{{ duration(request.upstream_time_ms) }}</dd></div>
        </dl></td></tr></template></tbody>
      </table></div>
      <div v-else class="empty-state clear-state"><PhCheckCircle :size="24" /><h3>{{ recentSource.length ? "没有匹配的请求记录" : "本时段暂无请求样本" }}</h3><p>{{ recentSource.length ? "调整搜索或筛选条件后重试" : "样本数量有限；没有样本不代表本时段没有发生请求或拦截" }}</p></div>
    </section>


  </div>
</template>

<style scoped>
.request-analysis-page { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; min-width: 0; }
.request-analysis-page > * { min-width: 0; }
.details-heading { padding: 2px 0 4px; }
h1 { margin: 0; font-size: 21px; font-weight: 650; line-height: 1.4; }
.details-heading p, .card-header p { margin: 3px 0 0; color: var(--text-muted); font-size: 14px; }
.details-notice { display: flex; align-items: center; gap: 10px; margin: 0; border: 1px solid var(--line); border-radius: 10px; padding: 10px 14px; color: var(--text-muted); font-size: 14px; line-height: 1.6; }
.details-notice > span { flex: 1; min-width: 0; overflow-wrap: anywhere; }.details-notice > svg { flex-shrink: 0; }.error-notice { color: var(--danger); }
.card-header { min-height: 58px; padding: 13px 18px; justify-content: space-between; gap: 12px; }.card-header h2 { font-size: 18px; }.analysis-header, .recent-heading { flex-wrap: wrap; }
.details-controls, .recent-filters { display: flex; align-items: center; justify-content: flex-end; flex-wrap: wrap; gap: 8px; min-width: 0; }.details-rule-select { width: 220px; min-width: 160px; max-width: 100%; }
.range-buttons, .chart-mode { display: flex; max-width: 100%; overflow-x: auto; border: 1px solid var(--line); border-radius: 9px; padding: 3px; background: var(--surface-soft); }
.range-buttons button, .chart-mode button { flex-shrink: 0; border: 0; border-radius: 6px; padding: 6px 10px; background: transparent; color: var(--text-muted); font-size: 13px; white-space: nowrap; }
.range-buttons button.active, .chart-mode button.active { color: var(--text); background: var(--surface-solid); box-shadow: 0 1px 4px #0001; }
.analysis-metrics { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 8px; padding: 6px 18px 10px; }
.analysis-metric { display: flex; flex-direction: column; align-items: flex-start; gap: 5px; min-width: 0; padding: 10px; border: 1px solid transparent; border-radius: 9px; background: transparent; color: var(--text); text-align: left; }
button.analysis-metric:hover, .analysis-metric.active { border-color: var(--line); background: var(--accent-soft); }.analysis-metric > span { display: flex; align-items: center; gap: 6px; color: var(--text-muted); font-size: 14px; }
.analysis-metric strong { font-size: 23px; font-weight: 600; line-height: 1.35; font-variant-numeric: tabular-nums; white-space: nowrap; }.analysis-metric small { color: var(--text-muted); font-size: 12px; line-height: 1.45; }
.analysis-metric i { width: 6px; height: 6px; border-radius: 50%; }.client-dot { background: var(--warning); }.server-dot { background: var(--danger); }
.chart-mode { width: max-content; margin: 0 18px 3px; }.trend-footer, .rules-footer { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px; padding: 0 18px 12px; color: var(--text-muted); font-size: 12px; }
.analysis-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }.ranking-list, .backend-list { padding: 0 18px 14px; }
.ranking-row { display: grid; grid-template-columns: minmax(120px, 1fr) auto auto auto; gap: 14px; align-items: center; min-height: 37px; border-top: 1px solid var(--line); font-size: 13px; }.ranking-row:first-child { border-top: 0; }
code { overflow: hidden; color: var(--text); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; text-overflow: ellipsis; white-space: nowrap; }
.ranking-path { min-width: 0; padding: 8px 0; overflow: visible; line-height: 1.45; overflow-wrap: anywhere; text-overflow: clip; white-space: normal; }
.status-list, .method-list { display: flex; flex-wrap: wrap; gap: 7px; padding: 0 18px 10px; }.status-chip, .method-list span { display: inline-flex; align-items: center; gap: 7px; min-height: 28px; border-radius: 7px; padding: 0 9px; background: var(--surface-soft); color: var(--text-muted); font-size: 13px; }
.status-chip strong { color: var(--text); }.status-chip.client strong { color: var(--warning); }.status-chip.server strong { color: var(--danger); }
.backend-list > div { display: flex; justify-content: space-between; gap: 12px; padding: 8px 0; border-top: 1px solid var(--line); font-size: 13px; color: var(--text-muted); }.backend-list code { max-width: 55%; }.compact-empty { padding: 8px 18px 20px; color: var(--text-muted); font-size: 13px; }
.request-search { display: flex; align-items: center; gap: 7px; width: 250px; height: 34px; padding: 0 10px; border: 1px solid var(--line-strong); border-radius: 8px; background: var(--surface); color: var(--text-muted); }
.request-search input { width: 100%; min-width: 0; border: 0; outline: 0; background: transparent; color: var(--text); }.recent-filters .select { width: 126px; height: 34px; }
.table-wrap { margin: 0 18px; max-width: calc(100% - 36px); overflow-x: auto; }.request-table { min-width: 1060px; }.rule-table { min-width: 840px; }.request-table td, .rule-table td { padding-top: 9px; padding-bottom: 9px; font-variant-numeric: tabular-nums; }
.request-table td:nth-child(3) code { display: block; max-width: 260px; }.request-table td:nth-child(5) code { display: block; max-width: 180px; }.request-table td small { display: block; margin-top: 3px; color: var(--text-muted); }
.status-code { display: inline-grid; min-width: 42px; place-items: center; border-radius: 6px; padding: 3px 6px; background: var(--surface-soft); }.status-code.upgrade { color: var(--accent); background: var(--accent-soft); }.status-code.client { color: var(--warning); background: var(--warning-soft); }.status-code.server { color: var(--danger); background: var(--danger-soft); }.duration-kind { margin: 0 0 3px !important; color: var(--accent) !important; white-space: nowrap; }
.rule-name { border: 0; padding: 0; background: transparent; color: var(--text); font-size: 14px; font-weight: 600; }.rule-name:hover { color: var(--accent); }.rule-entry { max-width: 230px; margin-top: 3px; overflow: hidden; color: var(--text-muted); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.error-count { color: var(--danger); }.affected-count { margin-left: 5px; color: var(--text-muted); font-size: 13px; font-weight: 500; }.clear-state > svg { margin-bottom: 7px; color: var(--accent); }.pagination { display: flex; align-items: center; gap: 8px; }
button:focus-visible, input:focus-visible, select:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
@media (max-width: 1080px) { .analysis-metrics { grid-template-columns: repeat(3, minmax(0, 1fr)); } .analysis-grid { grid-template-columns: minmax(0, 1fr); } }
@media (max-width: 720px) { .details-controls, .recent-filters { width: 100%; justify-content: stretch; } .details-rule-select, .request-search { flex: 1 1 100%; width: 100%; } .analysis-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); padding-inline: 12px; } .analysis-metric strong { font-size: 20px; } .chart-mode { margin-inline: 12px; } .table-wrap { margin-inline: 12px; max-width: calc(100% - 24px); } .ranking-row { grid-template-columns: minmax(120px, 1fr) auto; } .ranking-row > span:nth-of-type(2), .ranking-row > span:nth-of-type(3) { display: none; } }

.analysis-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.analysis-toolbar .details-controls { justify-content: flex-start; }
.analysis-tabs { display: flex; align-items: center; gap: 22px; border-bottom: 1px solid var(--line); }
.analysis-tabs > button { padding: 10px 2px; border: 0; border-bottom: 2px solid transparent; color: var(--text-muted); background: none; font-size: 14px; }
.analysis-tabs > button.active { border-bottom-color: var(--accent); color: var(--accent); font-weight: 600; }
.analysis-tabs > span { margin-left: auto; font-size: 12px; color: var(--text-muted); }
.analysis-metrics { padding: 0; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.analysis-metric { padding: 16px 18px; border: 1px solid var(--line); gap: 7px; }
.analysis-metric strong { font-size: 25px; }
.client-text { color: var(--warning); }.server-text { color: var(--danger); }
.card-header h2 { margin: 0; font-size: 16px; }.card-header p { font-size: 12px; }
.analysis-header .chart-mode { margin: 0; }.trend-footer { padding-top: 5px; }
.distribution-card .method-list { padding: 0; gap: 10px; }.method-list span { padding: 0; background: none; font-size: 12px; }
.status-distribution { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 28px; padding: 0 18px 18px; }
.status-item { display: grid; grid-template-columns: 42px minmax(40px, 1fr) 56px 58px; gap: 10px; align-items: center; font-size: 12px; }.status-item strong, .status-item small { text-align: right; font-variant-numeric: tabular-nums; }.status-item small { color: var(--text-muted); }
.status-track { height: 5px; background: var(--surface-soft); border-radius: 5px; overflow: hidden; }.status-track > div { height: 100%; background: var(--accent); }.status-track > .client { background: var(--warning); }.status-track > .server { background: var(--danger); }
.dimension-heading { display: flex; align-items: center; gap: 12px; }.dimension-heading .chart-mode { margin: 0; }.dimension-heading > span { font-size: 12px; color: var(--text-muted); }.dimension-heading > button { margin-left: auto; }
.request-table { min-width: 760px; }.request-detail-row td { background: var(--surface-soft); }.request-detail { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; margin: 6px 0; }.request-detail .full-path { grid-column: 1 / -1; }.request-detail dt { color: var(--text-muted); font-size: 12px; }.request-detail dd { margin: 5px 0 0; white-space: normal; overflow-wrap: anywhere; }
.ranking-card { padding-block: 8px; }.ranking-path { display: block; max-width: 440px; }.ranking-card td:not(:first-child) { white-space: nowrap; }
@media (max-width: 1080px) { .analysis-metrics { grid-template-columns: repeat(4, minmax(0, 1fr)); }.analysis-metric { padding: 13px; } }
@media (max-width: 720px) { .analysis-toolbar { align-items: flex-start; }.analysis-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }.analysis-tabs > span { display: none; }.status-distribution { grid-template-columns: 1fr; }.dimension-heading { flex-wrap: wrap; }.dimension-heading > span { display: none; }.request-detail { grid-template-columns: repeat(2, minmax(0, 1fr)); }.distribution-card .card-header { flex-wrap: wrap; } }
.limit-link { border: 0; padding: 0; background: none; color: var(--warning); font-size: 12px; text-align: left; }
.limit-table { min-width: 760px; }.limit-table td small { display: block; margin-top: 4px; color: var(--text-muted); font-size: 12px; }.request-table td .limit-reason { color: var(--warning); }
</style>
