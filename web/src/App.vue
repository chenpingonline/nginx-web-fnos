<script setup lang="ts">
import type { ACMEInput } from "./types";
import ACMEJobs from "./components/ACMEJobs.vue";
import AppSelect from "./components/AppSelect.vue";
import RevisionPreview from "./components/RevisionPreview.vue";
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
  PhArrowsLeftRight,
  PhCertificate,
  PhCheckCircle,
  PhList,
  PhPlusCircle,
  PhShareNetwork,
  PhX,
  PhXCircle,
} from "@phosphor-icons/vue";
import { errorMessage, jsonBody, request } from "./api";
import { followSystemTheme } from "./theme";
import { followScrollActivity } from "./scrollbars";
import { highlightLog, searchLogLines } from "./logHighlight";
const appVersion = __APP_VERSION__;
import RuleGroups from "./components/RuleGroups.vue";
import type { RuleGroup } from "./types";
import RuleForm from "./components/RuleForm.vue";
import DashboardPage from "./components/DashboardPage.vue";
import ErrorDetailsPage from "./components/ErrorDetailsPage.vue";
import CertificateForm from "./components/CertificateForm.vue";
import UpstreamPoolsPage from "./components/UpstreamPoolsPage.vue";
import RateLimitPoliciesPage from "./components/RateLimitPoliciesPage.vue";
import BackupPage, { type BackupFile } from "./components/BackupPage.vue";
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
  | "requests"
  | "backup"
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
    label: "代理 HTTP(S)",
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
    label: "后端服务组",
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
    label: "SSL/TLS 证书",
    subtitle: "管理证书导入、ACME 申请与自动续期",
  },
  { id: "errors", icon: "requests", label: "请求详情", subtitle: "查看 HTTP 请求统计与错误趋势" },
  {
    id: "logs",
    icon: "logs",
    label: "运行日志",
    subtitle: "查看 Nginx 与管理服务日志",
  },
  { id: "backup", icon: "backup", label: "备份与恢复", subtitle: "导出配置备份并恢复为草稿" },
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
const restoredRevision = computed(() => state.value?.dirty && state.value.draft_revision_id
  ? revisions.value.find(item => item.id === state.value!.draft_revision_id) : undefined);
const showDraftPreview = ref(false);
const draftPreview = computed<Revision | null>(() => state.value ? {
  id: "当前草稿",
  summary: state.value.dirty ? "待应用，以当前编辑内容为准" : "已同步",
  created_at: state.value.updated_at,
  rule_count: (state.value.rules?.length ?? 0) + (state.value.stream_rules?.length ?? 0),
  enabled_count: 0,
  state: state.value,
} : null);
const errorScope = ref({ minutes: 60, rule: "" });
const ruleProtocol = ref<"all" | "http" | "https">("all");
const ruleEnabled = ref<"all" | "enabled" | "disabled">("all");
const loading = ref(true),
  busy = ref(false),
  connectionError = ref(""),
  menuOpen = ref(false),
  ruleSearch = ref(""),
  logType = ref<LogType>("access"),
  logLines = ref<string[]>([]),
  logsLoading = ref(false),
  configTab = ref("master");
const previewRevisionID = ref<string | null>(null);
const activePreview = computed(() => showDraftPreview.value ? draftPreview.value : revisions.value.find(item => item.id === previewRevisionID.value) ?? null);
function closePreview() {
  showDraftPreview.value = false;
  previewRevisionID.value = null;
}
const logLineLimit = ref(500);
const logSearch = ref("");
const activeLogMatch = ref(0);
const highlightedLogLines = computed(() => logLines.value.map(highlightLog));
const searchedLogs = computed(() => searchLogLines(highlightedLogLines.value, logSearch.value));
async function revealLogMatch() {
  await nextTick();
  const view = logView.value;
  const target = view?.querySelector<HTMLElement>(`[data-log-match="${activeLogMatch.value}"]`);
  if (view && target) {
    view.scrollTop += target.getBoundingClientRect().top - view.getBoundingClientRect().top - view.clientHeight / 2;
  }
}
function jumpLogMatch(direction: number) {
  if (!searchedLogs.value.count) return;
  activeLogMatch.value = (activeLogMatch.value + direction + searchedLogs.value.count) % searchedLogs.value.count;
  void revealLogMatch();
}
watch(logSearch, (query) => {
  activeLogMatch.value = 0;
  if (query) {
    stopLogs();
    void revealLogMatch();
  } else if (page.value === "logs") void loadLogs();
});
const stopTheme = followSystemTheme();
const stopScrollActivity = followScrollActivity();
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
const selectedGroup = ref("all");
const groupPanel = ref<InstanceType<typeof RuleGroups>>();
const ruleGroups = computed(() => state.value?.rule_groups ?? []);
async function saveRuleGroup(value: RuleGroup): Promise<{ group: RuleGroup; error?: string }> {
  if (busy.value) throw new Error("请等待当前操作完成");
  busy.value = true;
  let saved = false;
  try {
    const group = await request<RuleGroup>(value.id ? `/rule-groups/${value.id}` : "/rule-groups", {
      method: value.id ? "PUT" : "POST", body: jsonBody(value),
    });
    saved = true;
    try {
      await request("/apply", { method: "POST", body: jsonBody({ summary: `保存分组：${group.name}` }) });
      toast("分组已保存并应用", "success");
      return { group };
    } catch (e) { return { group, error: `分组已保存，但应用失败：${errorMessage(e)}。修正后可再次保存并应用。` }; }
  } finally {
    if (saved) await loadCore(true);
    busy.value = false;
  }
}
async function deleteRuleGroup(id: string): Promise<boolean> {
  if (busy.value) return false;
  if (!(await ask("删除分组", "组内规则将移至未分组并保留当前访问配置。删除后会应用所有尚未应用的修改，是否继续？"))) return false;
  busy.value = true;
  let deleted = false;
  try {
    await request(`/rule-groups/${id}`, { method: "DELETE" });
    deleted = true;
    try {
      await request("/apply", { method: "POST", body: jsonBody({ summary: "删除代理分组" }) });
      toast("分组已删除，规则配置已保留", "success");
    } catch (e) { toast(`分组已删除，但应用失败：${errorMessage(e)}，请到“Nginx 配置”页面重试保存并应用。`, "error"); }
    return true;
  } finally {
    if (deleted) await loadCore(true);
    busy.value = false;
  }
}
const filteredRules = computed(() => {
  const rules = state.value?.rules ?? [],
    term = ruleSearch.value.trim().toLowerCase();
  const poolNames = new Map((state.value?.upstream_pools ?? []).map(pool => [pool.id, pool.name]));
  return rules.filter(
    (rule) =>
      (selectedGroup.value === "all" || (rule.group_id || "") === selectedGroup.value) &&
      (ruleProtocol.value === "all" || (rule.tls ? "https" : "http") === ruleProtocol.value) &&
      (ruleEnabled.value === "all" || rule.enabled === (ruleEnabled.value === "enabled")) &&
      (!term ||
      [
        rule.name,
        ...(rule.domains ?? []).map(domain => `${domain}:${rule.listen_port}`),
        String(rule.listen_port),
        `${rule.upstream_host}:${rule.upstream_port}`,
        poolNames.get(rule.upstream_pool_id),
      ]
        .join(" ")
        .toLowerCase()
        .includes(term)),
  );
});
const hasRuleFilters = computed(() => Boolean(selectedGroup.value !== "all" || ruleSearch.value || ruleProtocol.value !== "all" || ruleEnabled.value !== "all"));
function resetRuleFilters() {
  selectedGroup.value = "all";
  ruleSearch.value = "";
  ruleProtocol.value = "all";
  ruleEnabled.value = "all";
}
function formatRuleEntry(rule: ProxyRule, domain = rule.domains[0]): string {
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


function toast(message: string, type: Toast["type"] = "") {
  const id = ++toastID.value;
  toasts.value.push({ id, message, type });
  window.setTimeout(
    () => (toasts.value = toasts.value.filter((item) => item.id !== id)),
    5200,
  );
}
function setPage(value: Page) {
  if (value === "errors") errorScope.value = { ...errorScope.value, rule: "" };
  page.value = value;
  menuOpen.value = false;
}
function openErrorDetails(scope: { minutes: number; rule: string }) {
  setPage("errors");
  errorScope.value = scope;
  document.querySelector(".workspace")?.scrollTo({ top: 0 });
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
let logTimer: ReturnType<typeof setTimeout> | undefined;
let logController: AbortController | undefined;
let logGeneration = 0;
function stopLogs() {
  clearTimeout(logTimer);
  logController?.abort();
  logGeneration++;
  logsLoading.value = false;
}
async function loadLogs() {
  clearTimeout(logTimer);
  logController?.abort();
  const generation = ++logGeneration;
  logController = new AbortController();
  logsLoading.value = true;
  try {
    const result = await request<LogResponse>(
      `/logs?type=${encodeURIComponent(logType.value)}&lines=${logLineLimit.value}`,
      { signal: logController.signal },
    );
    if (generation !== logGeneration) return;
    logLines.value = result.lines ?? [];
  } catch (error) {
    if (generation !== logGeneration) return;
    logLines.value = [`读取失败：${errorMessage(error)}`];
  } finally {
    if (generation === logGeneration) {
      logsLoading.value = false;
      await nextTick();
      if (logSearch.value) {
        activeLogMatch.value = Math.min(activeLogMatch.value, Math.max(0, searchedLogs.value.count - 1));
        void revealLogMatch();
      } else if (logView.value) logView.value.scrollTop = logView.value.scrollHeight;
      if (page.value === "logs" && !logSearch.value) logTimer = setTimeout(() => void loadLogs(), 3000);
    }
  }
}
function changeLogScope() {
  activeLogMatch.value = 0;
  logLines.value = [];
  void loadLogs();
}
async function openDashboardRule(id: string) {
  await loadCore(true);
  if (connectionError.value) return;
  const rule = state.value?.rules.find(item => item.id === id);
  if (rule) openRule(rule);
  else toast("该规则已不存在，请刷新列表", "error");
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
  let persisted = false;
  const saved = await mutate(async () => {
    const rule = await request<ProxyRule>(id ? `/rules/${id}` : "/rules", {
      method: id ? "PUT" : "POST",
      body: jsonBody(value),
    });
    persisted = true;
    editingRule.value = rule;
    if (applyAfter) {
      try {
        await request("/apply", {
          method: "POST",
          body: jsonBody({ summary: `${id ? "修改" : "新增"}规则：${value.name}` }),
        });
      } catch (e) { throw new Error(`规则已保存，但应用失败：${errorMessage(e)}。修正后可再次保存并应用。`); }
    }
    return true;
  });
  if (saved) {
    modal.value = null;
    toast(applyAfter ? "规则已保存并应用" : "规则已保存为草稿", "success");
  }
  if (persisted) await loadCore(true);
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
async function createACME(value: ACMEInput) {
  const saved = await mutate(() => request('/acme', { method: 'POST', body: jsonBody(value) }));
  if (saved !== undefined) { modal.value = null; toast('ACME 任务已创建，将在后台申请证书', 'success'); }
}
async function acmeAction(id: string, action: string) {
  if (action === 'delete' && !await ask('移除 ACME 任务', '将删除申请配置和凭据并停止自动续期，已导入的证书会保留。')) return;
  const result = await mutate(() => request(`/acme/${id}${action === 'delete' ? '' : '/' + action}`, { method: action === 'delete' ? 'DELETE' : 'POST' }));
  if (result !== undefined) toast('ACME 任务已更新', 'success');
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
      "删除 SSL/TLS 证书",
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
        ? "历史版本的规则和设置将覆盖当前草稿（包括尚未应用的修改），不会立刻影响当前代理。"
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
  if (ok !== undefined) {
    await loadCore(true);
    if (restoring) {
      closePreview();
      await nextTick();
      document.querySelector(".workspace")?.scrollTo({ top: 0 });
    }
  }
}
async function saveSettings(value: Settings) {
  const ok = await mutate(
    () => request("/settings", { method: "PUT", body: jsonBody(value) }),
    "设置已保存为草稿",
  );
  if (ok !== undefined) await loadCore(true);
}
async function testSettings(value: Settings) {
  const result = await mutate(() => request<ApplyResult>("/settings/test", {
    method: "POST", body: jsonBody(value),
  }));
  if (result) toast(result.message, "success");
}
async function applySettings(value: Settings) {
  if (busy.value) return;
  busy.value = true;
  let saved = false;
  try {
    await request("/settings", { method: "PUT", body: jsonBody(value) });
    saved = true;
    const result = await request<ApplyResult>("/apply", {
      method: "POST", body: jsonBody({ summary: "保存并应用全局设置" }),
    });
    toast(result.message || "设置已保存并应用", "success");
  } catch (error) {
    toast((saved ? "设置已保存为草稿，但应用失败：" : "") + errorMessage(error), "error");
  } finally {
    if (saved) await loadCore(true);
    busy.value = false;
  }
}
async function downloadBackup() {
  const backup = await mutate(() => request<BackupFile>("/backup"));
  if (!backup) return;
  const url = URL.createObjectURL(new Blob([JSON.stringify(backup, null, 2)], { type: "application/json" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = `nginx-web-backup-${new Date().toISOString().replace(/[:.]/g, "-")}.json`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
async function restoreBackup(backup: BackupFile) {
  if (!await ask("从备份恢复", "将替换已保存的全局设置和代理配置，并保留恢复前的配置历史。恢复后仅保存为草稿，不会立即应用。")) return;
  const result = await mutate(() => request<State>("/backup/restore", {
    method: "POST", body: jsonBody(backup),
  }), "备份已恢复为草稿，请确认后应用");
  if (result) await loadCore(true);
}
async function clearCache() {
  if (!(await ask("清理代理缓存", "将删除 nginx-web 生成的全部 HTTP 缓存文件，正在处理的请求可能重新回源。"))) return;
  await mutate(() => request("/cache", { method: "DELETE" }), "代理缓存已清理");
}
async function saveAndApplyResource<T>(path: string, value: unknown, id: string, label: string, done: (saved?: T, error?: string) => void) {
  if (busy.value) return;
  busy.value = true;
  let saved: T | undefined;
  try {
    saved = await request<T>(id ? `${path}/${id}` : path, {
      method: id ? "PUT" : "POST", body: jsonBody(value),
    });
    await request("/apply", { method: "POST", body: jsonBody({ summary: `保存并应用${label}` }) });
    done(saved);
    toast(`${label}已保存并应用`, "success");
  } catch (error) {
    done(saved, saved ? `${label}已保存，但应用失败：${errorMessage(error)}。修正后可再次保存并应用。` : errorMessage(error));
  } finally {
    if (saved) await loadCore(true);
    busy.value = false;
  }
}
async function saveUpstreamPool(value: UpstreamPoolInput, id: string, done: (saved?: UpstreamPool, error?: string) => void) {
  await saveAndApplyResource("/upstreams", value, id, "后端服务组", done);
}
async function removeUpstreamPool(pool: UpstreamPool) {
  if (
    !(await ask(
      "删除后端服务组",
      `确定删除“${pool.name}”吗？正在使用的后端服务组不能删除。`,
    ))
  )
    return;
  const ok = await mutate(
    () => request(`/upstreams/${pool.id}`, { method: "DELETE" }),
    "后端服务组已删除",
  );
  if (ok !== undefined) await loadCore(true);
}
async function saveRateLimitPolicy(value: RateLimitPolicyInput, id: string, done: (saved?: RateLimitPolicy, error?: string) => void) {
  await saveAndApplyResource("/rate-limit-policies", value, id, "限流策略", done);
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
  let saved = false;
  await mutate(
    async () => {
      await request(id ? `/streams/${id}` : "/streams", {
        method: id ? "PUT" : "POST",
        body: jsonBody(value),
      });
      saved = true;
      try {
        return await request<ApplyResult>("/apply", {
          method: "POST",
          body: jsonBody({ summary: "保存并应用 TCP/UDP 规则" }),
        });
      } catch (error) {
        throw new Error(`规则已保存，但应用失败：${errorMessage(error)}。请修正后重新保存，或到“Nginx 配置”页面重新应用。`);
      }
    },
    "TCP/UDP 规则已保存并应用",
  );
  if (saved) await loadCore(true);
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
watch(page, (value) => {
  document.querySelector(".workspace")?.scrollTo({ top: 0 });
  if (value === "logs") void loadLogs();
  else stopLogs();
  if (value === "config") void loadConfig();
});
watch(menuOpen, (value) => document.body.classList.toggle("menu-open", value));
function keydown(event: KeyboardEvent) {
  if (event.key === "Escape") {
    if (activePreview.value) return;
    if (confirmBox.open) answerConfirm(false);
    else closeModal();
  }
}
onMounted(() => {
  document.addEventListener("keydown", keydown);
  void loadCore();
});
onBeforeUnmount(() => {
  stopTheme();
  stopScrollActivity();
  stopLogs();
  document.removeEventListener("keydown", keydown);
  document.body.classList.remove("menu-open");
});
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
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
          <span>{{ item.label }}</span>
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
          <span>v{{ overview?.app_version ?? appVersion }}</span
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
          <DashboardPage v-if="page === 'dashboard'"
            :overview="overview" :busy="busy" :updated-at="state.updated_at"
            :initial-minutes="errorScope.minutes" initial-rule=""
            @overview="overview = $event" @add="openRule()" @apply="applyConfiguration"
            @test="runNginxAction('test')" @reload="runNginxAction('reload')" @start="runNginxAction('start')"
            @stop="stopNginx"
            @errors="openErrorDetails"
            @navigate="setPage"
          />
          <ErrorDetailsPage v-else-if="page === 'errors'"
            :overview="overview" :busy="busy" :updated-at="state.updated_at"
            :initial-minutes="errorScope.minutes" :initial-rule="errorScope.rule"
            @overview="overview = $event" @edit="openDashboardRule"
          />
          <BackupPage v-else-if="page === 'backup'" :state="state" :busy="busy" @download="downloadBackup" @restore="restoreBackup" @apply="applyConfiguration" />
          <template v-else-if="page === 'streams'">
            <StreamRulesPage
              :rules="state.stream_rules"
              :pools="state.upstream_pools"
              :certificates="state.certificates"
              :busy="busy"
              @save="saveStreamRule"
              @remove="removeStreamRule"
              @toggle="toggleStreamRule"
              @refresh="loadCore()"
            />
          </template>
          <template v-else-if="page === 'upstreams'"
            ><UpstreamPoolsPage
              :pools="state.upstream_pools"
              :busy="busy"
              @save="saveUpstreamPool"
              @remove="removeUpstreamPool"
              @refresh="loadCore()"
          /></template>
          <template v-else-if="page === 'rate-limits'"
            ><RateLimitPoliciesPage
              :policies="state.rate_limit_policies"
              :rules="state.rules"
              :busy="busy"
              @save="saveRateLimitPolicy"
              @remove="removeRateLimitPolicy"
              @refresh="loadCore()"
          /></template>
          <template v-else-if="page === 'rules'"
            ><div class="toolbar rule-filters" role="search" aria-label="HTTP/HTTPS 规则筛选">
              <input
                v-model="ruleSearch"
                class="input search-input"
                type="search"
                aria-label="搜索 HTTP/HTTPS 规则"
                placeholder="搜索名称、域名、端口或目标"
              />
              <AppSelect v-model="ruleProtocol" class="select" aria-label="HTTP/HTTPS 协议筛选">
                <option value="all">全部协议</option><option value="http">HTTP</option><option value="https">HTTPS</option>
              </AppSelect>
              <AppSelect v-model="ruleEnabled" class="select" aria-label="HTTP/HTTPS 启用状态筛选">
                <option value="all">全部状态</option><option value="enabled">已启用</option><option value="disabled">已停用</option>
              </AppSelect>
              <button class="button ghost" :disabled="!hasRuleFilters" @click="resetRuleFilters">重置筛选</button>
              <span class="filter-count">{{ filteredRules.length }} / {{ state.rules.length }} 条</span>
            </div>
            <RuleGroups ref="groupPanel" :groups="ruleGroups" :rules="state.rules" :certificates="state.certificates"
              :selected="selectedGroup" :busy="busy" :default-port="state.settings.default_http_port"
              :save="saveRuleGroup" :remove="deleteRuleGroup" @select="selectedGroup = $event" />
            <div class="toolbar"><span
                class="badge"
                :class="state.dirty ? 'warning' : 'success'"
                >{{ state.dirty ? "有未应用变更" : "配置已同步" }}</span
              ><span class="spacer"></span
              ><button class="button ghost" :disabled="busy" @click="loadCore()">
                <PhArrowClockwise :size="15" aria-hidden="true" />刷新
              </button>
              <button class="button ghost" :disabled="busy" @click="groupPanel?.show()"><PhPlusCircle :size="17" aria-hidden="true" />新建分组</button>
              <button class="button primary" @click="openRule()">
                <PhPlusCircle :size="17" aria-hidden="true" />添加代理规则
              </button>
            </div>
            <article class="card proxy-rule-card">
              <div v-if="filteredRules.length" class="table-wrap">
                <table class="table">
                  <thead>
                    <tr>
                      <th>状态</th>
                      <th>名称</th>
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
                      </td>
                      <td>
                        <div v-for="domain in rule.domains" :key="domain">
                          {{ formatRuleEntry(rule, domain) }}
                        </div>
                        <div v-if="!rule.domains.length">
                          {{ formatRuleEntry(rule) }}
                        </div>
                        <div class="rule-sub">
                          <span
                            class="badge"
                            :class="rule.tls ? 'success' : 'info'"
                          >{{ rule.tls ? "HTTPS" : "HTTP" }}</span
                          >
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
                  {{ selectedGroup !== 'all' ? "此分组暂无匹配的规则" : state.rules.length ? "没有匹配的规则" : "还没有代理规则" }}
                </h3>
                <p>
                  {{
                    state.rules.length
                      ? "调整关键词、协议或启用状态后重试。"
                      : "添加第一条规则，将域名或端口转发到 NAS、Docker 或局域网服务。"
                  }}
                </p>
              </div>
            </article>
            <div class="notice section-gap">
              只允许监听 1024–65535 的非特权端口，默认 HTTP 端口为
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
              <button class="button primary" @click="openCertificate">
                <PhPlusCircle :size="17" aria-hidden="true" />导入证书
              </button>
            </div>
            <article class="card">
              <ACMEJobs @changed="loadCore(true)" @action="acmeAction" />
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
                <h3>尚未导入 SSL/TLS 证书</h3>
                <p>
                  支持 ACME 自动申请、文件上传、服务器路径和粘贴 PEM。
                </p>
              </div>
            </article></template
          >
          <template v-else-if="page === 'logs'"
            ><article class="card">
              <header class="card-header log-card-header">
                <div>
                  <h2>实时日志</h2>
                  <p>显示最近 {{ logLineLimit }} 行 · {{ logSearch ? "搜索中，自动刷新已暂停" : "每 3 秒刷新 · 自动滚动至最新日志" }}</p>
                </div>
                <span class="spacer"></span>
                <div class="log-toolbar">
                  <AppSelect v-model="logType" hide-check class="select" aria-label="日志类型" @change="changeLogScope">
                    <option value="access">HTTP/HTTPS 访问日志</option>
                    <option value="stream">TCP/UDP Stream 日志</option>
                    <option value="backend">nginx-web 管理日志</option>
                    <option value="error">Nginx 错误日志</option></AppSelect
                  ><button class="button ghost small" @click="loadLogs">
                    刷新
                  </button>
                </div>
              </header>
              <div class="log-search-toolbar" role="search" aria-label="搜索日志">
                <AppSelect v-model="logLineLimit" hide-check class="select log-line-limit" aria-label="显示日志行数" @change="changeLogScope">
                  <option v-for="limit in [100, 200, 500, 1000, 2000]" :key="limit" :value="limit">最近 {{ limit }} 行</option>
                </AppSelect>
                <input v-model="logSearch" class="input" type="search" :placeholder="`搜索最近 ${logLineLimit} 行日志…`" aria-label="搜索日志内容" @keydown.enter.prevent="jumpLogMatch($event.shiftKey ? -1 : 1)" @keydown.esc.prevent="logSearch = ''" />
                <span class="log-search-count" aria-live="polite">{{ logSearch ? searchedLogs.count ? `${activeLogMatch + 1} / ${searchedLogs.count}` : '无匹配' : '' }}</span>
                <button class="button ghost small" :disabled="!searchedLogs.count" aria-label="上一个匹配" title="上一个匹配（Shift+Enter）" @click="jumpLogMatch(-1)">↑</button>
                <button class="button ghost small" :disabled="!searchedLogs.count" aria-label="下一个匹配" title="下一个匹配（Enter）" @click="jumpLogMatch(1)">↓</button>
                <button v-if="logSearch" class="button ghost small" @click="logSearch = ''">清除</button>
              </div>
              <pre ref="logView" class="log-view"><template v-if="logLines.length"><template v-for="(line, lineIndex) in searchedLogs.lines" :key="lineIndex"><span v-for="(token, tokenIndex) in line" :key="tokenIndex" :data-log-match="token.matchIndex" :class="[token.tone ? `log-token-${token.tone}` : undefined, { 'log-search-hit': token.matchIndex !== undefined, 'log-search-current': token.matchIndex !== undefined && token.matchIndex === activeLogMatch }]">{{ token.text }}</span>{{ lineIndex < highlightedLogLines.length - 1 ? '\n' : '' }}</template></template><template v-else>{{ logsLoading ? "正在读取…" : "暂无日志。" }}</template></pre>
            </article></template
          >
          <template v-else-if="page === 'revisions'"
            >
            <article class="card" aria-label="当前草稿状态">
              <header class="card-header">
                <div aria-live="polite">
                  <h2>当前草稿 <span class="badge" :class="state.dirty ? 'warning' : 'success'">{{ state.dirty ? '待应用' : '已同步' }}</span></h2>
                  <template v-if="state.dirty && state.draft_revision_id">
                    <p class="draft-restored-title">已恢复：{{ restoredRevision ? `${formatDate(restoredRevision.created_at)} · ${restoredRevision.summary || '配置快照'}` : state.draft_revision_id }}</p>
                    <p>此版本已写入当前草稿，尚未应用到 Nginx。后续编辑会保留在草稿中。</p>
                  </template>
                  <p v-else>{{ state.dirty ? '当前有尚未应用的修改，运行配置尚未切换。' : '当前配置已同步，没有待应用的修改。' }}</p>
                </div>
                <span class="spacer"></span>
                <button class="button ghost small" aria-haspopup="dialog" @click="showDraftPreview = true">查看当前草稿</button>
                <button class="button ghost small" @click="page = 'rules'">编辑代理规则</button>
                <button
                  v-if="state.dirty"
                  class="button primary small"
                  :disabled="busy"
                  title="将当前草稿应用到 Nginx，并生成新的历史版本"
                  @click="applyConfiguration"
                >
                  <PhCheckCircle :size="14" aria-hidden="true" />保存并应用
                </button>
              </header>
              <div v-if="state.last_apply_error" class="notice warning" role="alert">上次应用失败：{{ state.last_apply_error }}</div>
            </article>
            <article class="card section-gap">
              <header class="card-header">
                <div>
                  <h2>已应用版本</h2>
                  <p>
                    每次应用成功后自动保留，可预览完整历史配置，当前上限
                    {{ state.settings.revision_limit }} 个
                  </p>
                </div>
                <span class="spacer"></span>
                <button class="button ghost small" :disabled="busy" @click="loadCore()">
                  <PhArrowClockwise :size="14" aria-hidden="true" />刷新
                </button>

              </header>
              <div v-if="revisions.length" class="table-wrap">
                <table class="table">
                  <thead>
                    <tr>
                      <th>时间</th>
                      <th>说明</th>
                      <th>规则</th>
                      <th>草稿状态</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <template v-for="revision in revisions" :key="revision.id">
                    <tr :class="{ 'revision-source-row': state.dirty && state.draft_revision_id === revision.id }">
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
                        <span v-if="state.dirty && state.draft_revision_id === revision.id" class="badge warning">当前草稿来源 · 待应用</span>
                        <span v-else class="rule-sub">—</span>
                      </td>
                      <td>
                        <div class="table-actions">
                          <button class="button ghost small"
                            aria-haspopup="dialog"
                            @click="showDraftPreview = false; previewRevisionID = revision.id">
                            预览配置
                          </button>
                          <button
                            class="button ghost small"
                            :disabled="busy"
                            @click="revisionAction(revision, 'restore')"
                          >
                            恢复为草稿</button
                          ><button
                            class="button danger-ghost small"
                            :disabled="busy"
                            @click="revisionAction(revision, 'delete')"
                          >
                            删除
                          </button>
                        </div>
                      </td>
                    </tr>
                    </template>
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
              预览不会修改配置。恢复为草稿会覆盖当前草稿中的规则和设置，但不会立即影响运行中的 Nginx；检查草稿后，点击“保存并应用”才会生效。顶部按钮应用的是当前全部草稿，并非正在预览的历史版本。
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
                  v-if="state.dirty"
                  class="button primary small"
                  :disabled="busy"
                  @click="applyConfiguration"
                >
                  <PhCheckCircle :size="14" aria-hidden="true" />保存并应用
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
              @test="testSettings"
              @apply="applySettings"
            />
          </template>
        </template>
      </main>
    </div>
  </div>
  <RevisionPreview v-if="activePreview" :revision="activePreview" :title="showDraftPreview ? '当前草稿预览' : '历史配置预览'" @close="closePreview" />
  <div
    v-if="modal"
    class="modal-backdrop"
    aria-hidden="false"
    @mousedown.self="closeModal"
  >
    <section
      class="modal"
      :class="{ 'rule-modal': modal === 'rule', 'certificate-modal': modal === 'certificate' }"
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
                : "导入 SSL/TLS 证书"
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
        <div v-if="modal === 'rule'" class="rule-header-actions">
          <button type="button" class="button ghost" :disabled="busy" @click="closeModal">取消</button>
          <button type="submit" form="proxy-rule-form" class="button primary" :disabled="busy">
            {{ busy ? "处理中…" : editingRule ? "保存修改" : "创建规则" }}
          </button>
        </div>
        <button type="button" class="icon-button modal-close" aria-label="关闭" @click="closeModal">
          <PhX :size="20" aria-hidden="true" />
        </button>
      </header>
      <div class="modal-body">
        <RuleForm
          v-if="modal === 'rule'"
          :rule="editingRule"
          :groups="ruleGroups"
          :initial-group="selectedGroup === 'all' ? '' : selectedGroup"
          :settings="state!.settings"
          :certificates="state!.certificates"
          :upstream-pools="state!.upstream_pools"
          :rate-limit-policies="state!.rate_limit_policies"
          :busy="busy"
          @save="saveRule"
        /><CertificateForm
          @acme="createACME"
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

<style scoped>
.draft-restored-title { color: var(--text); font-weight: 600; }
.table tr.revision-source-row > td { background: var(--warning-soft); }
.table tr.revision-source-row > td:first-child { box-shadow: inset 3px 0 var(--warning); }
</style>
