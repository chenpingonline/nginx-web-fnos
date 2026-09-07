<script setup lang="ts">
import { computed, ref, useId, watch } from "vue";
import type { MetricPoint } from "../types";
import type { TrafficMetric, TrafficSeries } from "./traffic-chart";

const props = defineProps<{
  points: MetricPoint[];
  metric?: TrafficMetric;
  label?: string;
  unit?: string;
  series?: TrafficSeries[];
  loading?: boolean;
  compact?: boolean;
  height?: number;
}>();
const chartId = useId();
const hover = ref<number | null>(null);
const hidden = ref<TrafficMetric[]>([]);
const multi = computed(() => Boolean(props.series?.length));
const allSeries = computed<TrafficSeries[]>(() =>
  props.series?.length
    ? props.series
    : [
        {
          key: props.metric ?? "rps",
          label: props.label ?? "请求速率",
          unit: props.unit ?? "req/s",
          color: "var(--accent)",
          area: true,
        },
      ],
);
const visibleSeries = computed(() =>
  allSeries.value.filter((item) => !hidden.value.includes(item.key)),
);
const rightAxis = computed(() =>
  visibleSeries.value.some((item) => item.axis === "right"),
);
const axes = computed(() => {
  const result = {
    left: { max: 1, unit: "", visible: false },
    right: { max: 1, unit: "", visible: false },
  };
  for (const side of ["left", "right"] as const) {
    const items = visibleSeries.value.filter(
      (item) => (item.axis ?? "left") === side,
    );
    const peak = Math.max(
      0,
      ...items.flatMap((item) =>
        props.points.map((point) => value(point, item.key) ?? 0),
      ),
    );
    const percent =
      items.length > 0 && items.every((item) => item.unit === "%");
    const minimum = items.every((item) => item.key === "connections") ? 4 : 1;
    const raw = Math.max(minimum, peak * 1.08) / 4;
    const magnitude = 10 ** Math.floor(Math.log10(raw));
    const step =
      [1, 2, 2.5, 5, 10].find((candidate) => candidate * magnitude >= raw) ??
      10;
    result[side] = {
      max: percent
        ? Math.min(100, Math.max(1, Math.ceil(peak / 5) * 5))
        : step * magnitude * 4,
      unit: [...new Set(items.map((item) => item.unit))].join(" / "),
      visible: items.length > 0,
    };
  }
  return result;
});
function value(point: MetricPoint, metric: TrafficMetric): number | null {
  const result = point[metric];
  return result != null && Number.isFinite(result) ? result : null;
}
const x = (index: number) =>
  props.points.length < 2 ? 500 : (index * 1000) / (props.points.length - 1);
const y = (amount: number, item: TrafficSeries) =>
  200 - (amount / axes.value[item.axis ?? "left"].max) * 188;
const plotted = computed(() =>
  visibleSeries.value.map((item) => {
    const values = props.points.map((point) => value(point, item.key));
    const paths: { line: string; area: string }[] = [];
    let points: string[] = [],
      first = 0,
      last = 0;
    const finish = () => {
      if (points.length)
        paths.push({
          line: `M ${points.join(" L ")}`,
          area: `M ${first},200 L ${points.join(" L ")} L ${last},200 Z`,
        });
      points = [];
    };
    values.forEach((amount, index) => {
      if (amount === null) {
        finish();
        return;
      }
      if (!points.length) first = x(index);
      last = x(index);
      points.push(`${last},${y(amount, item)}`);
    });
    finish();
    const isolated = values.flatMap((amount, index) =>
      amount !== null &&
      (index === 0 || values[index - 1] === null) &&
      (index === values.length - 1 || values[index + 1] === null)
        ? [{ x: x(index), y: y(amount, item) }]
        : [],
    );
    return {
      ...item,
      paths,
      isolated,
      hasData: values.some((amount) => amount !== null),
    };
  }),
);
const hasData = computed(() => plotted.value.some((item) => item.hasData));
const active = computed(() =>
  hover.value === null ? undefined : props.points[hover.value],
);
const description = computed(() =>
  visibleSeries.value
    .map(
      (item) =>
        `${item.label}，单位 ${item.unit}${item.axis === "right" ? "，右轴" : ""}`,
    )
    .join("；"),
);
const chartHeight = computed(() =>
  props.height ? `${Math.max(120, props.height)}px` : undefined,
);
function toggle(metric: TrafficMetric) {
  if (hidden.value.includes(metric))
    hidden.value = hidden.value.filter((item) => item !== metric);
  else if (visibleSeries.value.length > 1)
    hidden.value = [...hidden.value, metric];
}
function move(event: PointerEvent) {
  if (!props.points.length) return;
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  if (!rect.width) return;
  hover.value = Math.max(
    0,
    Math.min(
      props.points.length - 1,
      Math.round(
        ((event.clientX - rect.left) / rect.width) * (props.points.length - 1),
      ),
    ),
  );
}
function key(event: KeyboardEvent) {
  if (!["ArrowLeft", "ArrowRight", "Home", "End", "Escape"].includes(event.key))
    return;
  event.preventDefault();
  if (event.key === "Escape" || !props.points.length) {
    hover.value = null;
    return;
  }
  if (event.key === "Home") {
    hover.value = 0;
    return;
  }
  if (event.key === "End") {
    hover.value = props.points.length - 1;
    return;
  }
  hover.value = Math.max(
    0,
    Math.min(
      props.points.length - 1,
      (hover.value ?? props.points.length - 1) +
        (event.key === "ArrowLeft" ? -1 : 1),
    ),
  );
}
const multiDay = computed(() => {
  const first = props.points[0], last = props.points[props.points.length - 1];
  return first && last && Date.parse(last.time) - Date.parse(first.time) > 24 * 60 * 60 * 1000;
});
function time(timestamp: string, detail = false) {
  const date = new Date(timestamp);
  const clock = date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  if (!multiDay.value) return clock;
  const day = date.toLocaleDateString("zh-CN", { month: "2-digit", day: "2-digit" });
  return detail ? `${day} ${clock}` : day;
}
function number(amount: number) {
  return amount.toLocaleString("zh-CN", { maximumFractionDigits: 2 });
}
function tick(amount: number) {
  return amount >= 1000 ? `${number(amount / 1000)}k` : number(amount);
}
const ticks = computed(() => {
  const indices = [
    ...new Set(
      [0, 0.25, 0.5, 0.75, 1].map((fraction) =>
        Math.round(fraction * (props.points.length - 1)),
      ),
    ),
  ];
  return indices
    .map((index) => props.points[index])
    .filter((point): point is MetricPoint => Boolean(point));
});
watch(
  () => allSeries.value.map((item) => item.key).join(","),
  () => {
    hidden.value = hidden.value.filter((key) =>
      allSeries.value.some((item) => item.key === key),
    );
    if (!visibleSeries.value.length) hidden.value = [];
    hover.value = null;
  },
);
watch(
  () => props.points.length,
  () => {
    hover.value = null;
  },
);
</script>

<template>
  <div
    class="traffic-chart"
    :class="{ compact, 'has-right-axis': rightAxis, 'multiple-series': multi }"
    :style="{ '--chart-height': chartHeight }"
  >
    <div
      v-if="multi"
      class="chart-legend"
      role="group"
      aria-label="曲线图例，可点击显示或隐藏"
    >
      <button
        v-for="item in allSeries"
        :key="item.key"
        type="button"
        class="legend-item"
        :class="{ 'series-muted': hidden.includes(item.key) }"
        :aria-pressed="!hidden.includes(item.key)"
        :aria-label="`${item.label}（${item.unit}${item.axis === 'right' ? '，右轴' : ''}）`"
        :title="
          hidden.includes(item.key)
            ? '显示此曲线'
            : visibleSeries.length === 1
              ? '至少保留一条曲线'
              : '隐藏此曲线'
        "
        @click="toggle(item.key)"
      >
        <span
          class="legend-stroke"
          :style="{ backgroundColor: item.color }"
          aria-hidden="true"
        ></span>
        <span>{{ item.label }}</span
        ><small
          >{{ item.unit
          }}<template v-if="item.axis === 'right'"> · 右轴</template></small
        >
      </button>
    </div>
    <div v-if="multi && rightAxis" class="axis-units" aria-hidden="true">
      <span>{{ axes.left.unit }}</span
      ><span v-if="rightAxis">{{ axes.right.unit }}</span>
    </div>
    <div class="chart-scale left-scale" aria-hidden="true">
      <template v-if="axes.left.visible"
        ><span v-for="fraction in [1, 0.75, 0.5, 0.25, 0]" :key="fraction">{{
          tick(axes.left.max * fraction)
        }}</span></template
      >
    </div>
    <div
      class="chart-plot"
      tabindex="0"
      role="img"
      :aria-label="`${description}趋势。左右方向键查看采样点；空白处表示没有采集数据。`"
      @pointermove="move"
      @pointerleave="hover = null"
      @keydown="key"
      @blur="hover = null"
    >
      <svg viewBox="0 0 1000 210" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient
            v-for="item in plotted.filter((item) => item.area)"
            :id="`${chartId}-${item.key}`"
            :key="item.key"
            x1="0"
            y1="0"
            x2="0"
            y2="1"
          >
            <stop offset="0%" :stop-color="item.color" stop-opacity="0.2" />
            <stop offset="100%" :stop-color="item.color" stop-opacity="0.015" />
          </linearGradient>
        </defs>
        <line
          v-for="fraction in [0, 0.25, 0.5, 0.75, 1]"
          :key="fraction"
          x1="0"
          :y1="200 - fraction * 188"
          x2="1000"
          :y2="200 - fraction * 188"
          class="chart-grid"
        />
        <g v-for="item in plotted" :key="`${item.key}-area`">
          <template v-if="item.area">
            <path
              v-for="(segment, index) in item.paths"
              :key="index"
              :d="segment.area"
              :fill="`url(#${chartId}-${item.key})`"
            />
          </template>
        </g>
        <g v-for="item in plotted" :key="item.key" :data-series="item.key">
          <path
            v-for="(segment, index) in item.paths"
            :key="index"
            :d="segment.line"
            class="chart-line"
            :style="{ stroke: item.color }"
          />
          <circle
            v-for="(point, index) in item.isolated"
            :key="`point-${index}`"
            :cx="point.x"
            :cy="point.y"
            r="3"
            :fill="item.color"
          />
          <circle
            v-if="active && hover !== null && value(active, item.key) !== null"
            :cx="x(hover)"
            :cy="y(value(active, item.key)!, item)"
            r="3.5"
            class="chart-active-dot"
            :style="{ fill: item.color }"
          />
        </g>
        <line
          v-if="hover !== null"
          :x1="x(hover)"
          y1="8"
          :x2="x(hover)"
          y2="200"
          class="chart-cursor"
        />
      </svg>
      <div v-if="!hasData" class="chart-empty">
        <strong>{{
          loading
            ? "正在读取趋势数据…"
            : `尚无可展示的${multi ? "流量" : (label ?? "请求速率")}数据`
        }}</strong
        ><span v-if="!loading">采集后自动出现曲线，缺失数据不会填充为 0</span>
      </div>
      <div
        v-if="active"
        class="chart-tooltip"
        :class="{ 'tooltip-left': hover !== null && hover > points.length / 2 }"
        role="status"
      >
        <time>{{ time(active.time, true) }}</time>
        <div v-for="item in visibleSeries" :key="item.key" class="tooltip-row">
          <span
            class="legend-stroke"
            :style="{ backgroundColor: item.color }"
            aria-hidden="true"
          ></span
          ><span>{{ item.label }}</span
          ><strong>{{
            value(active, item.key) === null
              ? "无可用统计"
              : `${number(value(active, item.key)!)} ${item.unit}`
          }}</strong>
        </div>
      </div>
    </div>
    <div v-if="rightAxis" class="chart-scale right-scale" aria-hidden="true">
      <span v-for="fraction in [1, 0.75, 0.5, 0.25, 0]" :key="fraction">{{
        tick(axes.right.max * fraction)
      }}</span>
    </div>
    <div class="chart-times" aria-hidden="true">
      <span v-for="(point, index) in ticks" :key="index">{{
        time(point.time)
      }}</span>
    </div>
  </div>
</template>

<style scoped>
.traffic-chart {
  --chart-height: 210px;
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr);
  padding: 16px 22px 12px 12px;
}
.traffic-chart.has-right-axis {
  grid-template-columns: 44px minmax(0, 1fr) 44px;
}
.chart-legend {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  column-gap: 22px;
  row-gap: 6px;
  min-width: 0;
  padding: 0 0 4px 3px;
}
.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 2px 0;
  border: 0;
  background: transparent;
  color: var(--text-secondary, var(--text-muted));
  font: inherit;
  font-size: 14px;
  line-height: 16px;
  cursor: pointer;
  white-space: nowrap;
  border-radius: 3px;
}
.legend-item small {
  color: var(--text-muted);
  font-size: 12px;
}
.legend-item.series-muted {
  opacity: 0.42;
}
.legend-item.series-muted .legend-stroke {
  background: var(--text-muted) !important;
}
.legend-item:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 4px;
}
.legend-stroke {
  display: inline-block;
  flex: 0 0 auto;
  width: 16px;
  height: 3px;
  border-radius: 3px;
}
.axis-units {
  grid-column: 2;
  display: flex;
  justify-content: space-between;
  min-height: 16px;
  font-size: 12px;
  color: var(--text-muted);
}
.chart-scale {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  height: calc(var(--chart-height) * 188 / 210);
  margin-top: calc(var(--chart-height) * 12 / 210);
  text-align: right;
  padding-right: 12px;
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1;
  font-variant-numeric: tabular-nums;
  transform: translateY(-0.5em);
}
.left-scale {
  grid-column: 1;
}
.right-scale {
  grid-column: 3;
  text-align: left;
  padding-left: 10px;
  padding-right: 0;
}
.chart-plot {
  position: relative;
  grid-column: 2;
  min-width: 0;
  height: var(--chart-height);
  outline-offset: 4px;
}
svg {
  width: 100%;
  height: var(--chart-height);
  overflow: visible;
}
.chart-grid {
  stroke: var(--line);
  stroke-width: 1;
  stroke-dasharray: 2.5 3.5;
  vector-effect: non-scaling-stroke;
}
.chart-line {
  fill: none;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
  vector-effect: non-scaling-stroke;
}
.chart-active-dot {
  stroke: var(--surface-solid, #fff);
  stroke-width: 1.5;
  vector-effect: non-scaling-stroke;
}
.chart-cursor {
  stroke: var(--text-muted);
  stroke-width: 1;
  opacity: 0.55;
  stroke-dasharray: 4 4;
  vector-effect: non-scaling-stroke;
}
.chart-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 8px;
  color: var(--text-muted);
  background: var(--surface);
  border-radius: 9px;
  font-size: 14px;
}
.chart-empty strong {
  color: var(--text);
  font-weight: 550;
}
.chart-tooltip {
  position: absolute;
  z-index: 1;
  top: 6px;
  right: 8px;
  max-width: calc(100% - 16px);
  padding: 9px 12px;
  border-radius: 8px;
  background: var(--surface-solid);
  color: var(--text);
  border: 1px solid var(--line-strong);
  box-shadow: 0 3px 14px rgb(25 54 40 / 8%);
  font-size: 13px;
  pointer-events: none;
}
.chart-tooltip.tooltip-left {
  right: auto;
  left: 8px;
}
.chart-tooltip time {
  display: block;
  color: var(--text-muted);
  margin-bottom: 7px;
  font-variant-numeric: tabular-nums;
}
.tooltip-row {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-top: 5px;
}
.tooltip-row strong {
  margin-left: auto;
  padding-left: 10px;
  font-weight: 550;
  font-variant-numeric: tabular-nums;
}
.chart-times {
  grid-column: 2;
  display: flex;
  justify-content: space-between;
  margin-top: 5px;
  color: var(--text-muted);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}
.compact {
  --chart-height: 180px;
  padding: 10px 18px 8px 8px;
}
.multiple-series {
  padding-top: 8px;
  padding-bottom: 10px;
}
@media (max-width: 520px) {
  .traffic-chart {
    padding-right: 12px;
    grid-template-columns: 34px minmax(0, 1fr);
  }
  .traffic-chart.has-right-axis {
    grid-template-columns: 34px minmax(0, 1fr) 32px;
  }
  .chart-legend {
    column-gap: 14px;
  }
  .legend-item {
    font-size: 13px;
    gap: 5px;
  }
  .legend-item small {
    font-size: 11px;
  }
  .chart-scale {
    padding-right: 7px;
    font-size: 12px;
  }
  .right-scale {
    padding-left: 6px;
    padding-right: 0;
  }
  .chart-empty span {
    max-width: 210px;
    text-align: center;
    line-height: 1.6;
  }
  .chart-times {
    font-size: 12px;
  }
  .chart-times span:nth-child(even) {
    visibility: hidden;
  }
  .chart-tooltip {
    font-size: 12px;
    padding: 8px;
  }
  .tooltip-row strong {
    padding-left: 4px;
  }
}
</style>
