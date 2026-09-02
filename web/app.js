(() => {
  "use strict";

  const gateway = location.pathname.startsWith("/app/nginx-web") ? "/app/nginx-web" : "";
  const apiRoot = `${gateway}/api`;
  const pageInfo = {
    dashboard: ["总览", "查看代理服务与配置状态"],
    rules: ["代理规则", "管理域名、监听端口与上游服务"],
    certificates: ["HTTPS 证书", "导入并管理手动 TLS 证书"],
    logs: ["运行日志", "查看 Nginx 与管理服务日志"],
    revisions: ["配置历史", "恢复或清理已应用的配置版本"],
    config: ["Nginx 配置", "查看 nginx-web 实际生成的配置文件"],
    settings: ["设置", "调整默认端口与历史保留策略"]
  };

  const model = {
    page: "dashboard",
    overview: null,
    state: null,
    revisions: [],
    config: null,
    configTab: "master",
    logType: "error",
    logLines: [],
    ruleSearch: "",
    busy: false
  };

  const els = {
    nav: document.getElementById("nav"),
    content: document.getElementById("content"),
    pageTitle: document.getElementById("page-title"),
    pageSubtitle: document.getElementById("page-subtitle"),
    refresh: document.getElementById("refresh-button"),
    test: document.getElementById("test-button"),
    apply: document.getElementById("apply-button"),
    mobileMenu: document.getElementById("mobile-menu"),
    sidebarStatus: document.getElementById("sidebar-status"),
    appVersion: document.getElementById("app-version"),
    nginxVersion: document.getElementById("nginx-version"),
    modalBackdrop: document.getElementById("modal-backdrop"),
    modal: document.getElementById("modal"),
    modalTitle: document.getElementById("modal-title"),
    modalDescription: document.getElementById("modal-description"),
    modalBody: document.getElementById("modal-body"),
    modalClose: document.getElementById("modal-close"),
    confirmBackdrop: document.getElementById("confirm-backdrop"),
    confirmTitle: document.getElementById("confirm-title"),
    confirmMessage: document.getElementById("confirm-message"),
    confirmCancel: document.getElementById("confirm-cancel"),
    confirmOK: document.getElementById("confirm-ok"),
    toasts: document.getElementById("toast-region")
  };

  async function request(endpoint, options = {}) {
    const init = { credentials: "same-origin", ...options };
    init.headers = new Headers(options.headers || {});
    if (init.body !== undefined && !(init.body instanceof FormData)) {
      init.headers.set("Content-Type", "application/json");
      if (typeof init.body !== "string") init.body = JSON.stringify(init.body);
    }
    if (init.method && init.method !== "GET" && init.method !== "HEAD") {
      init.headers.set("X-FnProxy-Request", "1");
    }
    const response = await fetch(`${apiRoot}${endpoint}`, init);
    const contentType = response.headers.get("content-type") || "";
    const payload = contentType.includes("application/json") ? await response.json() : await response.text();
    if (!response.ok) {
      const message = payload && payload.error ? payload.error : String(payload || `HTTP ${response.status}`);
      throw new Error(message);
    }
    return payload;
  }

  async function loadCore({ quiet = false } = {}) {
    if (!quiet) setBusy(true);
    try {
      const [overview, state, revisions] = await Promise.all([
        request("/overview"), request("/state"), request("/revisions")
      ]);
      model.overview = overview;
      model.state = state;
      model.revisions = revisions || [];
      updateChrome();
      await renderPage();
    } catch (error) {
      renderConnectionError(error);
      if (quiet) toast(error.message, "error");
    } finally {
      if (!quiet) setBusy(false);
    }
  }

  function updateChrome() {
    if (!model.overview) return;
    const running = model.overview.nginx.running;
    const dirty = model.overview.dirty;
    els.sidebarStatus.innerHTML = `<span class="status-dot ${running ? (dirty ? "warning" : "online") : "offline"}"></span><span>${running ? (dirty ? "运行中 · 有未应用变更" : "Nginx 正常运行") : "Nginx 已停止"}</span>`;
    els.appVersion.textContent = `v${model.overview.app_version}`;
    els.nginxVersion.textContent = `Nginx ${model.overview.nginx_version}`;
    els.apply.textContent = dirty ? "保存并应用 ·" : "重新应用";
    els.apply.classList.toggle("secondary", !dirty);
    els.apply.classList.toggle("primary", dirty);
  }

  async function renderPage() {
    const [title, subtitle] = pageInfo[model.page];
    els.pageTitle.textContent = title;
    els.pageSubtitle.textContent = subtitle;
    document.querySelectorAll(".nav-item").forEach(button => {
      button.classList.toggle("active", button.dataset.page === model.page);
    });
    switch (model.page) {
      case "dashboard": renderDashboard(); break;
      case "rules": renderRules(); break;
      case "certificates": renderCertificates(); break;
      case "logs": await renderLogs(); break;
      case "revisions": renderRevisions(); break;
      case "config": await renderConfig(); break;
      case "settings": renderSettings(); break;
      default: renderDashboard();
    }
  }

  function renderDashboard() {
    const o = model.overview;
    const state = model.state;
    const running = o.nginx.running;
    const activeRules = state.rules.filter(rule => rule.enabled);
    const ports = o.nginx.ports.length ? o.nginx.ports.join(", ") : "—";
    els.content.innerHTML = `
      <section class="status-banner ${running ? "" : "offline"}">
        <div class="status-orb">${running ? "✓" : "!"}</div>
        <div>
          <h2>${running ? "独立 Nginx 正常运行" : "独立 Nginx 当前已停止"}</h2>
          <p>${running ? `PID ${o.nginx.pid} · 监听端口 ${escapeHTML(ports)} · 不使用飞牛系统 Nginx` : "管理页面仍可使用，可以先校验配置再启动服务。"}</p>
        </div>
        <div class="status-banner-actions">
          ${running ? `<button class="button secondary small" data-action="nginx-reload">平滑重载</button><button class="button danger-ghost small" data-action="nginx-stop">停止</button>` : `<button class="button primary small" data-action="nginx-start">启动 Nginx</button>`}
        </div>
      </section>

      <section class="grid metric-grid section-gap">
        ${metricCard("代理规则", o.rule_count, `${o.enabled_count} 条已启用`, "⇄")}
        ${metricCard("监听端口", o.nginx.ports.length, ports === "—" ? "默认端口尚未启动" : ports, "◎")}
        ${metricCard("HTTPS 证书", o.certificate_count, certificateFoot(state.certificates), "◇")}
        ${metricCard("配置状态", o.dirty ? "待应用" : "已同步", o.last_applied_at ? `上次应用 ${formatDate(o.last_applied_at)}` : "尚未正式应用", o.dirty ? "!" : "✓")}
      </section>

      <section class="grid two-column section-gap">
        <article class="card">
          <header class="card-header"><div><h2>当前代理</h2><p>已启用规则会被写入独立 Nginx 配置</p></div><span class="spacer"></span><button class="button ghost small" data-action="open-rules">查看全部</button></header>
          <div class="card-body">${activeRules.length ? activeRules.slice(0, 6).map(rule => `
            <div class="detail-row">
              <div><strong>${escapeHTML(rule.name)}</strong><div class="rule-sub">${escapeHTML(rule.domains.join(", "))}</div></div>
              <div style="text-align:right"><span class="badge ${rule.tls ? "success" : "info"}">${rule.tls ? "HTTPS" : "HTTP"} :${rule.listen_port}</span><div class="rule-sub">→ ${escapeHTML(rule.upstream_scheme)}://${escapeHTML(formatHost(rule.upstream_host))}:${rule.upstream_port}</div></div>
            </div>`).join("") : emptyInline("还没有启用的代理规则", "添加规则后，保存并应用即可开始转发。")}</div>
        </article>
        <aside class="grid">
          <article class="card">
            <header class="card-header"><div><h2>运行信息</h2><p>应用自己的进程与目录</p></div></header>
            <div class="card-body"><dl class="detail-list">
              ${detail("核心版本", `Nginx ${escapeHTML(o.nginx.version || o.nginx_version)}`)}
              ${detail("配置文件", escapeHTML(o.nginx.config_path))}
              ${detail("应用版本", `nginx-web ${escapeHTML(o.app_version)}`)}
              ${detail("运行方式", "原生 FPK · package 用户")}
            </dl></div>
          </article>
          <article class="card">
            <header class="card-header"><div><h2>快捷操作</h2><p>常用管理入口</p></div></header>
            <div class="card-body quick-actions">
              <button class="quick-action" data-action="add-rule"><strong>添加代理</strong><span>创建 HTTP/HTTPS 规则</span></button>
              <button class="quick-action" data-action="add-certificate"><strong>导入证书</strong><span>上传 PEM 证书与私钥</span></button>
              <button class="quick-action" data-action="nginx-test"><strong>校验配置</strong><span>运行 nginx -t</span></button>
              <button class="quick-action" data-action="open-logs"><strong>查看日志</strong><span>定位启动或代理错误</span></button>
            </div>
          </article>
        </aside>
      </section>
      ${o.nginx.last_error ? `<section class="notice warning section-gap"><strong>最近一条 Nginx 错误：</strong> ${escapeHTML(o.nginx.last_error)}</section>` : ""}
      ${o.dirty ? `<section class="notice warning section-gap">当前草稿与运行中的 Nginx 配置不一致。检查无误后点击右上角“保存并应用”。</section>` : ""}
    `;
  }

  function renderRules() {
    const rules = model.state.rules || [];
    const term = model.ruleSearch.trim().toLowerCase();
    const filtered = rules.filter(rule => !term || [rule.name, ...rule.domains, rule.upstream_host, String(rule.upstream_port)].join(" ").toLowerCase().includes(term));
    els.content.innerHTML = `
      <div class="toolbar">
        <input id="rule-search" class="input search-input" type="search" placeholder="搜索名称、域名或上游" value="${escapeAttr(model.ruleSearch)}">
        <span class="badge ${model.state.dirty ? "warning" : "success"}">${model.state.dirty ? "有未应用变更" : "配置已同步"}</span>
        <span class="spacer"></span>
        <button class="button primary" data-action="add-rule">＋ 添加代理规则</button>
      </div>
      <article class="card">
        ${filtered.length ? `<div class="table-wrap"><table class="table">
          <thead><tr><th>状态</th><th>名称与域名</th><th>入口</th><th>上游服务</th><th class="hide-mobile">能力</th><th></th></tr></thead>
          <tbody>${filtered.map(rule => `
            <tr>
              <td><label class="switch" title="${rule.enabled ? "停用" : "启用"}"><input type="checkbox" data-action="toggle-rule" data-id="${rule.id}" ${rule.enabled ? "checked" : ""}><span></span></label></td>
              <td><div class="rule-name">${escapeHTML(rule.name)}</div><div class="domain-list rule-sub">${rule.domains.map(domain => `<span class="domain-chip">${escapeHTML(domain)}</span>`).join("")}</div></td>
              <td><span class="badge ${rule.tls ? "success" : "info"}">${rule.tls ? "HTTPS" : "HTTP"}</span><div class="rule-sub">0.0.0.0:${rule.listen_port}</div></td>
              <td><div>${escapeHTML(rule.upstream_scheme)}://${escapeHTML(formatHost(rule.upstream_host))}:${rule.upstream_port}</div><div class="rule-sub">连接 ${rule.connect_timeout_seconds}s · 读取 ${rule.read_timeout_seconds}s</div></td>
              <td class="hide-mobile"><div class="domain-list">${rule.websocket ? `<span class="badge neutral">WebSocket</span>` : ""}${rule.streaming ? `<span class="badge neutral">流式</span>` : ""}${rule.http2 && rule.tls ? `<span class="badge neutral">HTTP/2</span>` : ""}</div></td>
              <td><div class="table-actions"><button class="button ghost small" data-action="edit-rule" data-id="${rule.id}">编辑</button><button class="button danger-ghost small" data-action="delete-rule" data-id="${rule.id}">删除</button></div></td>
            </tr>`).join("")}</tbody>
        </table></div>` : emptyState("⇄", rules.length ? "没有匹配的规则" : "还没有代理规则", rules.length ? "换一个关键词再试。" : "添加第一条规则，将域名或端口转发到 NAS、Docker 或局域网服务。", rules.length ? "" : `<button class="button primary" data-action="add-rule">添加代理规则</button>`) }
      </article>
      <div class="notice section-gap">第一版只允许监听 1024–65535 的非特权端口，默认 HTTP 端口为 ${model.state.settings.default_http_port}。这可以避免使用 root 权限，也不会抢占飞牛系统的 80/443。</div>
    `;
    const search = document.getElementById("rule-search");
    if (search) search.addEventListener("input", event => { model.ruleSearch = event.target.value; renderRules(); document.getElementById("rule-search")?.focus(); });
  }

  function renderCertificates() {
    const certificates = model.state.certificates || [];
    els.content.innerHTML = `
      <div class="toolbar"><div class="notice">nginx-web 只保存 PEM 文件，不会把私钥返回到浏览器。证书目录权限为 0700，私钥文件权限为 0600。</div><span class="spacer"></span><button class="button primary" data-action="add-certificate">＋ 导入证书</button></div>
      <article class="card">
        ${certificates.length ? `<div class="table-wrap"><table class="table">
          <thead><tr><th>名称</th><th>域名 / 主体</th><th>有效期</th><th>指纹</th><th></th></tr></thead>
          <tbody>${certificates.map(cert => {
            const expired = new Date(cert.not_after).getTime() < Date.now();
            const soon = !expired && new Date(cert.not_after).getTime() - Date.now() < 30 * 86400000;
            return `<tr>
              <td><div class="rule-name">${escapeHTML(cert.name)}</div><div class="rule-sub">${escapeHTML(cert.serial_number)}</div></td>
              <td><div>${escapeHTML((cert.dns_names || []).join(", ") || cert.subject)}</div><div class="rule-sub">${escapeHTML(cert.subject)}</div></td>
              <td><span class="badge ${expired ? "danger" : soon ? "warning" : "success"}">${expired ? "已过期" : soon ? "即将过期" : "有效"}</span><div class="rule-sub">${formatDate(cert.not_before, true)} ～ ${formatDate(cert.not_after, true)}</div></td>
              <td><code title="${escapeAttr(cert.fingerprint)}">${escapeHTML(shortFingerprint(cert.fingerprint))}</code></td>
              <td><div class="table-actions"><button class="button danger-ghost small" data-action="delete-certificate" data-id="${cert.id}">删除</button></div></td>
            </tr>`;
          }).join("")}</tbody>
        </table></div>` : emptyState("◇", "尚未导入 HTTPS 证书", "第一版支持手动导入完整证书链和对应私钥。自动 ACME 申请将在后续版本增加。", `<button class="button primary" data-action="add-certificate">导入证书</button>`) }
      </article>
    `;
  }

  async function renderLogs() {
    els.content.innerHTML = `<article class="card"><header class="card-header"><div><h2>实时日志</h2><p>最多读取最近 2000 行，不会把整个大日志载入浏览器</p></div><span class="spacer"></span><div class="log-toolbar"><select id="log-type" class="select"><option value="error">Nginx 错误日志</option><option value="access">Nginx 访问日志</option><option value="backend">nginx-web 管理日志</option></select><button class="button ghost small" data-action="refresh-logs">刷新</button></div></header><pre id="log-view" class="log-view">正在读取…</pre></article>`;
    document.getElementById("log-type").value = model.logType;
    document.getElementById("log-type").addEventListener("change", async event => {
      model.logType = event.target.value;
      await loadLogs();
    });
    await loadLogs();
  }

  async function loadLogs() {
    const view = document.getElementById("log-view");
    if (!view) return;
    view.textContent = "正在读取…";
    try {
      const result = await request(`/logs?type=${encodeURIComponent(model.logType)}&lines=500`);
      model.logLines = result.lines || [];
      view.textContent = model.logLines.length ? model.logLines.join("\n") : "暂无日志。";
      view.scrollTop = view.scrollHeight;
    } catch (error) {
      view.textContent = `读取失败：${error.message}`;
    }
  }

  function renderRevisions() {
    const revisions = model.revisions || [];
    els.content.innerHTML = `
      <article class="card">
        <header class="card-header"><div><h2>已应用版本</h2><p>每次“保存并应用”后自动保留，当前上限 ${model.state.settings.revision_limit} 个</p></div></header>
        ${revisions.length ? `<div class="table-wrap"><table class="table"><thead><tr><th>时间</th><th>说明</th><th>规则</th><th></th></tr></thead><tbody>${revisions.map(revision => `
          <tr><td>${formatDate(revision.created_at)}</td><td><div class="rule-name">${escapeHTML(revision.summary || "配置快照")}</div><div class="rule-sub">${escapeHTML(revision.id)}</div></td><td>${revision.enabled_count} / ${revision.rule_count} 条启用</td><td><div class="table-actions"><button class="button ghost small" data-action="restore-revision" data-id="${escapeAttr(revision.id)}">恢复为草稿</button><button class="button danger-ghost small" data-action="delete-revision" data-id="${escapeAttr(revision.id)}">删除</button></div></td></tr>`).join("")}</tbody></table></div>` : emptyState("↶", "还没有配置历史", "首次保存并应用成功后，这里会出现可恢复的版本。", "")}
      </article>
      <div class="notice warning section-gap">恢复历史只会恢复规则和设置，不会立刻重载 Nginx。检查草稿后，再点击右上角“保存并应用”。</div>
    `;
  }

  async function renderConfig() {
    els.content.innerHTML = `<article class="card"><header class="card-header"><div><h2>生成的只读配置</h2><p>配置由结构化规则生成，避免直接写入危险指令</p></div><span class="spacer"></span><button class="button ghost small" data-action="copy-config">复制当前文件</button></header><div id="config-tabs" class="code-tabs"></div><pre id="config-view" class="code-view">正在读取…</pre></article>`;
    try {
      model.config = await request("/config");
      const keys = ["master", ...Object.keys(model.config.files || {}).sort()];
      if (!keys.includes(model.configTab)) model.configTab = "master";
      const tabs = document.getElementById("config-tabs");
      tabs.innerHTML = keys.map(key => `<button class="code-tab ${key === model.configTab ? "active" : ""}" data-action="config-tab" data-id="${escapeAttr(key)}">${key === "master" ? "nginx.conf" : escapeHTML(key)}</button>`).join("");
      updateConfigView();
    } catch (error) {
      document.getElementById("config-view").textContent = `读取失败：${error.message}`;
    }
  }

  function updateConfigView() {
    const view = document.getElementById("config-view");
    if (!view || !model.config) return;
    view.textContent = model.configTab === "master" ? (model.config.master || "") : (model.config.files[model.configTab] || "");
    document.querySelectorAll(".code-tab").forEach(tab => tab.classList.toggle("active", tab.dataset.id === model.configTab));
  }

  function renderSettings() {
    const settings = model.state.settings;
    els.content.innerHTML = `
      <article class="card">
        <header class="card-header"><div><h2>基础设置</h2><p>这些值只用于新规则和无规则时的默认站点</p></div></header>
        <div class="card-body"><form id="settings-form" class="form-grid">
          ${field("默认 HTTP 端口", `<input class="input" name="default_http_port" type="number" min="1024" max="65535" required value="${settings.default_http_port}">`, "没有启用规则时，Nginx 会在该端口返回 404。")}
          ${field("默认 HTTPS 端口", `<input class="input" name="default_https_port" type="number" min="1024" max="65535" required value="${settings.default_https_port}">`, "创建 HTTPS 规则时使用的默认值。")}
          ${field("配置历史保留数量", `<input class="input" name="revision_limit" type="number" min="1" max="100" required value="${settings.revision_limit}">`, "范围 1–100，超过后自动删除最旧版本。")}
          <div class="full notice warning">nginx-web 0.1.1 不申请 root 权限，因此不能直接监听 80/443。需要标准公网端口时，请在路由器上将公网 80/443 映射到这里配置的高位端口。</div>
          <div class="full modal-footer" style="padding-left:0;padding-right:0;padding-bottom:0"><button class="button primary" type="submit">保存设置</button></div>
        </form></div>
      </article>
      <article class="card section-gap"><header class="card-header"><div><h2>安全边界</h2><p>首版固定策略</p></div></header><div class="card-body"><dl class="detail-list">
        ${detail("系统 Nginx", "不读取、不修改、不重启")}
        ${detail("运行用户", "nginx-web 专用 package 用户")}
        ${detail("Docker", "不依赖，不挂载 docker.sock")}
        ${detail("管理入口", "fnOS 统一网关 + Unix Socket")}
        ${detail("原始配置", "只读展示，不允许网页任意编辑")}
      </dl></div></article>
    `;
    document.getElementById("settings-form").addEventListener("submit", saveSettings);
  }

  async function saveSettings(event) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    try {
      await withBusy(event.submitter, async () => {
        await request("/settings", { method: "PUT", body: {
          default_http_port: Number(data.get("default_http_port")),
          default_https_port: Number(data.get("default_https_port")),
          revision_limit: Number(data.get("revision_limit"))
        }});
      });
      toast("设置已保存为草稿", "success");
      await loadCore({ quiet: true });
    } catch (error) { toast(error.message, "error"); }
  }

  function openRuleModal(id = "") {
    const editing = model.state.rules.find(item => item.id === id);
    const settings = model.state.settings;
    const rule = editing || {
      name: "", enabled: true, listen_port: settings.default_http_port, domains: [], tls: false, http2: true,
      certificate_id: "", upstream_scheme: "http", upstream_host: "127.0.0.1", upstream_port: 8080,
      preserve_host: true, websocket: true, streaming: true, verify_upstream_tls: false,
      connect_timeout_seconds: 10, read_timeout_seconds: 3600, send_timeout_seconds: 3600, client_max_body_mb: 0
    };
    const certOptions = model.state.certificates.map(cert => `<option value="${cert.id}" ${cert.id === rule.certificate_id ? "selected" : ""}>${escapeHTML(cert.name)} · ${formatDate(cert.not_after, true)}</option>`).join("");
    openModal(editing ? "编辑代理规则" : "添加代理规则", "保存前会进行结构化校验；正式应用时还会执行 nginx -t。", `
      <form id="rule-form" class="form-grid">
        ${field("规则名称", `<input class="input" name="name" required maxlength="80" value="${escapeAttr(rule.name)}" placeholder="例如：Jellyfin">`)}
        <div class="field"><label>规则状态</label><label class="checkbox-row"><input type="checkbox" name="enabled" ${rule.enabled ? "checked" : ""}> 启用此规则</label></div>
        ${field("访问域名 / IP", `<textarea class="textarea" name="domains" required placeholder="jellyfin.example.com&#10;media.example.com">${escapeHTML((rule.domains || []).join("\n"))}</textarea>`, "多个域名可用换行、空格或逗号分隔；使用 * 表示该端口的默认站点。", "full")}
        <div class="form-section">入口设置</div>
        ${field("监听端口", `<input id="rule-listen-port" class="input" name="listen_port" type="number" min="1024" max="65535" required value="${rule.listen_port}">`, "仅允许非特权端口。")}
        <div class="field"><label>入口协议</label><label class="checkbox-row"><input id="rule-tls" type="checkbox" name="tls" ${rule.tls ? "checked" : ""}> 启用 HTTPS</label></div>
        <div id="certificate-field" class="field ${rule.tls ? "" : "hidden"}"><label>HTTPS 证书</label><select class="select" name="certificate_id"><option value="">请选择证书</option>${certOptions}</select><span class="field-help">没有证书时，请先到“HTTPS 证书”页面导入。</span></div>
        <div id="http2-field" class="field ${rule.tls ? "" : "hidden"}"><label>HTTP/2</label><label class="checkbox-row"><input type="checkbox" name="http2" ${rule.http2 ? "checked" : ""}> 启用 HTTP/2</label></div>
        <div class="form-section">上游服务</div>
        ${field("上游协议", `<select id="upstream-scheme" class="select" name="upstream_scheme"><option value="http" ${rule.upstream_scheme === "http" ? "selected" : ""}>HTTP</option><option value="https" ${rule.upstream_scheme === "https" ? "selected" : ""}>HTTPS</option></select>`)}
        ${field("上游主机", `<input class="input" name="upstream_host" required value="${escapeAttr(rule.upstream_host)}" placeholder="127.0.0.1 或 192.168.1.20">`)}
        ${field("上游端口", `<input class="input" name="upstream_port" type="number" min="1" max="65535" required value="${rule.upstream_port}">`)}
        <div id="verify-tls-field" class="field ${rule.upstream_scheme === "https" ? "" : "hidden"}"><label>上游证书校验</label><label class="checkbox-row"><input type="checkbox" name="verify_upstream_tls" ${rule.verify_upstream_tls ? "checked" : ""}> 校验上游 HTTPS 证书</label></div>
        <div class="form-section">代理能力</div>
        <div class="field"><label>请求 Host</label><label class="checkbox-row"><input type="checkbox" name="preserve_host" ${rule.preserve_host ? "checked" : ""}> 保留客户端 Host</label></div>
        <div class="field"><label>WebSocket</label><label class="checkbox-row"><input type="checkbox" name="websocket" ${rule.websocket ? "checked" : ""}> 转发连接升级头</label></div>
        <div class="field"><label>流式传输</label><label class="checkbox-row"><input type="checkbox" name="streaming" ${rule.streaming ? "checked" : ""}> 关闭代理缓冲</label></div>
        ${field("请求体上限（MB）", `<input class="input" name="client_max_body_mb" type="number" min="0" max="102400" value="${rule.client_max_body_mb}">`, "0 表示不限制。")}
        <div class="form-section">超时设置</div>
        ${field("连接超时（秒）", `<input class="input" name="connect_timeout_seconds" type="number" min="1" max="600" value="${rule.connect_timeout_seconds}">`)}
        ${field("读取超时（秒）", `<input class="input" name="read_timeout_seconds" type="number" min="1" max="86400" value="${rule.read_timeout_seconds}">`)}
        ${field("发送超时（秒）", `<input class="input" name="send_timeout_seconds" type="number" min="1" max="86400" value="${rule.send_timeout_seconds}">`)}
        <div class="field"><label>保存方式</label><label class="checkbox-row"><input type="checkbox" name="apply_after" checked> 保存后立即应用</label></div>
        <footer class="modal-footer full"><button type="button" class="button ghost" data-action="close-modal">取消</button><button type="submit" class="button primary">${editing ? "保存修改" : "创建规则"}</button></footer>
      </form>
    `);
    const form = document.getElementById("rule-form");
    const tls = document.getElementById("rule-tls");
    const port = document.getElementById("rule-listen-port");
    tls.addEventListener("change", () => {
      document.getElementById("certificate-field").classList.toggle("hidden", !tls.checked);
      document.getElementById("http2-field").classList.toggle("hidden", !tls.checked);
      if (!editing || Number(port.value) === settings.default_http_port || Number(port.value) === settings.default_https_port) {
        port.value = tls.checked ? settings.default_https_port : settings.default_http_port;
      }
    });
    document.getElementById("upstream-scheme").addEventListener("change", event => {
      document.getElementById("verify-tls-field").classList.toggle("hidden", event.target.value !== "https");
    });
    form.addEventListener("submit", event => saveRule(event, id));
  }

  async function saveRule(event, id) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const rule = {
      name: String(data.get("name") || ""), enabled: data.has("enabled"),
      listen_port: Number(data.get("listen_port")), domains: String(data.get("domains") || "").split(/[\s,]+/).filter(Boolean),
      tls: data.has("tls"), http2: data.has("http2"), certificate_id: String(data.get("certificate_id") || ""),
      upstream_scheme: String(data.get("upstream_scheme") || "http"), upstream_host: String(data.get("upstream_host") || ""),
      upstream_port: Number(data.get("upstream_port")), preserve_host: data.has("preserve_host"), websocket: data.has("websocket"),
      streaming: data.has("streaming"), verify_upstream_tls: data.has("verify_upstream_tls"),
      connect_timeout_seconds: Number(data.get("connect_timeout_seconds")), read_timeout_seconds: Number(data.get("read_timeout_seconds")),
      send_timeout_seconds: Number(data.get("send_timeout_seconds")), client_max_body_mb: Number(data.get("client_max_body_mb"))
    };
    try {
      await withBusy(event.submitter, async () => {
        await request(id ? `/rules/${id}` : "/rules", { method: id ? "PUT" : "POST", body: rule });
        if (data.has("apply_after")) await request("/apply", { method: "POST", body: { summary: `${id ? "修改" : "新增"}规则：${rule.name}` } });
      });
      closeModal();
      toast(data.has("apply_after") ? "规则已保存并应用" : "规则已保存为草稿", "success");
      await loadCore({ quiet: true });
    } catch (error) { toast(error.message, "error"); }
  }

  function openCertificateModal() {
    openModal("导入 HTTPS 证书", "证书链必须与私钥匹配，支持 RSA 和 ECDSA PEM。", `
      <form id="certificate-form" class="form-grid">
        ${field("证书名称", `<input class="input" name="name" maxlength="80" placeholder="例如：example.com">`, "留空时自动使用证书 CN 或第一个域名。", "full")}
        ${field("完整证书链（PEM）", `<textarea class="textarea" name="certificate" required rows="10" spellcheck="false" placeholder="-----BEGIN CERTIFICATE-----"></textarea>`, "建议包含站点证书及中间证书。", "full")}
        ${field("私钥（PEM）", `<textarea class="textarea" name="private_key" required rows="8" spellcheck="false" placeholder="-----BEGIN PRIVATE KEY-----"></textarea>`, "私钥只发送给本机 nginx-web，并以 0600 权限保存。", "full")}
        <footer class="modal-footer full"><button type="button" class="button ghost" data-action="close-modal">取消</button><button type="submit" class="button primary">导入证书</button></footer>
      </form>`);
    document.getElementById("certificate-form").addEventListener("submit", async event => {
      event.preventDefault();
      const data = new FormData(event.currentTarget);
      try {
        await withBusy(event.submitter, () => request("/certificates", { method: "POST", body: {
          name: String(data.get("name") || ""), certificate: String(data.get("certificate") || ""), private_key: String(data.get("private_key") || "")
        }}));
        closeModal();
        toast("证书已安全导入", "success");
        await loadCore({ quiet: true });
      } catch (error) { toast(error.message, "error"); }
    });
  }

  async function runNginxAction(action) {
    const labels = { start: "启动", stop: "停止", reload: "重载", test: "校验" };
    try {
      const result = await request(`/nginx/${action}`, { method: "POST", body: {} });
      toast(result.message || `${labels[action]}成功`, "success");
      await loadCore({ quiet: true });
    } catch (error) { toast(error.message, "error"); }
  }

  async function applyConfiguration() {
    try {
      await withBusy(els.apply, async () => {
        const result = await request("/apply", { method: "POST", body: { summary: "从管理页面保存并应用" } });
        toast(result.message || "配置已应用", "success");
      });
      await loadCore({ quiet: true });
    } catch (error) { toast(error.message, "error"); }
  }

  async function handleContentAction(action, id, target) {
    switch (action) {
      case "add-rule": openRuleModal(); break;
      case "edit-rule": openRuleModal(id); break;
      case "delete-rule": {
        const rule = model.state.rules.find(item => item.id === id);
        if (await confirmAction("删除代理规则", `确定删除“${rule?.name || id}”吗？删除后仍需应用配置才会影响运行中的 Nginx。`)) {
          try { await request(`/rules/${id}`, { method: "DELETE" }); toast("规则已删除", "success"); await loadCore({ quiet: true }); } catch (error) { toast(error.message, "error"); }
        }
        break;
      }
      case "toggle-rule": {
        const rule = model.state.rules.find(item => item.id === id);
        if (!rule) break;
        const next = { ...rule, enabled: target.checked };
        try { await request(`/rules/${id}`, { method: "PUT", body: cleanRule(next) }); toast(target.checked ? "规则已启用，等待应用" : "规则已停用，等待应用", "success"); await loadCore({ quiet: true }); } catch (error) { target.checked = !target.checked; toast(error.message, "error"); }
        break;
      }
      case "add-certificate": openCertificateModal(); break;
      case "delete-certificate": {
        const cert = model.state.certificates.find(item => item.id === id);
        if (await confirmAction("删除 HTTPS 证书", `确定删除“${cert?.name || id}”吗？该操作会同时删除本机保存的私钥，且无法恢复。`)) {
          try { await request(`/certificates/${id}`, { method: "DELETE" }); toast("证书已删除", "success"); await loadCore({ quiet: true }); } catch (error) { toast(error.message, "error"); }
        }
        break;
      }
      case "nginx-start": await runNginxAction("start"); break;
      case "nginx-stop": if (await confirmAction("停止独立 Nginx", "停止后所有由 nginx-web 提供的代理入口都会暂时不可访问，但不会影响飞牛系统服务。")) await runNginxAction("stop"); break;
      case "nginx-reload": await runNginxAction("reload"); break;
      case "nginx-test": await runNginxAction("test"); break;
      case "open-rules": setPage("rules"); break;
      case "open-logs": setPage("logs"); break;
      case "refresh-logs": await loadLogs(); break;
      case "restore-revision": if (await confirmAction("恢复配置历史", "历史版本会恢复为草稿，不会立刻影响当前代理。")) {
        try { await request(`/revisions/${encodeURIComponent(id)}/restore`, { method: "POST", body: {} }); toast("历史版本已恢复为草稿", "success"); await loadCore({ quiet: true }); } catch (error) { toast(error.message, "error"); }
      } break;
      case "delete-revision": if (await confirmAction("删除配置历史", "删除后不能再恢复该版本。")) {
        try { await request(`/revisions/${encodeURIComponent(id)}`, { method: "DELETE" }); toast("配置历史已删除", "success"); await loadCore({ quiet: true }); } catch (error) { toast(error.message, "error"); }
      } break;
      case "config-tab": model.configTab = id; updateConfigView(); break;
      case "copy-config": {
        const text = model.configTab === "master" ? model.config?.master : model.config?.files?.[model.configTab];
        try { await navigator.clipboard.writeText(text || ""); toast("配置已复制", "success"); } catch { toast("浏览器不允许复制，请手动选择文本", "error"); }
        break;
      }
      case "close-modal": closeModal(); break;
    }
  }

  function cleanRule(rule) {
    const allowed = ["name","enabled","listen_port","domains","tls","http2","certificate_id","upstream_scheme","upstream_host","upstream_port","preserve_host","websocket","streaming","verify_upstream_tls","connect_timeout_seconds","read_timeout_seconds","send_timeout_seconds","client_max_body_mb"];
    return Object.fromEntries(allowed.map(key => [key, rule[key]]));
  }

  function setPage(page) {
    if (!pageInfo[page]) return;
    model.page = page;
    document.body.classList.remove("menu-open");
    renderPage();
  }

  function openModal(title, description, body) {
    els.modalTitle.textContent = title;
    els.modalDescription.textContent = description || "";
    els.modalBody.innerHTML = body;
    els.modalBackdrop.classList.remove("hidden");
    els.modalBackdrop.setAttribute("aria-hidden", "false");
    setTimeout(() => els.modalBody.querySelector("input,select,textarea,button")?.focus(), 20);
  }

  function closeModal() {
    els.modalBackdrop.classList.add("hidden");
    els.modalBackdrop.setAttribute("aria-hidden", "true");
    els.modalBody.innerHTML = "";
  }

  function confirmAction(title, message) {
    els.confirmTitle.textContent = title;
    els.confirmMessage.textContent = message;
    els.confirmBackdrop.classList.remove("hidden");
    els.confirmBackdrop.setAttribute("aria-hidden", "false");
    return new Promise(resolve => {
      const finish = value => {
        els.confirmBackdrop.classList.add("hidden");
        els.confirmBackdrop.setAttribute("aria-hidden", "true");
        els.confirmOK.onclick = null;
        els.confirmCancel.onclick = null;
        resolve(value);
      };
      els.confirmOK.onclick = () => finish(true);
      els.confirmCancel.onclick = () => finish(false);
    });
  }

  function toast(message, type = "") {
    const item = document.createElement("div");
    item.className = `toast ${type}`;
    const icon = document.createElement("b");
    icon.textContent = type === "success" ? "✓" : type === "error" ? "!" : "i";
    const text = document.createElement("span");
    text.textContent = message;
    item.append(icon, text);
    els.toasts.appendChild(item);
    setTimeout(() => item.remove(), 5200);
  }

  function setBusy(value) {
    model.busy = value;
    [els.refresh, els.test, els.apply].forEach(button => button.disabled = value);
  }

  async function withBusy(button, fn) {
    if (!button) return fn();
    const text = button.textContent;
    button.disabled = true;
    button.textContent = "处理中…";
    try { return await fn(); } finally { button.disabled = false; button.textContent = text; }
  }

  function renderConnectionError(error) {
    els.sidebarStatus.innerHTML = `<span class="status-dot offline"></span><span>连接管理服务失败</span>`;
    els.content.innerHTML = `<article class="card empty-state"><div class="empty-icon">!</div><h3>无法读取 nginx-web 状态</h3><p>${escapeHTML(error.message)}</p><button class="button primary" data-action="retry">重新连接</button></article>`;
  }

  function metricCard(label, value, foot, icon) {
    return `<article class="card metric-card"><div class="metric-label"><span>${escapeHTML(label)}</span><span class="metric-icon">${escapeHTML(icon)}</span></div><div class="metric-value">${escapeHTML(String(value))}</div><div class="metric-foot">${escapeHTML(String(foot))}</div></article>`;
  }

  function detail(label, value) { return `<div class="detail-row"><dt>${escapeHTML(label)}</dt><dd>${value}</dd></div>`; }
  function field(label, control, help = "", extra = "") { return `<div class="field ${extra}"><label>${escapeHTML(label)}</label>${control}${help ? `<span class="field-help">${escapeHTML(help)}</span>` : ""}</div>`; }
  function emptyState(icon, title, text, action) { return `<div class="empty-state"><div class="empty-icon">${escapeHTML(icon)}</div><h3>${escapeHTML(title)}</h3><p>${escapeHTML(text)}</p>${action}</div>`; }
  function emptyInline(title, text) { return `<div class="empty-state" style="padding:24px"><h3>${escapeHTML(title)}</h3><p>${escapeHTML(text)}</p></div>`; }
  function certificateFoot(certs) {
    if (!certs.length) return "尚未导入证书";
    const valid = certs.filter(cert => new Date(cert.not_after).getTime() > Date.now()).length;
    return `${valid} 张仍在有效期`;
  }
  function shortFingerprint(value = "") { return value.length > 23 ? `${value.slice(0, 17)}…${value.slice(-5)}` : value; }
  function formatHost(host = "") { return host.includes(":") && !host.startsWith("[") ? `[${host}]` : host; }
  function formatDate(value, dateOnly = false) {
    if (!value) return "—";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return String(value);
    return new Intl.DateTimeFormat("zh-CN", dateOnly ? { year: "numeric", month: "2-digit", day: "2-digit" } : { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }).format(date);
  }
  function escapeHTML(value) { return String(value ?? "").replace(/[&<>'"]/g, char => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" })[char]); }
  function escapeAttr(value) { return escapeHTML(value).replace(/`/g, "&#96;"); }

  els.nav.addEventListener("click", event => {
    const button = event.target.closest("[data-page]");
    if (button) setPage(button.dataset.page);
  });
  els.content.addEventListener("click", async event => {
    const target = event.target.closest("[data-action]");
    if (!target) return;
    if (target.dataset.action === "retry") { await loadCore(); return; }
    if (target.dataset.action === "toggle-rule") return;
    await handleContentAction(target.dataset.action, target.dataset.id || "", target);
  });
  els.content.addEventListener("change", async event => {
    const target = event.target.closest("[data-action='toggle-rule']");
    if (target) await handleContentAction("toggle-rule", target.dataset.id, target);
  });
  els.modalBody.addEventListener("click", event => {
    const target = event.target.closest("[data-action='close-modal']");
    if (target) closeModal();
  });
  els.modalClose.addEventListener("click", closeModal);
  els.modalBackdrop.addEventListener("click", event => { if (event.target === els.modalBackdrop) closeModal(); });
  els.mobileMenu.addEventListener("click", () => document.body.classList.toggle("menu-open"));
  els.refresh.addEventListener("click", () => loadCore());
  els.test.addEventListener("click", () => runNginxAction("test"));
  els.apply.addEventListener("click", applyConfiguration);
  document.addEventListener("keydown", event => { if (event.key === "Escape") closeModal(); });

  loadCore();
})();
