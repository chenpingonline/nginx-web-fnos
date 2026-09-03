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
import { errorMessage, jsonBody, request } from "./api";
import RuleForm from "./components/RuleForm.vue";
import CertificateForm from "./components/CertificateForm.vue";
import UpstreamPoolsPage from "./components/UpstreamPoolsPage.vue";
import RuntimeSettingsForm from "./components/RuntimeSettingsForm.vue";
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
  Revision,
  Settings,
  State,
  Toast,
  UpstreamPool,
  UpstreamPoolInput,
} from "./types";
import { formatDate, formatHost, shortFingerprint } from "./utils";

const pages: { id: Page; icon: string; label: string; subtitle: string }[] = [
  {
    id: "dashboard",
    icon: "⌂",
    label: "总览",
    subtitle: "查看代理服务与配置状态",
  },
  {
    id: "rules",
    icon: "⇄",
    label: "代理规则",
    subtitle: "管理域名、监听端口与上游服务",
  },
  {
    id: "upstreams",
    icon: "⇶",
    label: "上游服务器池",
    subtitle: "管理负载均衡、节点健康参数与连接复用",
  },
  {
    id: "certificates",
    icon: "◇",
    label: "HTTPS 证书",
    subtitle: "导入并管理手动 TLS 证书",
  },
  {
    id: "logs",
    icon: "≡",
    label: "运行日志",
    subtitle: "查看 Nginx 与管理服务日志",
  },
  {
    id: "revisions",
    icon: "↶",
    label: "配置历史",
    subtitle: "恢复或清理已应用的配置版本",
  },
  {
    id: "config",
    icon: "{ }",
    label: "Nginx 配置",
    subtitle: "查看 nginx-web 实际生成的配置文件",
  },
  {
    id: "settings",
    icon: "⚙",
    label: "设置",
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
async function saveUpstreamPool(value: UpstreamPoolInput, id: string) {
  const ok = await mutate(
    () =>
      request(id ? `/upstreams/${id}` : "/upstreams", {
        method: id ? "PUT" : "POST",
        body: jsonBody(value),
      }),
    id ? "上游池已更新" : "上游池已创建",
  );
  if (ok !== undefined) await loadCore(true);
}
async function removeUpstreamPool(pool: UpstreamPool) {
  if (
    !(await ask(
      "删除上游服务器池",
      `确定删除“${pool.name}”吗？正在使用的上游池不能删除。`,
    ))
  )
    return;
  const ok = await mutate(
    () => request(`/upstreams/${pool.id}`, { method: "DELETE" }),
    "上游池已删除",
  );
  if (ok !== undefined) await loadCore(true);
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
  document.addEventListener("keydown", keydown);
  void loadCore();
});
onBeforeUnmount(() => {
  document.removeEventListener("keydown", keydown);
  document.body.classList.remove("menu-open");
});
</script>

<template>
  <div class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-mark" aria-hidden="true">
          <span></span><span></span><span></span>
        </div>
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
          <span class="nav-icon">{{ item.icon }}</span
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
          <span>v{{ overview?.app_version ?? "0.1.1" }}</span
          ><span>Nginx {{ overview?.nginx_version ?? "1.30.4" }}</span>
        </div>
      </div>
    </aside>
    <div class="workspace">
      <header class="topbar">
        <button
          type="button"
          class="icon-button mobile-only"
          aria-label="打开菜单"
          @click="menuOpen = !menuOpen"
        >
          ☰
        </button>
        <div class="page-heading">
          <h1>{{ currentPage.label }}</h1>
          <p>{{ currentPage.subtitle }}</p>
        </div>
        <div class="top-actions">
          <button
            type="button"
            class="button ghost"
            :disabled="busy"
            @click="loadCore()"
          >
            刷新</button
          ><button
            type="button"
            class="button secondary"
            :disabled="busy"
            @click="runNginxAction('test')"
          >
            校验配置</button
          ><button
            type="button"
            class="button"
            :class="state?.dirty ? 'primary' : 'secondary'"
            :disabled="busy"
            @click="applyConfiguration"
          >
            {{ busy ? "处理中…" : state?.dirty ? "保存并应用 ·" : "重新应用" }}
          </button>
        </div>
      </header>
      <main class="content" aria-live="polite">
        <div v-if="loading" class="loading-panel">
          <div class="spinner"></div>
          <p>正在读取 nginx-web 状态…</p>
        </div>
        <article v-else-if="connectionError" class="card empty-state">
          <div class="empty-icon">!</div>
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
                {{ overview.nginx.running ? "✓" : "!" }}
              </div>
              <div>
                <h2>
                  {{
                    overview.nginx.running
                      ? "独立 Nginx 正常运行"
                      : "独立 Nginx 当前已停止"
                  }}
                </h2>
                <p>
                  {{
                    overview.nginx.running
                      ? `PID ${overview.nginx.pid} · 监听端口 ${ports} · 不使用飞牛系统 Nginx`
                      : "管理页面仍可使用，可以先校验配置再启动服务。"
                  }}
                </p>
              </div>
              <div class="status-banner-actions">
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
                v-for="metric in [
                  {
                    label: '代理规则',
                    value: overview.rule_count,
                    foot: `${overview.enabled_count} 条已启用`,
                    icon: '⇄',
                  },
                  {
                    label: '监听端口',
                    value: overview.nginx.ports.length,
                    foot: ports === '—' ? '默认端口尚未启动' : ports,
                    icon: '◎',
                  },
                  {
                    label: 'HTTPS 证书',
                    value: overview.certificate_count,
                    foot: certificateFoot(),
                    icon: '◇',
                  },
                  {
                    label: '配置状态',
                    value: overview.dirty ? '待应用' : '已同步',
                    foot: overview.last_applied_at
                      ? `上次应用 ${formatDate(overview.last_applied_at)}`
                      : '尚未正式应用',
                    icon: overview.dirty ? '!' : '✓',
                  },
                ]"
                :key="metric.label"
                class="card metric-card"
              >
                <div class="metric-label">
                  <span>{{ metric.label }}</span
                  ><span class="metric-icon">{{ metric.icon }}</span>
                </div>
                <div class="metric-value">{{ metric.value }}</div>
                <div class="metric-foot">{{ metric.foot }}</div>
              </article>
            </section>
            <section class="grid two-column section-gap">
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
                  <div
                    v-for="rule in activeRules.slice(0, 6)"
                    :key="rule.id"
                    class="detail-row"
                  >
                    <div>
                      <strong>{{ rule.name }}</strong>
                      <div class="rule-sub">{{ rule.domains.join(", ") }}</div>
                    </div>
                    <div style="text-align: right">
                      <span class="badge" :class="rule.tls ? 'success' : 'info'"
                        >{{ rule.tls ? "HTTPS" : "HTTP" }} :{{
                          rule.listen_port
                        }}</span
                      >
                      <div class="rule-sub">
                        → {{ rule.upstream_scheme }}://{{
                          formatHost(rule.upstream_host)
                        }}:{{ rule.upstream_port }}
                      </div>
                    </div>
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
              <aside class="grid">
                <article class="card">
                  <header class="card-header">
                    <div>
                      <h2>运行信息</h2>
                      <p>应用自己的进程与目录</p>
                    </div>
                  </header>
                  <div class="card-body">
                    <dl class="detail-list">
                      <div class="detail-row">
                        <dt>核心版本</dt>
                        <dd>
                          Nginx
                          {{ overview.nginx.version || overview.nginx_version }}
                        </dd>
                      </div>
                      <div class="detail-row">
                        <dt>配置文件</dt>
                        <dd>{{ overview.nginx.config_path }}</dd>
                      </div>
                      <div class="detail-row">
                        <dt>应用版本</dt>
                        <dd>nginx-web {{ overview.app_version }}</dd>
                      </div>
                      <div class="detail-row">
                        <dt>运行方式</dt>
                        <dd>原生 FPK · package 用户</dd>
                      </div>
                    </dl>
                  </div>
                </article>
                <article class="card">
                  <header class="card-header">
                    <div>
                      <h2>快捷操作</h2>
                      <p>常用管理入口</p>
                    </div>
                  </header>
                  <div class="card-body quick-actions">
                    <button class="quick-action" @click="openRule()">
                      <strong>添加代理</strong
                      ><span>创建 HTTP/HTTPS 规则</span></button
                    ><button class="quick-action" @click="openCertificate">
                      <strong>导入证书</strong
                      ><span>上传 PEM 证书与私钥</span></button
                    ><button
                      class="quick-action"
                      @click="runNginxAction('test')"
                    >
                      <strong>校验配置</strong
                      ><span>运行 nginx -t</span></button
                    ><button class="quick-action" @click="setPage('logs')">
                      <strong>查看日志</strong><span>定位启动或代理错误</span>
                    </button>
                  </div>
                </article>
              </aside>
            </section>
            <section
              v-if="overview.nginx.last_error"
              class="notice warning section-gap"
            >
              <strong>最近一条 Nginx 错误：</strong>
              {{ overview.nginx.last_error }}
            </section>
            <section v-if="overview.dirty" class="notice warning section-gap">
              当前草稿与运行中的 Nginx
              配置不一致。检查无误后点击右上角“保存并应用”。
            </section>
          </template>
          <template v-else-if="page === 'upstreams'"
            ><UpstreamPoolsPage
              :pools="state.upstream_pools"
              :busy="busy"
              @save="saveUpstreamPool"
              @remove="removeUpstreamPool"
          /></template>
          <template v-else-if="page === 'rules'"
            ><div class="toolbar">
              <input
                v-model="ruleSearch"
                class="input search-input"
                type="search"
                placeholder="搜索名称、域名或上游"
              /><span
                class="badge"
                :class="state.dirty ? 'warning' : 'success'"
                >{{ state.dirty ? "有未应用变更" : "配置已同步" }}</span
              ><span class="spacer"></span
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
                      <th>上游服务</th>
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
                          0.0.0.0:{{ rule.listen_port }}
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
                          {{ cert.dns_names.join(", ") || cert.subject }}
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
              <header class="card-header">
                <div>
                  <h2>实时日志</h2>
                  <p>最多读取最近 2000 行，不会把整个大日志载入浏览器</p>
                </div>
                <span class="spacer"></span>
                <div class="log-toolbar">
                  <select v-model="logType" class="select" @change="loadLogs">
                    <option value="error">Nginx 错误日志</option>
                    <option value="access">Nginx 访问日志</option>
                    <option value="backend">nginx-web 管理日志</option></select
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
              Nginx。检查草稿后，再点击右上角“保存并应用”。
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
                ><button class="button ghost small" @click="copyConfig">
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
              @save="saveSettings"
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
      role="dialog"
      aria-modal="true"
      :aria-labelledby="`${modal}-title`"
    >
      <header class="modal-header">
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
