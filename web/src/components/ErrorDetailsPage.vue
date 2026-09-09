<script setup lang="ts">
import AppSelect from "./AppSelect.vue";
import { computed, ref, watch } from "vue";
import {
  PhArrowRight,
  PhArrowClockwise,
  PhCheckCircle,
  PhWarningCircle,
} from "@phosphor-icons/vue";
import { useDashboardData } from "../composables/useDashboardData";
import type { MetricCounts, Overview } from "../types";
import TrafficChart from "./TrafficChart.vue";
import { trafficRanges, trafficPeriod } from "./traffic-ranges";

const props = defineProps<{
  overview: Overview;
  busy: boolean;
  updatedAt: string;
  initialMinutes: number;
  initialRule: string;
}>();
const emit = defineEmits<{
  overview: [value: Overview];
  edit: [id: string];
}>();
const minutes = ref(props.initialMinutes);
const selected = ref(props.initialRule);
type ErrorMetric = "error_rate" | "client_error_rate" | "server_error_rate";
const metric = ref<ErrorMetric>("error_rate");
const page = ref(1);
const { data, error, fetching, stats, rules, load } = useDashboardData({
  minutes,
  selected,
  updatedAt: () => props.updatedAt,
  onOverview: (value) => emit("overview", value),
});
const httpRules = computed(() =>
  rules.value.filter(
    (rule) => rule.protocol === "HTTP" || rule.protocol === "HTTPS",
  ),
);
const scopedRules = computed(() =>
  httpRules.value.filter(
    (rule) => !selected.value || rule.id === selected.value,
  ),
);
const affected = computed(() =>
  scopedRules.value
    .filter(
      (rule) =>
        (rule.counts?.errors ?? 0) + (rule.counts?.client_errors ?? 0) > 0,
    )
    .sort(
      (a, b) =>
        b.counts!.errors +
        (b.counts!.client_errors ?? 0) -
        (a.counts!.errors + (a.counts!.client_errors ?? 0)),
    ),
);
const pageCount = computed(() =>
  Math.max(1, Math.ceil(affected.value.length / 8)),
);
const visible = computed(() =>
  affected.value.slice((page.value - 1) * 8, page.value * 8),
);
watch([minutes, selected], () => {
  page.value = 1;
});
watch(pageCount, (value) => {
  page.value = Math.min(page.value, value);
});
const hasCoverage = computed(
  () =>
    (stats.value?.observed_seconds ?? 0) > 0 ||
    (stats.value?.counts.requests ?? 0) > 0,
);
const counts = computed(() =>
  hasCoverage.value ? stats.value?.counts : undefined,
);
const period = computed(() => trafficPeriod(minutes.value));
const metricNames = {
  error_rate: "总错误率",
  client_error_rate: "4xx 请求错误率",
  server_error_rate: "5xx 服务端错误率",
} satisfies Record<ErrorMetric, string>;
const metricColors = {
  error_rate: "color-mix(in srgb, var(--warning) 45%, var(--danger))",
  client_error_rate: "var(--warning)",
  server_error_rate: "var(--danger)",
} satisfies Record<ErrorMetric, string>;
const incomplete = computed(
  () => hasCoverage.value && counts.value?.client_errors == null,
);
const hasUnassignedErrors = computed(
  () => (counts.value?.errors ?? 0) + (counts.value?.client_errors ?? 0) > 0,
);
const scopeName = computed(() =>
  selected.value
    ? (httpRules.value.find((rule) => rule.id === selected.value)?.name ??
      "所选规则")
    : "全部 HTTP / HTTPS",
);
const issue = computed(() => {
  if (!data.value) return "";
  if (!data.value.monitoring_ready)
    return "统计尚未启用，返回总览保存并应用配置后开始采集；当前草稿也会一并应用。";
  if (!data.value.access_logging)
    return "访问日志已关闭，错误统计暂停采集；下方保留已采集的历史记录。";
  if (!(data.value.overview ?? props.overview).nginx.running)
    return "Nginx 已停止，下方展示已采集的历史记录。";
  return stats.value?.issue || stats.value?.history_issue || "";
});
function number(value: number | null | undefined, digits = 0) {
  return value == null
    ? "—"
    : value.toLocaleString("zh-CN", { maximumFractionDigits: digits });
}
function rate(value: number | null | undefined) {
  return value == null ? "—" : `${number(value, 2)}%`;
}
function total(value: MetricCounts | null | undefined) {
  return value?.client_errors == null
    ? null
    : value.client_errors + value.errors;
}
function ruleRate(value: MetricCounts | null) {
  const errors = total(value);
  return errors == null || !value?.requests
    ? null
    : (errors / value.requests) * 100;
}
function date(value: string | null | undefined) {
  return value
    ? new Date(value).toLocaleString("zh-CN", {
        month: "2-digit",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
      })
    : "—";
}
</script>

<template>
  <div class="error-details-page">
    <header class="details-heading">
      <div>
        <h1>请求详情</h1>
        <p>查看 HTTP / HTTPS 请求统计、错误趋势与受影响规则</p>
      </div>
    </header>

    <div v-if="error" class="details-notice error-notice" role="alert">
      <PhWarningCircle :size="18" /><span
        >数据更新失败：{{ error
        }}<template v-if="data"> · 当前显示上次成功获取的数据</template></span
      >
      <button class="button ghost small" :disabled="fetching" @click="load()">
        <PhArrowClockwise :size="14" />重试
      </button>
    </div>
    <p v-if="issue" class="details-notice" role="status">
      <PhWarningCircle :size="18" /><span>{{ issue }}</span>
    </p>

    <section class="card error-trend">
      <header class="card-header">
        <div>
          <h2>错误趋势</h2>
          <p>{{ scopeName }} · {{ period }}</p>
        </div>
        <div class="details-controls">
          <AppSelect v-model="selected" class="select details-rule-select" aria-label="请求统计规则">
            <option value="">全部 HTTP / HTTPS</option>
            <option v-for="rule in httpRules" :key="rule.id" :value="rule.id">
              {{ rule.name }}
            </option>
          </AppSelect>
          <div class="range-buttons" aria-label="错误统计时间范围">
            <button
              v-for="range in trafficRanges"
              :key="range.value"
              :class="{ active: minutes === range.value }"
              :aria-pressed="minutes === range.value"
              @click="minutes = range.value"
            >
              {{ range.label }}
            </button>
          </div>
        </div>
      </header>

      <div class="error-metrics">
        <button
          class="error-metric"
          :class="{ active: metric === 'error_rate' }"
          :aria-pressed="metric === 'error_rate'"
          @click="metric = 'error_rate'"
        >
          <span><i class="total-dot"></i>总错误率</span
          ><strong>{{ rate(stats?.error_rate) }}</strong
          ><small>{{ number(total(counts)) }} 次错误 · 4xx + 5xx</small>
        </button>
        <button
          class="error-metric"
          :class="{ active: metric === 'client_error_rate' }"
          :aria-pressed="metric === 'client_error_rate'"
          @click="metric = 'client_error_rate'"
        >
          <span><i class="client-dot"></i>4xx 请求错误</span
          ><strong>{{ rate(stats?.client_error_rate) }}</strong
          ><small>{{ number(counts?.client_errors) }} 次 · 如 403、404</small>
        </button>
        <button
          class="error-metric"
          :class="{ active: metric === 'server_error_rate' }"
          :aria-pressed="metric === 'server_error_rate'"
          @click="metric = 'server_error_rate'"
        >
          <span><i class="server-dot"></i>5xx 服务端错误</span
          ><strong>{{ rate(stats?.server_error_rate) }}</strong
          ><small>{{ number(counts?.errors) }} 次 · 如 502、504</small>
        </button>
        <div class="error-metric request-metric">
          <span>完成请求</span><strong>{{ number(counts?.requests) }}</strong
          ><small>本时段已记录的 HTTP 请求</small>
        </div>
      </div>

      <TrafficChart
        :points="stats?.points ?? []"
        :metric="metric"
        :label="metricNames[metric]"
        :color="metricColors[metric]"
        unit="%"
        :loading="fetching && !stats"
      />
      <footer class="trend-footer">
        <span>{{ metricNames[metric] }} = 对应错误次数 ÷ 完成请求数</span
        ><span>最近采集 {{ date(stats?.sampled_at) }}</span>
      </footer>
      <p v-if="incomplete" class="coverage-note">
        所选时段包含尚未采集 4xx 的旧记录，4xx 和总错误率暂不完整；5xx
        仍可查看。可选择较短时段查看新采集的数据。
      </p>
      <p class="metric-explanation">
        4xx 表示请求错误，例如无访问权限或页面不存在；5xx
        表示服务端处理失败，例如后端不可用或响应超时。没有请求或采集不足时显示「—」。
      </p>
    </section>

    <section class="card affected-rules">
      <header class="card-header">
        <div>
          <h2>
            受影响规则 <span class="affected-count">{{ affected.length }}</span>
          </h2>
          <p>按错误次数排序 · {{ period }}</p>
        </div>
      </header>
      <div v-if="visible.length" class="table-wrap">
        <table class="table error-table">
          <thead>
            <tr>
              <th>名称 / 入口</th>
              <th>4xx 请求错误</th>
              <th>5xx 服务端错误</th>
              <th>合计 / 错误率</th>
              <th>时段内最近访问</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rule in visible" :key="rule.id">
              <td>
                <button
                  class="rule-name"
                  :title="`查看 ${rule.name} 的错误趋势`"
                  @click="selected = rule.id"
                >
                  {{ rule.name }}
                </button>
                <div class="rule-entry" :title="rule.entry">
                  {{ rule.entry }}
                </div>
                <span
                  v-if="rule.config_state === 'pending_delete'"
                  class="pending-delete"
                  >待删除 · 仍在生效配置</span
                >
              </td>
              <td
                :class="{
                  'client-count': (rule.counts?.client_errors ?? 0) > 0,
                }"
              >
                {{ number(rule.counts?.client_errors) }}
              </td>
              <td :class="{ 'server-count': (rule.counts?.errors ?? 0) > 0 }">
                {{ number(rule.counts?.errors) }}
              </td>
              <td>
                {{ number(total(rule.counts)) }}
                <span class="rate-divider">/</span>
                {{ rate(ruleRate(rule.counts)) }}
              </td>
              <td>{{ date(rule.counts?.last_seen) }}</td>
              <td>
                <button
                  class="button ghost small"
                  :disabled="busy || rule.config_state === 'pending_delete'"
                  :title="
                    rule.config_state === 'pending_delete'
                      ? '该规则已从草稿删除，可返回总览查看配置'
                      : `管理 ${rule.name}`
                  "
                  @click="emit('edit', rule.id)"
                >
                  管理规则<PhArrowRight :size="13" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else-if="fetching && !stats" class="empty-state" role="status">
        <h3>正在读取错误统计…</h3>
        <p>获取所选规则和时间范围内的数据</p>
      </div>
      <div v-else-if="error && !stats" class="empty-state">
        <h3>暂时无法读取错误统计</h3>
        <p>重试获取数据，或稍后再查看</p>
      </div>
      <div v-else-if="!hasCoverage" class="empty-state">
        <h3>本时段暂无错误统计</h3>
        <p>启用访问日志并开始采集后，这里会展示出现错误的规则</p>
      </div>
      <div v-else-if="hasUnassignedErrors" class="empty-state">
        <h3>暂无可归属到规则的错误</h3>
        <p>
          全局已记录错误，但未关联到当前代理规则；默认站点产生的错误也会计入全局统计
        </p>
      </div>
      <div v-else class="empty-state clear-state">
        <PhCheckCircle :size="24" />
        <h3>
          {{ incomplete ? "暂未观测到错误" : "本时段未观测到 HTTP 错误" }}
        </h3>
        <p>
          {{
            incomplete
              ? "4xx 历史统计尚不完整，已采集的记录中没有发现错误"
              : "所选范围内没有出现 4xx 或 5xx 的规则"
          }}
        </p>
      </div>
      <footer class="rules-footer">
        <span>全局统计包含默认站点；表格仅展示能归属到规则的错误</span>
        <div v-if="pageCount > 1" class="pagination">
          <button
            class="button ghost small"
            :disabled="page <= 1"
            @click="page--"
          >
            上一页</button
          ><span>{{ page }} / {{ pageCount }}</span
          ><button
            class="button ghost small"
            :disabled="page >= pageCount"
            @click="page++"
          >
            下一页
          </button>
        </div>
      </footer>
    </section>
  </div>
</template>

<style scoped>
.error-details-page {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 12px;
  min-width: 0;
}
.error-details-page > * {
  min-width: 0;
}
.details-heading {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 2px 0 4px;
}
h1 {
  font-size: 21px;
  font-weight: 650;
  line-height: 1.4;
  margin: 0;
}
.details-heading p {
  color: var(--text-muted);
  font-size: 14px;
  margin: 3px 0 0;
}
.details-notice {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 0;
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 10px 14px;
  color: var(--text-muted);
  font-size: 14px;
  line-height: 1.6;
}
.details-notice > svg {
  flex-shrink: 0;
}
.details-notice > span {
  flex: 1;
  min-width: 0;
  overflow-wrap: anywhere;
}
.error-notice {
  color: var(--danger);
}
.card-header {
  padding: 14px 18px;
  justify-content: space-between;
  gap: 12px;
}
.card-header p {
  line-height: 1.5;
}
.details-controls {
  min-width: 0;
  margin-left: auto;
  justify-content: flex-end;
  flex-wrap: wrap;
  display: flex;
  align-items: center;
  gap: 8px;
}
.details-controls .details-rule-select {
  flex: 0 1 220px;
  width: 220px;
  min-width: 160px;
  max-width: 100%;
  font-size: 14px;
}
.range-buttons {
  max-width: 100%;
  overflow-x: auto;
  display: flex;
  border: 1px solid var(--line);
  border-radius: 9px;
  padding: 3px;
  background: var(--surface-soft);
}
.range-buttons button {
  flex-shrink: 0;
  border: 0;
  background: transparent;
  color: var(--text-muted);
  padding: 6px 10px;
  font-size: 13px;
  border-radius: 6px;
  white-space: nowrap;
}
.range-buttons button.active {
  color: var(--text);
  background: var(--surface-solid);
  box-shadow: 0 1px 4px #0001;
}
.error-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  padding: 5px 18px 3px;
  gap: 8px;
}
.error-metric {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 5px;
  min-width: 0;
  padding: 9px 10px;
  text-align: left;
  color: var(--text);
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
}
.error-metric.active {
  background: var(--accent-soft);
  border-color: var(--line);
}
.error-metric > span {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--text-muted);
}
.error-metric i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  display: inline-block;
}
.total-dot {
  background: color-mix(in srgb, var(--warning) 45%, var(--danger));
}
.client-dot {
  background: var(--warning);
}
.server-dot {
  background: var(--danger);
}
.error-metric strong {
  font-size: 26px;
  font-weight: 600;
  line-height: 1.4;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.error-metric small {
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.5;
}
.request-metric {
  cursor: default;
}
.trend-footer {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 6px;
  padding: 0 20px 12px;
  font-size: 13px;
  color: var(--text-muted);
}
.metric-explanation {
  margin: 0;
  padding: 11px 18px;
  border-top: 1px solid var(--line);
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.7;
}
.coverage-note {
  margin: 0 18px 12px;
  color: var(--warning);
  font-size: 14px;
  line-height: 1.6;
}
.affected-count {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-muted);
  margin-left: 5px;
}
.table-wrap {
  margin: 0 18px;
  max-width: calc(100% - 36px);
  overflow-x: auto;
}
.error-table {
  min-width: 830px;
}
.error-table td {
  padding-top: 11px;
  padding-bottom: 11px;
  font-variant-numeric: tabular-nums;
}
.rule-name {
  border: 0;
  padding: 0;
  background: none;
  text-align: left;
  color: var(--text);
  font-size: 15px;
  font-weight: 550;
}
.rule-name:hover {
  color: var(--accent);
}
.rule-entry {
  margin-top: 4px;
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-muted);
  font-size: 13px;
}
.pending-delete {
  color: var(--warning);
  font-size: 12px;
  line-height: 1.8;
}
.client-count {
  color: var(--warning);
}
.server-count {
  color: var(--danger);
}
.rate-divider {
  color: var(--text-muted);
  margin: 0 3px;
}
.clear-state > svg {
  color: var(--accent);
  margin-bottom: 8px;
}
.rules-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 18px;
  color: var(--text-muted);
  font-size: 13px;
}
.pagination {
  display: flex;
  align-items: center;
  gap: 10px;
}
@media (max-width: 1000px) {
  .error-trend > .card-header {
    align-items: flex-start;
    flex-direction: column;
  }
  .details-controls {
    width: 100%;
    margin-left: 0;
    justify-content: flex-start;
  }
}
@media (max-width: 600px) {
  .details-heading {
    gap: 10px;
  }
  .details-heading p {
    display: none;
  }
  h1 {
    font-size: 19px;
  }
  .card-header {
    padding: 12px 14px;
  }
  .details-controls {
    flex-wrap: wrap;
  }
  .details-controls .details-rule-select {
    flex: 1;
    min-width: 150px;
  }
  .error-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    padding: 4px 12px;
    gap: 3px;
  }
  .error-metric strong {
    font-size: 24px;
  }
  .table-wrap {
    margin: 0 12px;
    max-width: calc(100% - 24px);
  }
  .rules-footer,
  .metric-explanation {
    padding: 10px 14px;
  }
  .trend-footer {
    padding: 0 14px 12px;
  }
}
</style>
