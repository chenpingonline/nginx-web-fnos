<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import {
  PhArrowClockwise,
  PhArrowRight,
  PhArrowsLeftRight,
  PhBroadcast,
  PhCertificate,
  PhCheckCircle,
  PhGlobeHemisphereWest,
  PhInfo,
  PhList,
  PhShieldCheck,
  PhShareNetwork,
  PhWarningCircle,
  PhXCircle,
} from "@phosphor-icons/vue";
import { errorMessage, jsonBody, request } from "./api";
import brandIcon from "./assets/nginx-web.png";
import RuleForm from "./components/RuleForm.vue";
import CertificateForm from "./components/CertificateForm.vue";
import UpstreamPoolsPage from "./components/UpstreamPoolsPage.vue";
import RateLimitPoliciesPage from "./components/RateLimitPoliciesPage.vue";
import RuntimeSettingsForm from "./components/RuntimeSettingsForm.vue";
import SidebarIcon from "./components/SidebarIcon.vue";
import StreamRulesPage from "./components/StreamRulesPage.vue";
import type {
  ApplyResult,
  CertificateInput,
  GeneratedConfig,
  LogResponse,
  LogType,
  NginxAction,
  Overview,
  Page,
  ProxyRule,
  ProxyRuleInput,
  RateLimitPolicy,
  RateLimitPolicyInput,
  Revision,
  Settings,
  State,
  StreamRule,
  StreamRuleInput,
  Toast,
  UpstreamPool,
  UpstreamPoolInput,
} from "./types";
import { formatDate, formatHost, shortFingerprint } from "./utils";

type SidebarIconName =
  | "overview"
  | "http"
  | "stream"
  | "upstreams"
  | "rate-limit"
  | "certificate"
  | "logs"
  | "history"
  | "config"
  | "settings";
const pages: { id: Page; icon: SidebarIconName; label: string; subtitle: string }[] = [
  {
    id: "dashboard",
    icon: "overview",
    label: "总览",
    subtitle: "查看代理服务与配置状态",
  },
  {
    id: "rules",
    icon: "http",
    label: "HTTP/HTTPS 代理",
    subtitle: "管理域名、监听端口与后端服务",
  },
  {
    id: "streams",
    icon: "stream",
    label: "TCP/UDP 代理",
    subtitle: "管理四层转发、TLS 终止与 SNI 透传",
  },
  {
    id: "upstreams",
    icon: "upstreams",
    label: "后端服务池",
    subtitle: "管理负载均衡、节点健康参数与连接复用",
  },
  {
    id: "rate-limits",
    icon: "rate-limit",
    label: "限流策略",
    subtitle: "复用限流参数并保持规则额度独立",
  },
  {
    id: "certificates",
    icon: "certificate",
    label: "HTTPS 证书",
    subtitle: "导入并管理手动 TLS 证书",
  },
  {
    id: "logs",
    icon: "logs",
    label: "运行日志",
    subtitle: "查看 Nginx 与管理服务日志",
  },
  {
    id: "revisions",
    icon: "history",
    label: "配置历史",
    subtitle: "恢复或清理已应用的配置版本",
  },
  {
    id: "config",
    icon: "config",
    label: "Nginx 配置",
    subtitle: "查看 nginx-web 实际生成的配置文件",
  },
  {
    id: "settings",
    icon: "settings",
    label: "全局设置",
    subtitle: "调整默认端口与历史保留策略",
  },
];
const page = ref<Page>("dashboard"),
  overview = ref<Overview | null>(null),
  state = ref<State | null>(null),
  revisions = ref<Revision[]>([]),
  config = ref<GeneratedConfig | null>(null);
const loading = ref(true),
  busy = ref(false),
  connectionError = ref(""),
  menuOpen = ref(false),
  ruleSearch = ref(""),
  logType = ref<LogType>("error"),
  logLines = ref<string[]>([]),
  logsLoading = ref(false),
  configTab = ref("master");
type Theme = "light" | "dark";
const systemThemeQuery = window.matchMedia("(prefers-color-scheme: dark)");
function applySystemTheme(matches = systemThemeQuery.matches) {
  const theme: Theme = matches ? "dark" : "light";
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
}
function systemThemeChanged(event: MediaQueryListEvent) {
  applySystemTheme(event.matches);
}
applySystemTheme();
const modal = ref<"rule" | "certificate" | null>(null),
  editingRule = ref<ProxyRule | null>(null),
  toasts = ref<Toast[]>([]),
  logView = ref<HTMLElement | null>(null),
  toastID = ref(0);
const confirmBox = reactive({
  open: false,
  title: "",
  message: "",
  resolve: null as ((value: boolean) => void) | null,
});
const currentPage = computed(
  () => pages.find((item) => item.id === page.value) ?? pages[0]!,
);
const filteredRules = computed(() => {
  const rules = state.value?.rules ?? [],
    term = ruleSearch.value.trim().toLowerCase();
  return rules.filter(
    (rule) =>
      !term ||
      [
        rule.name,
        ...rule.domains,
        rule.upstream_host,
        String(rule.upstream_port),
      ]
        .join(" ")
        .toLowerCase()
        .includes(term),
  );
});
const activeRules = computed(() =>
  (state.value?.rules ?? []).filter((rule) => rule.enabled),
);
function formatRuleEntry(rule: ProxyRule): string {
  const domain = rule.domains[0];
  if (!domain) return `未配置域名:${rule.listen_port}`;
  if (domain === "*") return `所有域名:${rule.listen_port}`;
  return `${formatHost(domain)}:${rule.listen_port}`;
}
const configKeys = computed(() => [
  "master",
  ...Object.keys(config.value?.files ?? {}).sort(),
]);
const configText = computed(() =>
  configTab.value === "master"
    ? (config.value?.master ?? "")
    : (config.value?.files[configTab.value] ?? ""),
);
const ports = computed(() =>
  overview.value?.nginx.ports.length
    ? overview.value.nginx.ports.join(", ")
    : "—",
);
const dashboardMetrics = computed(() => [
  {
    label: "HTTP/HTTPS 代理",
    value: overview.value?.rule_count ?? 0,
    foot: `${overview.value?.enabled_count ?? 0} 条已启用`,
    icon: PhGlobeHemisphereWest,
    tone: "green",
  },
  {
    label: "TCP/UDP 代理",
    value: state.value?.stream_rules.length ?? 0,
    foot: `${state.value?.stream_rules.filter((rule) => rule.enabled).length ?? 0} 条已启用`,
    icon: PhBroadcast,
    tone: "cyan",
  },
  {
    label: "HTTPS 证书",
    value: overview.value?.certificate_count ?? 0,
    foot: certificateFoot(),
    icon: PhShieldCheck,
    tone: "green",
  },
]);
const recentEvents = computed(() => {
  const events = (state.value?.rules ?? []).map((rule) => ({
    id: rule.id,
    title: rule.enabled ? `代理已启用 · ${rule.name}` : `代理已保存 · ${rule.name}`,
    detail: rule.domains.join(", ") || `${formatHost(rule.upstream_host)}:${rule.upstream_port}`,
    time: rule.updated_at,
    tone: rule.enabled ? "success" : "neutral",
  }));
  if (state.value?.last_applied_at) {
    events.push({
      id: "last-applied",
      title: "配置已校验并应用",
      detail: state.value.last_apply_message || "独立 Nginx 已平滑重载",
      time: state.value.last_applied_at,
      tone: "success",
    });
  }
  return events
    .sort((a, b) => new Date(b.time).getTime() - new Date(a.time).getTime())
    .slice(0, 5);
});

function toast(message: string, type: Toast["type"] = "") {
  const id = ++toastID.value;
  toasts.value.push({ id, message, type });
  window.setTimeout(
    () => (toasts.value = toasts.value.filter((item) => item.id !== id)),
    5200,
  );
}
function setPage(value: Page) {
  page.value = value;
  menuOpen.value = false;
}
async function loadCore(quiet = false) {
  if (!quiet) {
    loading.value = true;
    busy.value = true;
  }
  try {
    const [o, s, r] = await Promise.all([
      request<Overview>("/overview"),
      request<State>("/state"),
      request<Revision[]>("/revisions"),
    ]);
    overview.value = o;
    state.value = s;
    revisions.value = r ?? [];
    connectionError.value = "";
  } catch (error) {
    connectionError.value = errorMessage(error);
    if (quiet) toast(connectionError.value, "error");
  } finally {
    loading.value = false;
    busy.value = false;
  }
}
async function loadLogs() {
  logsLoading.value = true;
  try {
    const result = await request<LogResponse>(
      `/logs?type=${encodeURIComponent(logType.value)}&lines=500`,
    );
    logLines.value = result.lines ?? [];
    await nextTick();
    if (logView.value) logView.value.scrollTop = logView.value.scrollHeight;
  } catch (error) {
    logLines.value = [`读取失败：${errorMessage(error)}`];
  } finally {
    logsLoading.value = false;
  }
}
async function loadConfig() {
  try {
    config.value = await request<GeneratedConfig>("/config");
    if (!configKeys.value.includes(configTab.value)) configTab.value = "master";
  } catch (error) {
    toast(errorMessage(error), "error");
  }
}
async function mutate<T>(fn: () => Promise<T>, success?: string) {
  if (busy.value) return;
  busy.value = true;
  try {
    const result = await fn();
    if (success) toast(success, "success");
    return result;
  } catch (error) {
    toast(errorMessage(error), "error");
    return undefined;
  } finally {
    busy.value = false;
  }
}
async function runNginxAction(action: NginxAction) {
  const labels = { start: "启动", stop: "停止", reload: "重载", test: "校验" };
  const result = await mutate(() =>
    request<ApplyResult>(`/nginx/${action}`, { method: "POST", body: "{}" }),
  );
  if (result) {
    toast(result.message || `${labels[action]}成功`, "success");
    await loadCore(true);
  }
}
async function applyConfiguration() {
  const result = await mutate(() =>
    request<ApplyResult>("/apply", {
      method: "POST",
      body: jsonBody({ summary: "从管理页面保存并应用" }),
    }),
  );
  if (result) {
    toast(result.message || "配置已应用", "success");
    await loadCore(true);
  }
}
function openRule(rule: ProxyRule | null = null) {
  editingRule.value = rule;
  modal.value = "rule";
}
function openCertificate() {
  modal.value = "certificate";
}
function closeModal() {
  if (!busy.value) modal.value = null;
}
async function saveRule(value: ProxyRuleInput, applyAfter: boolean) {
  const id = editingRule.value?.id;
  const saved = await mutate(async () => {
    await request(id ? `/rules/${id}` : "/rules", {
      method: id ? "PUT" : "POST",
      body: jsonBody(value),
    });
    if (applyAfter)
      await request("/apply", {
        method: "POST",
        body: jsonBody({
          summary: `${id ? "修改" : "新增"}规则：${value.name}`,
        }),
      });
    return true;
  });
  if (saved) {
    modal.value = null;
    toast(applyAfter ? "规则已保存并应用" : "规则已保存为草稿", "success");
    await loadCore(true);
  }
}
async function importCertificate(value: CertificateInput) {
  const saved = await mutate(() =>
    request("/certificates", { method: "POST", body: jsonBody(value) }),
  );
  if (saved !== undefined) {
    modal.value = null;
    toast("证书已安全导入", "success");
    await loadCore(true);
  }
}
function ask(title: string, message: string) {
  confirmBox.title = title;
  confirmBox.message = message;
  confirmBox.open = true;
  return new Promise<boolean>((resolve) => (confirmBox.resolve = resolve));
}
function answerConfirm(value: boolean) {
  confirmBox.open = false;
  confirmBox.resolve?.(value);
  confirmBox.resolve = null;
}
async function deleteRule(rule: ProxyRule) {
  if (
    !(await ask(
      "删除代理规则",
      `确定删除“${rule.name}”吗？删除后仍需应用配置才会影响运行中的 Nginx。`,
    ))
  )
    return;
  const ok = await mutate(
    () => request(`/rules/${rule.id}`, { method: "DELETE" }),
    "规则已删除",
  );
  if (ok !== undefined) await loadCore(true);
}
async function toggleRule(rule: ProxyRule, enabled: boolean) {
  const payload: ProxyRuleInput = { ...rule, enabled };
  const ok = await mutate(() =>
    request(`/rules/${rule.id}`, { method: "PUT", body: jsonBody(payload) }),
  );
  if (ok !== undefined) {
    toast(enabled ? "规则已启用，等待应用" : "规则已停用，等待应用", "success");
    await loadCore(true);
  }
}
async function deleteCertificate(id: string, name: string) {
  if (
    !(await ask(
      "删除 HTTPS 证书",
      `确定删除“${name}”吗？该操作会同时删除本机保存的私钥，且无法恢复。`,
    ))
  )
    return;
  const ok = await mutate(
    () => request(`/certificates/${id}`, { method: "DELETE" }),
    "证书已删除",
  );
  if (ok !== undefined) await loadCore(true);
}
async function stopNginx() {
  if (
    await ask(
      "停止独立 Nginx",
      "停止后所有由 nginx-web 提供的代理入口都会暂时不可访问，但不会影响飞牛系统服务。",
    )
  )
    await runNginxAction("stop");
}
async function revisionAction(
  revision: Revision,
  action: "restore" | "delete",
) {
  const restoring = action === "restore";
  if (
    !(await ask(
      restoring ? "恢复配置历史" : "删除配置历史",
      restoring
        ? "历史版本会恢复为草稿，不会立刻影响当前代理。"
        : "删除后不能再恢复该版本。",
    ))
  )
    return;
  const ok = await mutate(
    () =>
      request(
        `/revisions/${encodeURIComponent(revision.id)}${restoring ? "/restore" : ""}`,
        {
          method: restoring ? "POST" : "DELETE",
          body: restoring ? "{}" : undefined,
        },
      ),
    restoring ? "历史版本已恢复为草稿" : "配置历史已删除",
  );
  if (ok !== undefined) await loadCore(true);
}
async function saveSettings(value: Settings) {
  const ok = await mutate(
    () => request("/settings", { method: "PUT", body: jsonBody(value) }),
    "设置已保存为草稿",
  );
  if (ok !== undefined) await loadCore(true);
}
async function clearCache() {
  if (!(await ask("清理代理缓存", "将删除 nginx-web 生成的全部 HTTP 缓存文件，正在处理的请求可能重新回源。"))) return;
  await mutate(() => request("/cache", { method: "DELETE" }), "代理缓存已清理");
}
async function rotateLogs() {
  const ok = await mutate(() => request("/logs/rotate", { method: "POST", body: "{}" }), "日志已轮转");
  if (ok !== undefined) await loadLogs();
}
async function saveUpstreamPool(value: UpstreamPoolInput, id: string) {
  const ok = await mutate(
    () =>
      request(id ? `/upstreams/${id}` : "/upstreams", {
        method: id ? "PUT" : "POST",
        body: jsonBody(value),
      }),
    id ? "后端服务池已更新" : "后端服务池已创建",
  );
  if (ok !== undefined) await loadCore(true);
}
async function removeUpstreamPool(pool: UpstreamPool) {
  if (
    !(await ask(
      "删除后端服务池",
      `确定删除“${pool.name}”吗？正在使用的后端服务池不能删除。`,
    ))
  )
    return;
  const ok = await mutate(
    () => request(`/upstreams/${pool.id}`, { method: "DELETE" }),
    "后端服务池已删除",
  );
  if (ok !== undefined) await loadCore(true);
}
async function saveRateLimitPolicy(value: RateLimitPolicyInput, id: string) {
  const ok = await mutate(
    () =>
      request(id ? `/rate-limit-policies/${id}` : "/rate-limit-policies", {
        method: id ? "PUT" : "POST",
        body: jsonBody(value),
      }),
    id ? "限流策略已更新" : "限流策略已创建",
  );
  if (ok !== undefined) await loadCore(true);
}
async function removeRateLimitPolicy(policy: RateLimitPolicy) {
  if (
    !(await ask(
      "删除限流策略",
      `确定删除“${policy.name}”吗？正在使用的策略不能删除。`,
    ))
  )
    return;
  const ok = await mutate(
    () => request(`/rate-limit-policies/${policy.id}`, { method: "DELETE" }),
    "限流策略已删除",
  );
  if (ok !== undefined) await loadCore(true);
}
async function saveStreamRule(value: StreamRuleInput, id: string) {
  const ok = await mutate(
    () =>
      request(id ? `/streams/${id}` : "/streams", {
        method: id ? "PUT" : "POST",
        body: jsonBody(value),
      }),
    id ? "Stream 规则已更新" : "Stream 规则已创建",
  );
  if (ok !== undefined) await loadCore(true);
}
async function removeStreamRule(rule: StreamRule) {
  if (!(await ask("删除 TCP/UDP 规则", `确定删除“${rule.name}”吗？`))) return;
  const ok = await mutate(
    () => request(`/streams/${rule.id}`, { method: "DELETE" }),
    "Stream 规则已删除",
  );
  if (ok !== undefined) await loadCore(true);
}
async function toggleStreamRule(rule: StreamRule, enabled: boolean) {
  const ok = await mutate(() =>
    request(`/streams/${rule.id}`, {
      method: "PUT",
      body: jsonBody({ ...rule, enabled }),
    }),
  );
  if (ok !== undefined) {
    toast(
      enabled ? "Stream 规则已启用，等待应用" : "Stream 规则已停用，等待应用",
      "success",
    );
    await loadCore(true);
  }
}
async function copyConfig() {
  try {
    await navigator.clipboard.writeText(configText.value);
    toast("配置已复制", "success");
  } catch {
    toast("浏览器不允许复制，请手动选择文本", "error");
  }
}
function certificateState(notAfter: string) {
  const end = new Date(notAfter).getTime(),
    expired = end < Date.now(),
    soon = !expired && end - Date.now() < 30 * 86400000;
  return {
    label: expired ? "已过期" : soon ? "即将过期" : "有效",
    className: expired ? "danger" : soon ? "warning" : "success",
  };
}
function certificateFoot() {
  const certs = state.value?.certificates ?? [];
  return certs.length
    ? `${certs.filter((cert) => new Date(cert.not_after).getTime() > Date.now()).length} 张仍在有效期`
    : "尚未导入证书";
}
watch(page, (value) => {
  if (value === "logs") void loadLogs();
  if (value === "config") void loadConfig();
});
watch(menuOpen, (value) => document.body.classList.toggle("menu-open", value));
function keydown(event: KeyboardEvent) {
  if (event.key === "Escape") {
    if (confirmBox.open) answerConfirm(false);
    else closeModal();
  }
}
onMounted(() => {
  window.localStorage.removeItem("nginx-web-theme");
  systemThemeQuery.addEventListener("change", systemThemeChanged);
  document.addEventListener("keydown", keydown);
  void loadCore();
});
onBeforeUnmount(() => {
  systemThemeQuery.removeEventListener("change", systemThemeChanged);
  document.removeEventListener("keydown", keydown);
  document.body.classList.remove("menu-open");
});
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <img class="brand-mark" :src="brandIcon" alt="" />
        <div><strong>nginx-web</strong><small>Nginx 管理中心</small></div>
      </div>
      <nav class="nav" aria-label="主导航">
        <button
          v-for="item in pages"
          :key="item.id"
          type="button"
          class="nav-item"
          :class="{ active: page === item.id }"
          @click="setPage(item.id)"
        >
          <SidebarIcon :name="item.icon" class="nav-icon" />
          ><span>{{ item.label }}</span>
        </button>
      </nav>
      <div class="sidebar-footer">
        <div class="mini-status">
          <span
            class="status-dot"
            :class="
              connectionError
                ? 'offline'
                : !overview
                  ? 'neutral'
                  : overview.nginx.running
                    ? overview.dirty
                      ? 'warning'
                      : 'online'
                    : 'offline'
            "
          ></span
          ><span>{{
            connectionError
              ? "连接管理服务失败"
              : !overview
                ? "正在连接…"
                : overview.nginx.running
                  ? overview.dirty
                    ? "运行中 · 有未应用变更"
                    : "Nginx 正常运行"
                  : "Nginx 已停止"
          }}</span>
        </div>
        <div class="version-row">
          <span>v{{ overview?.app_version ?? "0.1.7" }}</span
          ><span>Nginx {{ overview?.nginx_version ?? "1.30.4" }}</span>
        </div>
      </div>
    </aside>
    <div class="workspace">
      <div class="mobile-nav-bar">
        <button
          type="button"
          class="icon-button"
          aria-label="打开菜单"
          @click="menuOpen = !menuOpen"
        >
          <PhList :size="22" aria-hidden="true" />
        </button>
        <strong>{{ currentPage.label }}</strong>
      </div>
      <main class="content" aria-live="polite">
        <div v-if="loading" class="loading-panel">
          <div class="spinner"></div>
          <p>正在读取 nginx-web 状态…</p>
        </div>
        <article v-else-if="connectionError" class="card empty-state">
          <div class="empty-icon"><PhXCircle :size="24" aria-hidden="true" /></div>
          <h3>无法读取 nginx-web 状态</h3>
          <p>{{ connectionError }}</p>
          <button type="button" class="button primary" @click="loadCore()">
            重新连接
          </button>
        </article>
        <template v-else-if="overview && state">
          <template v-if="page === 'dashboard'">
            <section
              class="status-banner"
              :class="{ offline: !overview.nginx.running }"
            >
              <div class="status-orb">
                <PhCheckCircle v-if="overview.nginx.running" :size="30" weight="fill" aria-hidden="true" />
                <PhWarningCircle v-else :size="30" weight="fill" aria-hidden="true" />
              </div>
              <div class="status-copy">
                <div class="status-heading-row">
                  <h2>
                    {{
                      overview.nginx.running
                        ? "独立 Nginx 正常运行"
                        : "独立 Nginx 当前已停止"
                    }}
                  </h2>
                  <span
                    class="status-config-badge"
                    :class="{ pending: overview.dirty }"
                  >
                    {{ overview.dirty ? "配置待应用" : "配置已同步" }}
                  </span>
                </div>
                <p>
                  {{
                    overview.nginx.running
                      ? `服务运行稳定 · 反向代理工作正常 · 不使用飞牛系统 Nginx`
                      : "管理页面仍可使用，可以先校验配置再启动服务。"
                  }}
                </p>
                <span v-if="overview.nginx.running" class="status-pid">
                  PID {{ overview.nginx.pid }}
                </span>
              </div>
              <dl v-if="overview.nginx.running" class="status-facts">
                <div><dt>版本</dt><dd>{{ overview.nginx.version || overview.nginx_version }}</dd></div>
                <div><dt>监听端口</dt><dd>{{ ports }}</dd></div>
              </dl>
              <div class="status-banner-actions">
                <button
                  class="button ghost small"
                  :disabled="busy"
                  @click="loadCore()"
                >
                  <PhArrowClockwise :size="14" aria-hidden="true" />刷新状态
                </button>
                <template v-if="overview.nginx.running"
                  ><button
                    class="button secondary small"
                    @click="runNginxAction('reload')"
                  >
                    平滑重载</button
                  ><button class="button danger-ghost small" @click="stopNginx">
                    停止
                  </button></template
                ><button
                  v-else
                  class="button primary small"
                  @click="runNginxAction('start')"
                >
                  启动 Nginx
                </button>
              </div>
            </section>
            <section class="grid metric-grid section-gap">
              <article
                v-for="metric in dashboardMetrics"
                :key="metric.label"
                class="card metric-card"
                :class="`metric-${metric.tone}`"
              >
                <div class="metric-label">
                  <span>{{ metric.label }}</span
                  ><span class="metric-icon"><component :is="metric.icon" :size="23" weight="regular" aria-hidden="true" /></span>
                </div>
                <div class="metric-value">{{ metric.value }}</div>
                <div class="metric-foot">{{ metric.foot }}</div>
              </article>
            </section>
            <section
              v-if="overview.dirty"
              class="notice configuration-notice section-gap warning"
            >
              <div>
                <strong>配置等待应用</strong>
                <span>草稿与运行配置不一致</span>
              </div>
              <div class="configuration-actions">
                <button
                  class="button secondary small"
                  :disabled="busy"
                  @click="runNginxAction('test')"
                >
                  <PhCheckCircle :size="14" aria-hidden="true" />校验配置
                </button>
                <button
                  class="button primary small"
                  :disabled="busy"
                  @click="applyConfiguration"
                >
                  {{ busy ? "处理中…" : "保存并应用" }}
                </button>
              </div>
            </section>
            <section class="section-gap">
              <article class="card">
                <header class="card-header">
                  <div>
                    <h2>当前代理</h2>
                    <p>已启用规则会被写入独立 Nginx 配置</p>
                  </div>
                  <span class="spacer"></span
                  ><button class="button ghost small" @click="setPage('rules')">
                    查看全部
                  </button>
                </header>
                <div class="card-body">
                  <div v-if="activeRules.length" class="proxy-list-head" aria-hidden="true">
                    <span>名称 / 域名</span><span>协议</span><span>后端服务</span><span>状态</span>
                  </div>
                  <div
                    v-for="rule in activeRules.slice(0, 6)"
                    :key="rule.id"
                    class="detail-row proxy-row"
                  >
                    <div class="proxy-identity">
                      <strong>{{ rule.name }}</strong>
                      <div class="rule-sub">{{ rule.domains.join(", ") }}</div>
                    </div>
                    <div class="proxy-protocol">{{ rule.tls ? "HTTPS" : "HTTP" }} :{{ rule.listen_port }}</div>
                    <div class="proxy-upstream">{{ rule.upstream_scheme }}://{{ formatHost(rule.upstream_host) }}:{{ rule.upstream_port }}</div>
                    <div class="proxy-state"><span class="event-dot success"></span>正常<PhArrowRight :size="14" aria-hidden="true" /></div>
                  </div>
                  <div
                    v-if="!activeRules.length"
                    class="empty-state"
                    style="padding: 24px"
                  >
                    <h3>还没有启用的代理规则</h3>
                    <p>添加规则后，保存并应用即可开始转发。</p>
                  </div>
                </div>
              </article>
            </section>
            <section class="card recent-events section-gap">
              <header class="card-header">
                <div>
                  <h2>最近事件</h2>
                  <p>配置变更与代理状态记录</p>
                </div>
                <span class="spacer"></span>
                <button class="button ghost small" @click="setPage('logs')">查看日志</button>
              </header>
              <div v-if="recentEvents.length" class="event-list">
                <div v-for="event in recentEvents" :key="event.id" class="event-row">
                  <span class="event-dot" :class="event.tone"></span>
                  <div class="event-copy"><strong>{{ event.title }}</strong><span>{{ event.detail }}</span></div>
                  <time :datetime="event.time">{{ formatDate(event.time) }}</time>
                </div>
              </div>
              <div v-else class="event-empty">
                <PhInfo :size="18" aria-hidden="true" /> 首次保存或应用配置后，事件会显示在这里。
              </div>
            </section>
            <section
              v-if="overview.nginx.last_error"
              class="notice warning section-gap"
            >
              <strong>最近一条 Nginx 错误：</strong>
              {{ overview.nginx.last_error }}
            </section>
          </template>
          <template v-else-if="page === 'streams'">
            <StreamRulesPage
              :rules="state.stream_rules"
              :pools="state.upstream_pools"
              :certificates="state.certificates"
              :busy="busy"
              :dirty="state.dirty"
              @save="saveStreamRule"
              @remove="removeStreamRule"
              @toggle="toggleStreamRule"
              @refresh="loadCore()"
              @apply="applyConfiguration"
            />
          </template>
          <template v-else-if="page === 'upstreams'"
            ><UpstreamPoolsPage
              :pools="state.upstream_pools"
              :busy="busy"
              :dirty="state.dirty"
              @save="saveUpstreamPool"
              @remove="removeUpstreamPool"
              @refresh="loadCore()"
              @apply="applyConfiguration"
          /></template>
          <template v-else-if="page === 'rate-limits'"
            ><RateLimitPoliciesPage
              :policies="state.rate_limit_policies"
              :rules="state.rules"
              :busy="busy"
              :dirty="state.dirty"
              @save="saveRateLimitPolicy"
              @remove="removeRateLimitPolicy"
              @refresh="loadCore()"
              @apply="applyConfiguration"
          /></template>
          <template v-else-if="page === 'rules'"
            ><div class="toolbar">
              <input
                v-model="ruleSearch"
                class="input search-input"
                type="search"
                placeholder="搜索名称、域名或后端服务"
              /><span
                class="badge"
                :class="state.dirty ? 'warning' : 'success'"
                >{{ state.dirty ? "有未应用变更" : "配置已同步" }}</span
              ><span class="spacer"></span
              ><button class="button ghost" :disabled="busy" @click="loadCore()">
                <PhArrowClockwise :size="15" aria-hidden="true" />刷新
              </button>
              <button
                class="button"
                :class="state.dirty ? 'primary' : 'secondary'"
                :disabled="busy"
                @click="applyConfiguration"
              >
                {{ state.dirty ? "保存并应用" : "重新应用" }}
              </button>
              ><button class="button primary" @click="openRule()">
                ＋ 添加代理规则
              </button>
            </div>
            <article class="card">
              <div v-if="filteredRules.length" class="table-wrap">
                <table class="table">
                  <thead>
                    <tr>
                      <th>状态</th>
                      <th>名称与域名</th>
                      <th>入口</th>
                      <th>后端服务</th>
                      <th class="hide-mobile">能力</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="rule in filteredRules" :key="rule.id">
                      <td>
                        <label class="switch"
                          ><input
                            type="checkbox"
                            :checked="rule.enabled"
                            @change="
                              toggleRule(
                                rule,
                                ($event.target as HTMLInputElement).checked,
                              )
                            " /><span></span
                        ></label>
                      </td>
                      <td>
                        <div class="rule-name">{{ rule.name }}</div>
                        <div class="domain-list rule-sub">
                          <span
                            v-for="domain in rule.domains"
                            :key="domain"
                            class="domain-chip"
                            >{{ domain }}</span
                          >
                        </div>
                      </td>
                      <td>
                        <span
                          class="badge"
                          :class="rule.tls ? 'success' : 'info'"
                        >{{ rule.tls ? "HTTPS" : "HTTP" }}</span
                        >
                        <div class="rule-sub">
                          {{ formatRuleEntry(rule) }}
                          <template v-if="rule.domains.length > 1">
                            · 另 {{ rule.domains.length - 1 }} 个域名
                          </template>
                        </div>
                      </td>
                      <td>
                        <div>
                          {{ rule.upstream_scheme }}://{{
                            formatHost(rule.upstream_host)
                          }}:{{ rule.upstream_port }}
                        </div>
                        <div class="rule-sub">
                          连接 {{ rule.connect_timeout_seconds }}s · 读取
                          {{ rule.read_timeout_seconds }}s
                        </div>
                      </td>
                      <td class="hide-mobile">
                        <div class="domain-list">
                          <span v-if="rule.websocket" class="badge neutral"
                            >WebSocket</span
                          ><span v-if="rule.streaming" class="badge neutral"
                            >流式</span
                          ><span
                            v-if="rule.http2 && rule.tls"
                            class="badge neutral"
                            >HTTP/2</span
                          >
                        </div>
                      </td>
                      <td>
                        <div class="table-actions">
                          <button
                            class="button ghost small"
                            @click="openRule(rule)"
                          >
                            编辑</button
                          ><button
                            class="button danger-ghost small"
                            @click="deleteRule(rule)"
                          >
                            删除
                          </button>
                        </div>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-else class="empty-state">
                <div class="empty-icon">⇄</div>
                <h3>
                  {{ state.rules.length ? "没有匹配的规则" : "还没有代理规则" }}
                </h3>
                <p>
                  {{
                    state.rules.length
                      ? "换一个关键词再试。"
                      : "添加第一条规则，将域名或端口转发到 NAS、Docker 或局域网服务。"
                  }}
                </p>
                <button
                  v-if="!state.rules.length"
                  class="button primary"
                  @click="openRule()"
                >
                  添加代理规则
                </button>
              </div>
            </article>
            <div class="notice section-gap">
              第一版只允许监听 1024–65535 的非特权端口，默认 HTTP 端口为
              {{ state.settings.default_http_port }}。这可以避免使用 root
              权限，也不会抢占飞牛系统的 80/443。
            </div></template
          >
          <template v-else-if="page === 'certificates'"
            ><div class="toolbar">
              <div class="notice">
                nginx-web 只保存 PEM
                文件，不会把私钥返回到浏览器。证书目录权限为
                0700，私钥文件权限为 0600。
              </div>
              <span class="spacer"></span
              ><button class="button ghost" :disabled="busy" @click="loadCore()">
                <PhArrowClockwise :size="15" aria-hidden="true" />刷新
              </button>
              <button
                class="button"
                :class="state.dirty ? 'primary' : 'secondary'"
                :disabled="busy"
                @click="applyConfiguration"
              >
                {{ state.dirty ? "保存并应用" : "重新应用" }}
              </button>
              ><button class="button primary" @click="openCertificate">
                ＋ 导入证书
              </button>
            </div>
            <article class="card">
              <div v-if="state.certificates.length" class="table-wrap">
                <table class="table">
                  <thead>
                    <tr>
                      <th>名称</th>
                      <th>域名 / 主体</th>
                      <th>有效期</th>
                      <th>指纹</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="cert in state.certificates" :key="cert.id">
                      <td>
                        <div class="rule-name">{{ cert.name }}</div>
                        <div class="rule-sub">{{ cert.serial_number }}</div>
                      </td>
                      <td>
                        <div>
                          {{ (cert.dns_names ?? []).join(", ") || cert.subject || "—" }}
                        </div>
                        <div class="rule-sub">{{ cert.subject }}</div>
                      </td>
                      <td>
                        <span
                          class="badge"
                          :class="certificateState(cert.not_after).className"
                          >{{ certificateState(cert.not_after).label }}</span
                        >
                        <div class="rule-sub">
                          {{ formatDate(cert.not_before, true) }} ～
                          {{ formatDate(cert.not_after, true) }}
                        </div>
                      </td>
                      <td>
                        <code :title="cert.fingerprint">{{
                          shortFingerprint(cert.fingerprint)
                        }}</code>
                      </td>
                      <td>
                        <button
                          class="button danger-ghost small"
                          @click="deleteCertificate(cert.id, cert.name)"
                        >
                          删除
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-else class="empty-state">
                <div class="empty-icon">◇</div>
                <h3>尚未导入 HTTPS 证书</h3>
                <p>
                  第一版支持手动导入完整证书链和对应私钥。自动 ACME
                  申请将在后续版本增加。
                </p>
                <button class="button primary" @click="openCertificate">
                  导入证书
                </button>
              </div>
            </article></template
          >
          <template v-else-if="page === 'logs'"
            ><article class="card">
              <header class="card-header log-card-header">
                <div>
                  <h2>实时日志</h2>
                  <p>最多读取最近 2000 行，不会把整个大日志载入浏览器</p>
                </div>
                <span class="spacer"></span>
                <div class="log-toolbar">
                  <select v-model="logType" class="select" @change="loadLogs">
                    <option value="error">Nginx 错误日志</option>
                    <option value="access">Nginx 访问日志</option>
                    <option value="stream">TCP/UDP Stream 日志</option>
                    <option value="backend">nginx-web 管理日志</option></select
                  ><button class="button ghost small" @click="rotateLogs">立即轮转</button
                  ><button class="button ghost small" @click="loadLogs">
                    刷新
                  </button>
                </div>
              </header>
              <pre ref="logView" class="log-view">{{
                logsLoading
                  ? "正在读取…"
                  : logLines.length
                    ? logLines.join("\n")
                    : "暂无日志。"
              }}</pre>
            </article></template
          >
          <template v-else-if="page === 'revisions'"
            ><article class="card">
              <header class="card-header">
                <div>
                  <h2>已应用版本</h2>
                  <p>
                    每次“保存并应用”后自动保留，当前上限
                    {{ state.settings.revision_limit }} 个
                  </p>
                </div>
                <span class="spacer"></span>
                <button class="button ghost small" :disabled="busy" @click="loadCore()">
                  <PhArrowClockwise :size="14" aria-hidden="true" />刷新
                </button>
                <button
                  v-if="state.dirty"
                  class="button primary small"
                  :disabled="busy"
                  @click="applyConfiguration"
                >
                  保存并应用
                </button>
              </header>
              <div v-if="revisions.length" class="table-wrap">
                <table class="table">
                  <thead>
                    <tr>
                      <th>时间</th>
                      <th>说明</th>
                      <th>规则</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="revision in revisions" :key="revision.id">
                      <td>{{ formatDate(revision.created_at) }}</td>
                      <td>
                        <div class="rule-name">
                          {{ revision.summary || "配置快照" }}
                        </div>
                        <div class="rule-sub">{{ revision.id }}</div>
                      </td>
                      <td>
                        {{ revision.enabled_count }} /
                        {{ revision.rule_count }} 条启用
                      </td>
                      <td>
                        <div class="table-actions">
                          <button
                            class="button ghost small"
                            @click="revisionAction(revision, 'restore')"
                          >
                            恢复为草稿</button
                          ><button
                            class="button danger-ghost small"
                            @click="revisionAction(revision, 'delete')"
                          >
                            删除
                          </button>
                        </div>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-else class="empty-state">
                <div class="empty-icon">↶</div>
                <h3>还没有配置历史</h3>
                <p>首次保存并应用成功后，这里会出现可恢复的版本。</p>
              </div>
            </article>
            <div class="notice warning section-gap">
              恢复历史只会恢复规则和设置，不会立刻重载
              Nginx。检查草稿后，再在本页点击“保存并应用”。
            </div></template
          >
          <template v-else-if="page === 'config'"
            ><article class="card">
              <header class="card-header">
                <div>
                  <h2>生成的只读配置</h2>
                  <p>配置由结构化规则生成，避免直接写入危险指令</p>
                </div>
                <span class="spacer"></span
                ><button class="button ghost small" :disabled="busy" @click="loadConfig">
                  <PhArrowClockwise :size="14" aria-hidden="true" />刷新配置
                </button>
                <button class="button secondary small" :disabled="busy" @click="runNginxAction('test')">
                  <PhCheckCircle :size="14" aria-hidden="true" />校验配置
                </button>
                <button
                  class="button small"
                  :class="state.dirty ? 'primary' : 'secondary'"
                  :disabled="busy"
                  @click="applyConfiguration"
                >
                  {{ state.dirty ? "保存并应用" : "重新应用" }}
                </button>
                <button class="button ghost small" @click="copyConfig">
                  复制当前文件
                </button>
              </header>
              <div class="code-tabs">
                <button
                  v-for="key in configKeys"
                  :key="key"
                  class="code-tab"
                  :class="{ active: key === configTab }"
                  @click="configTab = key"
                >
                  {{ key === "master" ? "nginx.conf" : key }}
                </button>
              </div>
              <pre class="code-view">{{
                config ? configText : "正在读取…"
              }}</pre>
            </article></template
          >
          <template v-else>
            <RuntimeSettingsForm
              :settings="state.settings"
              :busy="busy"
              :dirty="state.dirty"
              @save="saveSettings"
              @clear-cache="clearCache"
              @test="runNginxAction('test')"
              @apply="applyConfiguration"
            />
          </template>
        </template>
      </main>
    </div>
  </div>
  <div
    v-if="modal"
    class="modal-backdrop"
    aria-hidden="false"
    @mousedown.self="closeModal"
  >
    <section
      class="modal"
      :class="{ 'rule-modal': modal === 'rule' }"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="`${modal}-title`"
    >
      <header class="modal-header">
        <PhShareNetwork
          v-if="modal === 'rule'"
          class="modal-title-icon"
          :size="34"
          weight="regular"
          aria-hidden="true"
        />
        <div>
          <h2 :id="`${modal}-title`">
            {{
              modal === "rule"
                ? editingRule
                  ? "编辑代理规则"
                  : "添加代理规则"
                : "导入 HTTPS 证书"
            }}
          </h2>
          <p>
            {{
              modal === "rule"
                ? "保存前会进行结构化校验；正式应用时还会执行 nginx -t。"
                : "证书链必须与私钥匹配，支持 RSA 和 ECDSA PEM。"
            }}
          </p>
        </div>
        <button class="icon-button" aria-label="关闭" @click="closeModal">
          ×
        </button>
      </header>
      <div class="modal-body">
        <RuleForm
          v-if="modal === 'rule'"
          :rule="editingRule"
          :settings="state!.settings"
          :certificates="state!.certificates"
          :upstream-pools="state!.upstream_pools"
          :rate-limit-policies="state!.rate_limit_policies"
          :busy="busy"
          @save="saveRule"
          @cancel="closeModal"
        /><CertificateForm
          v-else
          :busy="busy"
          @save="importCertificate"
          @cancel="closeModal"
        />
      </div>
    </section>
  </div>
  <div v-if="confirmBox.open" class="modal-backdrop" aria-hidden="false">
    <section
      class="modal confirm-modal"
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="confirm-title"
    >
      <header class="modal-header">
        <div>
          <h2 id="confirm-title">{{ confirmBox.title }}</h2>
          <p>{{ confirmBox.message }}</p>
        </div>
      </header>
      <footer class="modal-footer">
        <button class="button ghost" @click="answerConfirm(false)">取消</button
        ><button class="button danger" @click="answerConfirm(true)">
          确认
        </button>
      </footer>
    </section>
  </div>
  <div class="toast-region" aria-live="assertive">
    <div v-for="item in toasts" :key="item.id" class="toast" :class="item.type">
      <b>{{
        item.type === "success" ? "✓" : item.type === "error" ? "!" : "i"
      }}</b
      ><span>{{ item.message }}</span>
    </div>
  </div>
</template>
