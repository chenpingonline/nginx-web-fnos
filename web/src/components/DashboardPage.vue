<script setup lang="ts">
import AppSelect from "./AppSelect.vue";
import { computed, ref, watch } from "vue";
import {
  PhArrowRight,
  PhCheckCircle, PhWarningCircle, PhMagnifyingGlass,
  PhWarning, PhGlobe, PhShareNetwork, PhPencilSimple, PhCaretLeft, PhCaretRight,
} from "@phosphor-icons/vue";
import { useDashboardData } from "../composables/useDashboardData";
import type { DashboardData, DashboardRule, MetricCounts, Overview } from "../types";
import type { TrafficSeries } from "./traffic-chart";
import TrafficChart from "./TrafficChart.vue";
import { trafficRanges, trafficPeriod } from "./traffic-ranges";

const props = defineProps<{
  overview: Overview; busy: boolean; updatedAt: string;
  initialMinutes: number; initialRule: string;
}>();
const emit = defineEmits<{
  overview: [value: Overview]; add: []; apply: []; test: []; reload: []; start: [];
  edit: [id: string]; errors: [scope: { minutes: number; rule: string }];
  navigate: [page: "rules" | "streams" | "certificates" | "logs" | "config"];
}>();
const minutes = ref(props.initialMinutes), selected = ref(props.initialRule);
const { data, error, fetching, stats, rules, load } = useDashboardData({
  minutes, selected, updatedAt: () => props.updatedAt,
  onOverview: (value) => emit("overview", value),
});
const search = ref(""), protocol = ref("all"), configFilter = ref("all");
const sort = ref("errors"), page = ref(1), pageSize = ref(5);
const overview = computed(() => data.value?.overview ?? props.overview);
const period = computed(() => trafficPeriod(minutes.value));
const protocols = computed(() => ["all", "HTTP", "HTTPS", "TCP", "UDP"].map(value => ({
  value, label: value === "all" ? "全部" : value,
  count: rules.value.filter(rule => value === "all" || rule.protocol === value).length,
})));
const filtered = computed(() => {
  const term = search.value.trim().toLowerCase();
  return rules.value.filter(rule =>
    (!term || `${rule.name} ${rule.entry} ${rule.target}`.toLowerCase().includes(term)) &&
    (protocol.value === "all" || rule.protocol === protocol.value) &&
    (configFilter.value === "all" || rule.config_state === configFilter.value),
  ).sort((a, b) => sort.value === "name" ? a.name.localeCompare(b.name, "zh-CN") :
    sort.value === "requests" ? (b.counts?.requests ?? -1) - (a.counts?.requests ?? -1) :
    (totalErrors(b.counts) ?? -1) - (totalErrors(a.counts) ?? -1));
});
const pageCount = computed(() => Math.max(1, Math.ceil(filtered.value.length / pageSize.value)));
const visible = computed(() => filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value));
const pageNumbers = computed(() => {
  const start = Math.max(1, Math.min(page.value - 1, pageCount.value - 4));
  return Array.from({ length: Math.min(5, pageCount.value) }, (_, index) => start + index);
});
watch([search, protocol, configFilter, sort, pageSize], () => { page.value = 1; });
watch(pageCount, count => { page.value = Math.min(page.value, count); });
const chosenName = computed(() => rules.value.find(rule => rule.id === selected.value)?.name);
const series = computed<TrafficSeries[]>(() => selected.value ? [
  { key: "response_rps", label: "响应速率", color: "#3b82f6", unit: "req/s", area: true },
] : [
  { key: "rps", label: "请求速率", color: "#00a653", unit: "req/s", area: true },
  { key: "response_rps", label: "响应速率", color: "#3b82f6", unit: "req/s" },
]);
const configNames = {
  applied: "已生效", pending: "待应用", disabled: "已停用",
  unknown: "待确认", pending_delete: "待删除",
};
function num(value: number | null | undefined, digits = 1) {
  return value == null ? "—" : value.toLocaleString("zh-CN", { maximumFractionDigits: digits });
}
function totalErrors(counts?: MetricCounts | null) {
  return counts?.client_errors == null ? null : counts.client_errors + counts.errors;
}
function errorRate(counts?: MetricCounts | null) {
  const total = totalErrors(counts);
  return total == null || !counts?.requests ? null : total / counts.requests * 100;
}
function showErrors(rule = selected.value) { emit("errors", { minutes: minutes.value, rule }); }
function uptime(seconds?: number | null) {
  if (seconds == null) return "—";
  const days = Math.floor(seconds / 86400), hours = Math.floor(seconds / 3600) % 24;
  const mins = Math.floor(seconds / 60) % 60;
  return days ? `${days} 天 ${hours} 小时` : hours ? `${hours} 小时 ${mins} 分` : `${mins} 分 ${Math.floor(seconds) % 60} 秒`;
}
function date(value?: string | null) {
  return value ? new Date(value).toLocaleString("zh-CN", {
    month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit",
  }) : "—";
}
function openRule(rule: DashboardRule) {
  if (rule.config_state === "pending_delete") emit("navigate", "config");
  else if (["TCP", "UDP"].includes(rule.protocol)) emit("navigate", "streams");
  else emit("edit", rule.id);
}
function averageRate(rule: DashboardRule) {
  return rule.counts && stats.value && stats.value.observed_seconds > 0
    ? rule.counts.requests / stats.value.observed_seconds : null;
}
function certificateText(cert: DashboardData["certificates"][number]) {
  if (new Date(cert.not_before).getTime() > Date.now()) return "尚未生效";
  if (new Date(cert.not_after).getTime() <= Date.now()) return "已过期";
  return cert.days < 1 ? "不足 1 天后到期" : `${cert.days} 天后到期`;
}
const issue = computed(() => {
  if (!data.value) return "";
  if (!data.value.monitoring_ready) return "新版统计尚未启用，保存并应用配置后开始采集；当前草稿也会一并应用。";
  if (!overview.value.nginx.running) return "Nginx 已停止，当前保留历史记录。";
  if (!data.value.metrics.available) return "暂时无法读取运行指标，正在重试；历史记录仍可查看。";
  if (!data.value.access_logging) return "访问日志已关闭，响应速率与错误率暂停统计；请求速率与连接数继续采集。";
  return stats.value?.issue || stats.value?.history_issue || "";
});
</script>

<template>
  <div class="dashboard-page">
    <section class="card dashboard-status">
      <div class="dashboard-service">
        <div class="service-emblem" :class="{ stopped: !overview.nginx.running }">
          <PhCheckCircle v-if="overview.nginx.running" :size="78" weight="fill" aria-hidden="true" />
          <PhWarningCircle v-else :size="78" weight="fill" aria-hidden="true" />
        </div>
        <div class="service-copy">
          <h2><span>Nginx</span> {{ overview.nginx.running ? "运行中" : "已停止" }}</h2>
          <p>{{ overview.last_apply_error ? "最近配置应用失败" : overview.dirty ? "有待应用的配置变更" : "配置已同步" }}</p>
          <button v-if="!overview.nginx.running" class="button secondary small" :disabled="busy" @click="emit('start')">启动 Nginx</button>
        </div>
      </div>
      <dl class="service-facts">
        <div><dt>运行时长</dt><dd>{{ uptime(overview.nginx.uptime_seconds) }}</dd></div>
        <div><dt>进程 ID</dt><dd>{{ overview.nginx.running ? num(overview.nginx.pid, 0).replaceAll(',', '') : "—" }}</dd></div>
        <div><dt>工作进程</dt><dd>{{ num(overview.nginx.worker_processes, 0) }}</dd></div>
        <div><dt>活动连接</dt><dd>{{ error ? "—" : num(stats?.connections, 0) }}</dd></div>
        <div><dt>请求/秒</dt><dd>{{ error || selected ? "—" : num(stats?.rps, 2) }}</dd></div>
      </dl>
    </section>

    <div v-if="error" class="notice danger dashboard-notice" role="alert">
      <PhWarningCircle :size="18" /><span>刷新失败：{{ error }}。{{ data ? "下面保留上次数据。" : "" }}</span><button class="button small" @click="load">重试</button>
    </div>
    <div v-if="overview.last_apply_error" class="notice danger dashboard-notice" role="alert">
      <PhWarningCircle :size="18" /><span>最近应用失败：{{ overview.last_apply_error }}</span><button class="button secondary small" @click="emit('navigate', 'config')">查看配置</button>
    </div>
    <div v-if="issue || overview.dirty" class="notice warning dashboard-notice">
      <PhWarningCircle :size="18" /><span>{{ issue || "当前草稿尚未应用，正在转发的服务仍使用上次生效配置。" }}</span>
      <button v-if="!data?.monitoring_ready || overview.dirty" class="button secondary small" :disabled="busy" @click="emit('apply')">保存并应用</button>
    </div>

    <section class="card traffic-card" :aria-busy="fetching">
      <header class="card-header">
        <h2>实时流量</h2>
        <div class="traffic-controls">
          <div class="range-buttons" aria-label="时间范围">
            <button v-for="range in trafficRanges"
              :key="range.value" :class="{ active: minutes === range.value }" :aria-pressed="minutes === range.value" @click="minutes = range.value">{{ range.label }}</button>
          </div>
        </div>
      </header>
      <div class="traffic-metrics">
        <div class="traffic-metric connections-metric" title="HTTP 客户端连接数，包含空闲 Keepalive 连接">
          <span><i></i>当前连接数</span><strong>{{ error ? "—" : num(stats?.connections, 0) }}</strong>
        </div>
        <div v-if="!selected" class="traffic-metric requests-metric" title="Nginx 每秒接收的 HTTP 请求数">
          <span><i></i>请求速率</span><strong>{{ error ? "—" : num(stats?.rps, 2) }} <small>req/s</small></strong>
        </div>
        <div class="traffic-metric responses-metric" title="每秒完成的 HTTP 请求数；长连接结束后才计入">
          <span><i></i>响应速率</span><strong>{{ error ? "—" : num(stats?.response_rps, 2) }} <small>req/s</small></strong>
        </div>
        <div class="traffic-metric completed-metric" :title="period">
          <span><i></i>完成请求</span><strong>{{ stats && (stats.observed_seconds > 0 || stats.counts.requests > 0) ? num(stats.counts.requests, 0) : "—" }} <small>次</small></strong>
        </div>
        <button class="traffic-metric errors-metric" aria-label="查看错误率详情" title="4xx 与 5xx 占已完成请求的比例，点击查看详情" @click="showErrors()">
          <span><i></i>错误率 <PhArrowRight :size="15" class="metric-arrow" /></span><strong>{{ num(stats?.error_rate, 2) }} <small>%</small></strong>
        </button>
      </div>
      <div v-if="selected" class="selected-rule">
        <span>{{ chosenName || "所选规则" }} · 按已完成请求统计</span><button class="button ghost small" @click="selected = ''">返回全部 HTTP</button>
      </div>
      <TrafficChart :points="stats?.points ?? []" :loading="fetching && !stats" :series="series" :height="160" />
      <footer class="traffic-foot">
        <span>{{ period }} · 每 5 秒采集</span>
        <span>最近采集 {{ date(stats?.sampled_at) }}<template v-if="stats"> · 日志覆盖 {{ Math.round(stats.observed_seconds / 60) }} 分钟</template></span>
      </footer>
    </section>

    <section class="card dashboard-rules">
      <header class="rules-heading">
        <h2>代理规则</h2>
        <div class="rules-filters">
          <label class="dashboard-search"><PhMagnifyingGlass :size="17" /><input v-model="search" type="search" aria-label="搜索规则名称、入口或目标" placeholder="搜索域名、路径或目标" /></label>
          <AppSelect v-model="configFilter" class="select" aria-label="配置状态"><option value="all">全部状态</option><option v-for="(name, key) in configNames" :key="key" :value="key">{{ name }}</option></AppSelect>
          <AppSelect v-model="sort" class="select" aria-label="规则排序"><option value="errors">按错误数</option><option value="requests">按请求数</option><option value="name">按名称</option></AppSelect>
        </div>
      </header>
      <div class="protocol-tabs" aria-label="规则类型">
        <button v-for="item in protocols" :key="item.value" :aria-pressed="protocol === item.value" :class="{ active: protocol === item.value }" @click="protocol = item.value">{{ item.label }} ({{ item.count }})</button>
      </div>
      <div v-if="visible.length" class="table-wrap">
        <table class="table dashboard-table">
          <thead><tr><th>名称 / 入口</th><th>类型</th><th>监听地址</th><th>目标地址</th><th>配置状态</th><th>平均请求速率</th><th>错误率</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="rule in visible" :key="rule.id">
              <td><div class="rule-identity"><PhGlobe v-if="rule.protocol === 'HTTP' || rule.protocol === 'HTTPS'" :size="16" /><PhShareNetwork v-else :size="17" />
                <div><button v-if="rule.protocol === 'HTTP' || rule.protocol === 'HTTPS'" class="rule-name-link" :title="`查看 ${rule.name} 的访问趋势`" @click="selected = rule.id">{{ rule.name }}</button><strong v-else>{{ rule.name }}</strong><div class="rule-sub" :title="rule.entry">{{ rule.entry }}</div></div>
              </div></td>
              <td>{{ rule.protocol }}</td><td>{{ rule.listen_address || "—" }}</td><td class="dashboard-target" :title="rule.target">{{ rule.target }}</td>
              <td><span class="rule-status" :class="rule.config_state" :title="rule.config_state === 'pending_delete' ? '草稿已删除，仍在上次生效配置中' : '配置状态不代表服务健康'"><i></i><span>{{ configNames[rule.config_state] }}</span></span></td>
              <td :title="`${period}平均完成请求速率`">{{ num(averageRate(rule), 2) }} <small v-if="averageRate(rule) != null">req/s</small></td>
              <td><button v-if="rule.protocol === 'HTTP' || rule.protocol === 'HTTPS'" class="error-rate-link" :class="{ 'error-count': (totalErrors(rule.counts) ?? 0) > 0 }" :aria-label="`查看 ${rule.name} 的错误率详情`" @click="showErrors(rule.id)">{{ errorRate(rule.counts) == null ? "—" : `${num(errorRate(rule.counts), 2)}%` }}<PhArrowRight :size="13" /></button><span v-else>—</span></td>
              <td><button class="row-action" :aria-label="`管理 ${rule.name}`" :title="rule.config_state === 'pending_delete' ? '查看配置' : '编辑规则'" :disabled="busy" @click="openRule(rule)"><PhPencilSimple :size="18" /></button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else-if="!data" class="empty-state" role="status"><h3>{{ error ? "暂时无法读取规则" : "正在读取规则…" }}</h3><p>{{ error ? "请重试加载首页数据" : "获取配置与访问统计" }}</p></div>
      <div v-else class="empty-state"><h3>{{ rules.length ? "没有符合条件的规则" : "还没有代理规则" }}</h3><p>{{ rules.length ? "调整搜索条件或筛选后重试" : "添加代理规则并应用配置后，在这里查看运行情况" }}</p></div>
      <footer class="dashboard-pagination">
        <span>共 {{ filtered.length }} 条</span>
        <div><button class="page-button" :disabled="page <= 1" aria-label="上一页" @click="page--"><PhCaretLeft :size="15" /></button>
          <button v-for="number in pageNumbers" :key="number" class="page-button" :class="{ active: page === number }" :aria-label="`第 ${number} 页`" :aria-current="page === number ? 'page' : undefined" @click="page = number">{{ number }}</button>
          <button class="page-button" :disabled="page >= pageCount" aria-label="下一页" @click="page++"><PhCaretRight :size="15" /></button>
          <AppSelect v-model="pageSize" class="select page-size" aria-label="每页条数"><option :value="5">5 条/页</option><option :value="10">10 条/页</option><option :value="20">20 条/页</option></AppSelect>
        </div>
      </footer>
    </section>

    <section class="card certificate-attention" :class="{ 'has-alert': data?.certificates.length }">
      <h2>证书状态</h2>
      <div class="certificate-summary">
        <PhWarning v-if="data?.certificates.length" :size="23" /><PhCheckCircle v-else :size="22" />
        <span v-if="!data">{{ error ? "暂时无法读取证书状态" : "正在读取…" }}</span>
        <template v-else-if="data.certificates.length"><strong>发现 {{ data.certificates.length }} 项需要处理</strong><span class="certificate-preview" :title="data.certificates.map(cert => `${cert.name} · ${certificateText(cert)}`).join('；')">{{ data.certificates[0]!.name }} · {{ certificateText(data.certificates[0]!) }}</span></template>
        <span v-else>当前在用证书正常</span>
        <button class="button secondary small" @click="emit('navigate', 'certificates')">查看证书<PhArrowRight :size="17" /></button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.dashboard-page { display: grid; grid-template-columns: minmax(0, 1fr); min-width: 0; gap: 10px; }
.dashboard-page > * { min-width: 0; }
.dashboard-status { display: flex; align-items: center; gap: 30px; min-height: 130px; padding: 17px 28px; }
.dashboard-service { display: flex; align-items: center; gap: 28px; flex: 0 0 38%; min-width: 0; }
.service-emblem { width: 94px; height: 94px; flex: 0 0 94px; display: grid; place-items: center; border: 1px solid #7fd4aa; border-radius: 50%; background: #e8f8ef; color: var(--accent); box-shadow: inset 0 0 0 6px #f7fdf9, inset 0 0 0 11px #b8eace; }
.service-emblem.stopped { color: var(--warning); border-color: #f8d496; background: var(--warning-soft); box-shadow: none; }
.service-copy h2 { margin: 0; font-size: 30px; font-weight: 550; line-height: 1.35; white-space: nowrap; }
.service-copy h2 > span { color: var(--accent-dark); font-weight: 600; }
.service-copy p { margin: 7px 0 0; font-size: 17px; color: #606a77; }
.service-copy > .button { margin-top: 7px; }
.service-facts { display: grid; grid-template-columns: 1.12fr repeat(4, 1fr); flex: 1; margin: 0; min-width: 0; }
.service-facts > div { padding: 0 16px; text-align: center; border-left: 1px solid var(--line); min-width: 0; }
.service-facts > div:first-child { padding-left: 0; border: 0; }
.service-facts > div:last-child { padding-right: 0; }
.service-facts dt { font-size: 13px; color: #46515e; white-space: nowrap; }
.service-facts dd { margin: 12px 0 0; font-size: 17px; font-weight: 520; font-variant-numeric: tabular-nums; white-space: nowrap; }
.dashboard-notice { display: flex; align-items: center; gap: 10px; margin: 0; line-height: 1.6; }
.dashboard-notice svg { flex-shrink: 0; }
.dashboard-notice > span { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.traffic-card { margin-top: 2px; }
.card-header { justify-content: space-between; min-height: 58px; padding: 14px 20px 8px; border-bottom: 0; }
.traffic-card .card-header h2 { font-size: 17px; font-weight: 570; }
h2 { font-size: 17px; font-weight: 570; }
.traffic-controls { display: flex; gap: 20px; align-items: center; }
.range-buttons { display: flex; border: 1px solid var(--line); border-radius: 8px; }
.range-buttons button { border: 1px solid transparent; background: transparent; color: #525d6c; padding: 7px 13px; border-radius: 7px; font-size: 13px; white-space: nowrap; }
.range-buttons button + button { position: relative; }
.range-buttons button:not(.active) + button:not(.active)::before { position: absolute; content: ''; height: 16px; width: 1px; background: var(--line); left: -1px; top: 8px; }
.range-buttons button.active { background: #f7fdf9; border-color: var(--accent); color: var(--accent-dark); }
.row-action { display: inline-flex; align-items: center; justify-content: center; color: #55616f; border: 1px solid var(--line); border-radius: 7px; background: #fff; width: 34px; height: 34px; }
.row-action:hover { color: var(--accent-dark); background: var(--accent-soft); }
.traffic-metrics { display: grid; grid-auto-flow: column; grid-auto-columns: minmax(0, 1fr); padding: 8px 20px 4px; gap: 20px; }
.traffic-metric { text-align: left; border: 0; background: transparent; padding: 0 0 0 1px; color: var(--text); min-width: 0; display: flex; flex-direction: column; gap: 11px; }
.traffic-metric > span { color: #46515d; font-size: 13px; display: flex; align-items: center; gap: 7px; }
.traffic-metric > span i { width: 9px; height: 9px; flex: 0 0 9px; border-radius: 50%; background: var(--accent); }
.requests-metric > span i { background: #00a653; }
.responses-metric > span i { background: #3b82f6; }
.completed-metric > span i { background: #159da7; }
.errors-metric > span i { background: var(--warning); }
.traffic-metric strong { font-size: 22px; line-height: 1; font-weight: 540; font-variant-numeric: tabular-nums; white-space: nowrap; }
.traffic-metric small { font-size: 12px; color: var(--text-muted); font-weight: 400; }
.errors-metric { border-left: 1px solid var(--line); padding-left: 24px; }
.metric-arrow { transition: transform 140ms ease; }
.errors-metric:hover .metric-arrow { transform: translateX(3px); }
.traffic-foot { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px; padding: 0 20px 12px; color: #7a8490; font-size: 10px; }
.selected-rule { display: flex; justify-content: space-between; align-items: center; margin: 10px 20px 0; color: var(--text-muted); font-size: 12px; }
.rules-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 13px 20px 0; flex-wrap: wrap; }
.rules-heading h2 { margin: 0; flex-shrink: 0; align-self: flex-start; line-height: 35px; }
.rules-filters { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; min-width: 0; max-width: 100%; }
.rules-filters .select { width: 134px; font-size: 13px; height: 35px; padding-block: 0; color: #526070; }
.dashboard-search { display: flex; gap: 8px; align-items: center; width: 236px; height: 35px; padding: 0 10px; border: 1px solid var(--line-strong); border-radius: 8px; color: #707b88; background: #fff; }
.dashboard-search > svg { flex-shrink: 0; }
.dashboard-search input { border: 0; background: transparent; outline: 0; width: 100%; min-width: 0; color: var(--text); font-size: 13px; }
.dashboard-search:focus-within { outline: 2px solid var(--accent); outline-offset: 1px; }
.protocol-tabs { display: flex; width: max-content; max-width: calc(100% - 40px); margin: 12px 20px 12px; border: 1px solid var(--line); border-radius: 7px; overflow-x: auto; }
.protocol-tabs button { position: relative; padding: 5px 14px; border: 1px solid transparent; background: transparent; color: #647180; border-radius: 6px; font-size: 12px; line-height: 16px; white-space: nowrap; }
.protocol-tabs button.active { border-color: var(--accent); background: #f4fcf7; color: var(--accent-dark); }
.protocol-tabs button:not(.active) + button:not(.active)::before { content: ''; position: absolute; left: -1px; height: 14px; width: 1px; top: 6px; background: var(--line); }
.dashboard-rules .table-wrap { margin: 0 19px; border: 1px solid var(--line); border-radius: 10px; }
.dashboard-table { min-width: 960px; font-size: 12px; }
.dashboard-table th { padding: 9px 14px; font-size: 12px; font-weight: 450; background: #f8fbfb; color: #4d5867; }
.dashboard-table td { padding: 5px 14px; height: 43px; color: #536071; }
.dashboard-table td:first-child { min-width: 184px; }
.dashboard-table small { font-size: 11px; }
.rule-identity { display: flex; align-items: center; gap: 13px; }
.rule-identity > svg { color: #5d676d; flex-shrink: 0; }
.rule-identity > div { min-width: 0; }
.rule-identity strong { font-size: 12px; color: var(--text); font-weight: 550; }
.rule-sub { max-width: 205px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-top: 3px; font-size: 10px; color: #788493; }
.dashboard-target { max-width: 175px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rule-name-link { border: 0; background: transparent; color: var(--text); padding: 0; font-size: 12px; font-weight: 550; text-align: left; white-space: nowrap; }
.rule-name-link:hover { color: var(--accent-dark); text-decoration: underline; }
.rule-status { display: inline-flex; align-items: center; gap: 7px; white-space: nowrap; color: #6a7582; }
.rule-status > i { width: 7px; height: 7px; background: currentColor; border-radius: 50%; }
.rule-status > span { padding: 3px 7px; border-radius: 5px; background: #f1f4f5; }
.rule-status.applied { color: #009e50; }
.rule-status.applied > span { background: #e6f7ee; }
.rule-status.pending, .rule-status.pending_delete, .rule-status.unknown { color: #e48700; }
.rule-status.pending > span, .rule-status.pending_delete > span, .rule-status.unknown > span { background: #fff5e6; }
.row-action { border: 0; width: 28px; height: 28px; }
.error-rate-link { display: inline-flex; align-items: center; gap: 6px; padding: 4px 0; border: 0; background: transparent; color: var(--text-muted); font: inherit; font-variant-numeric: tabular-nums; }
.error-rate-link.error-count { color: #d7790a; }
.error-rate-link:hover { text-decoration: underline; }
.dashboard-pagination { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 20px; color: #6c7785; font-size: 13px; }
.dashboard-pagination > div { display: flex; gap: 6px; align-items: center; white-space: nowrap; }
.page-button { display: grid; place-items: center; border: 1px solid var(--line); border-radius: 7px; min-width: 32px; height: 32px; background: #fff; color: #465565; font-size: 13px; }
.page-button:disabled { opacity: .4; }
.page-button.active { color: var(--accent-dark); border-color: var(--accent); }
.page-size { height: 33px; width: 98px; padding-block: 0; font-size: 13px; margin-left: 7px; }
.certificate-attention { padding: 9px 20px; }
.certificate-attention h2 { margin: 0; font-size: 15px; font-weight: 500; }
.certificate-summary { display: flex; align-items: center; gap: 14px; padding: 0 8px; color: #414b57; font-size: 13px; min-height: 33px; }
.certificate-summary > svg { color: var(--accent); flex-shrink: 0; }
.certificate-summary > strong { font-size: 15px; font-weight: 550; white-space: nowrap; }
.certificate-summary .button { margin-left: auto; flex-shrink: 0; gap: 14px; min-width: 128px; font-size: 13px; font-weight: 450; background: #fff; }
.certificate-preview { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.certificate-attention.has-alert { border-color: #ffe0a6; margin-top: 3px; }
.has-alert .certificate-summary > strong, .has-alert .certificate-summary > svg { color: var(--warning); }
.has-alert .certificate-summary .button { border-color: #ffe0a6; }
button:focus-visible, select:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
@media (max-width: 1200px) {
  .dashboard-service { flex-basis: 35%; gap: 15px; }
  .dashboard-status { gap: 15px; padding-inline: 20px; }
  .service-emblem { width: 72px; height: 72px; flex-basis: 72px; }
  .service-emblem svg { width: 64px; height: 64px; }
  .service-copy h2 { font-size: 22px; }
  .service-copy p { font-size: 14px; }
  .service-facts > div { padding-inline: 10px; }
  .service-facts dd { font-size: 14px; }
  .rules-filters { gap: 8px; }
  .rules-filters .select { width: 112px; }
  .dashboard-search { width: 205px; }
  .traffic-metrics { gap: 14px; }
  .traffic-metric strong { font-size: 20px; }
}
@media (max-width: 1020px) {
  .dashboard-status { flex-wrap: wrap; gap: 18px; }
  .dashboard-service { flex-basis: 100%; }
  .service-facts { flex-basis: 100%; }
  .rules-heading { flex-wrap: wrap; }
  .rules-filters { width: 100%; }
  .dashboard-search { flex: 1; }
  .protocol-tabs { margin-top: 9px; }
  .traffic-metric > span { font-size: 12px; }
  .traffic-metric strong { font-size: 18px; }
}
@media (max-width: 760px) {
  .dashboard-status { padding: 16px; }
  .service-copy h2 { font-size: 23px; }
  .service-facts { grid-template-columns: repeat(3, minmax(0, 1fr)); row-gap: 16px; }
  .service-facts > div { padding: 0 8px; }
  .service-facts > div:first-child { padding-left: 8px; }
  .service-facts > div:nth-child(4) { border: 0; }
  .service-facts dd { margin-top: 5px; }
  .card-header { padding: 12px 14px 6px; min-height: 52px; }
  .traffic-card > .card-header { flex-wrap: wrap; gap: 8px; }
  .traffic-controls { gap: 8px; max-width: 100%; }
  .range-buttons { min-width: 0; overflow-x: auto; }
  .range-buttons button { padding: 6px 7px; font-size: 12px; flex-shrink: 0; }
  .traffic-metrics { grid-auto-flow: row; grid-template-columns: repeat(3, minmax(0, 1fr)); padding: 10px 14px; gap: 17px 10px; }
  .traffic-metric { gap: 7px; }
  .traffic-metric strong { font-size: 20px; }
  .errors-metric { padding-left: 10px; }
  .traffic-foot { padding: 0 14px 12px; line-height: 1.5; }
  .rules-heading { padding: 12px 14px 0; }
  .rules-filters { flex-wrap: wrap; }
  .dashboard-search { flex-basis: 100%; }
  .rules-filters .select { flex: 1; }
  .protocol-tabs { margin: 9px 14px; max-width: calc(100% - 28px); }
  .protocol-tabs button { padding-inline: 10px; }
  .dashboard-rules .table-wrap { margin-inline: 13px; }
  .dashboard-pagination { padding: 8px 14px; flex-wrap: wrap; }
  .dashboard-pagination > div { margin-left: auto; }
  .certificate-attention { padding: 10px 14px; }
  .certificate-summary { flex-wrap: wrap; padding: 6px 0 0; gap: 8px; }
  .certificate-summary > strong { font-size: 13px; }
  .certificate-preview { flex-basis: 100%; order: 1; }
  .certificate-summary .button { min-width: 102px; gap: 8px; }
}
@media (prefers-reduced-motion: reduce) { .metric-arrow { transition: none; } }
</style>
