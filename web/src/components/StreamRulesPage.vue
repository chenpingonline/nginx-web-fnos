<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import type {
  CertificateMeta,
  SNIRoute,
  StreamRule,
  StreamRuleInput,
  UpstreamPool,
} from "../types";
const props = defineProps<{
  rules: StreamRule[];
  pools: UpstreamPool[];
  certificates: CertificateMeta[];
  busy: boolean;
  dirty: boolean;
}>();
const emit = defineEmits<{
  save: [value: StreamRuleInput, id: string];
  remove: [rule: StreamRule];
  toggle: [rule: StreamRule, enabled: boolean];
  refresh: [];
  apply: [];
}>();
const open = ref(false),
  editing = ref<StreamRule | null>(null),
  trustedText = ref(""),
  allowText = ref(""),
  denyText = ref("");
const blankRoute = (): SNIRoute => ({
  server_names: [],
  upstream_pool_id: "",
  upstream_host: "127.0.0.1",
  upstream_port: 443,
});
const blank = (): StreamRuleInput => ({
  name: "",
  enabled: true,
  protocol: "tcp",
  listen_address: "0.0.0.0",
  listen_port: 10000,
  upstream_pool_id: "",
  upstream_host: "127.0.0.1",
  upstream_port: 8080,
  connect_timeout_seconds: 10,
  proxy_timeout_seconds: 3600,
  udp_responses: 1,
  proxy_protocol: false,
  accept_proxy_protocol: false,
  trusted_proxies: [],
  tls_mode: "off",
  certificate_id: "",
  sni_routes: [],
  access_log: true,
  allow: [],
  deny: [],
  max_connections: 0,
});
const form = reactive<StreamRuleInput>(blank());
const routeNames = ref<string[]>([]);
function show(rule: StreamRule | null = null) {
  editing.value = rule;
  Object.assign(form, rule ? structuredClone(rule) : blank());
  trustedText.value = form.trusted_proxies.join("\n");
  allowText.value = form.allow.join("\n");
  denyText.value = form.deny.join("\n");
  routeNames.value = form.sni_routes.map((route) =>
    route.server_names.join(", "),
  );
  open.value = true;
}
function addRoute() {
  form.sni_routes.push(blankRoute());
  routeNames.value.push("");
}
function removeRoute(index: number) {
  form.sni_routes.splice(index, 1);
  routeNames.value.splice(index, 1);
}
function submit() {
  const value = structuredClone(form);
  value.trusted_proxies = trustedText.value.split(/[\s,]+/).filter(Boolean);
  value.allow = allowText.value.split(/[\s,]+/).filter(Boolean);
  value.deny = denyText.value.split(/[\s,]+/).filter(Boolean);
  value.sni_routes = value.sni_routes.map((route, index) => ({
    ...route,
    server_names: (routeNames.value[index] ?? "")
      .split(/[\s,]+/)
      .filter(Boolean),
  }));
  emit("save", value, editing.value?.id ?? "");
}
watch(
  () => props.busy,
  (value) => {
    if (
      !value &&
      open.value &&
      props.rules.some(
        (rule) =>
          rule.id === editing.value?.id ||
          (!editing.value && rule.name === form.name),
      )
    )
      open.value = false;
  },
);
</script>
<template>
  <div class="toolbar">
    <div class="notice">
      四层代理独立于 HTTP 规则，适用于 SSH、数据库、MQTT、游戏服务和 HTTPS SNI
      透传。
    </div>
    <span class="spacer"></span>
    <button class="button ghost" :disabled="busy" @click="emit('refresh')">
      刷新
    </button>
    <button
      class="button"
      :class="dirty ? 'primary' : 'secondary'"
      :disabled="busy"
      @click="emit('apply')"
    >
      {{ dirty ? "保存并应用" : "重新应用" }}
    </button>
    <button class="button primary" @click="show()">
      ＋ 添加 TCP/UDP 规则
    </button>
  </div>
  <article class="card">
    <div v-if="rules.length" class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>状态</th>
            <th>名称</th>
            <th>入口</th>
            <th>目标</th>
            <th>TLS / 能力</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="rule in rules" :key="rule.id">
            <td>
              <label class="switch"
                ><input
                  type="checkbox"
                  :checked="rule.enabled"
                  @change="
                    emit(
                      'toggle',
                      rule,
                      ($event.target as HTMLInputElement).checked,
                    )
                  " /><span></span
              ></label>
            </td>
            <td>
              <div class="rule-name">{{ rule.name }}</div>
              <div class="rule-sub">{{ rule.id }}</div>
            </td>
            <td>
              <span class="badge info">{{ rule.protocol.toUpperCase() }}</span>
              {{ rule.listen_address }}:{{ rule.listen_port }}
            </td>
            <td>
              {{
                rule.upstream_pool_id
                  ? "服务器池"
                  : `${rule.upstream_host}:${rule.upstream_port}`
              }}
            </td>
            <td>
              <div class="domain-list">
                <span class="badge neutral">{{ rule.tls_mode }}</span
                ><span v-if="rule.accept_proxy_protocol" class="badge neutral"
                  >接收 PROXY</span
                ><span v-if="rule.proxy_protocol" class="badge neutral"
                  >发送 PROXY</span
                ><span v-if="rule.sni_routes.length" class="badge neutral"
                  >SNI {{ rule.sni_routes.length }}</span
                >
              </div>
            </td>
            <td>
              <div class="table-actions">
                <button class="button ghost small" @click="show(rule)">
                  编辑</button
                ><button
                  class="button danger-ghost small"
                  @click="emit('remove', rule)"
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
      <div class="empty-icon">⇆</div>
      <h3>还没有 TCP/UDP 代理</h3>
      <p>创建独立监听端口并转发到单个目标服务或 Stream 目标服务池。</p>
      <button class="button primary" @click="show()">添加规则</button>
    </div>
  </article>
  <div v-if="open" class="modal-backdrop" @mousedown.self="open = false">
    <section
      class="modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="stream-title"
    >
      <header class="modal-header">
        <div>
          <h2 id="stream-title">
            {{ editing ? "编辑" : "添加" }} TCP/UDP 规则
          </h2>
          <p>支持 TLS 终止、SNI 透传、PROXY Protocol 和访问控制。</p>
        </div>
        <button class="icon-button" aria-label="关闭" @click="open = false">
          ×
        </button>
      </header>
      <div class="modal-body">
        <form class="form-grid modal-form-grid" @submit.prevent="submit">
          <div class="field">
            <label>名称</label
            ><input
              v-model.trim="form.name"
              class="input"
              required
              maxlength="80"
              autofocus
            />
          </div>
          <div class="field">
            <label>状态</label
            ><label class="checkbox-row"
              ><input v-model="form.enabled" type="checkbox" />启用规则</label
            >
          </div>
          <div class="form-section">监听入口</div>
          <div class="field">
            <label>协议</label
            ><select v-model="form.protocol" class="select">
              <option value="tcp">TCP</option>
              <option value="udp">UDP</option>
            </select>
          </div>
          <div class="field">
            <label>监听地址</label
            ><input
              v-model.trim="form.listen_address"
              class="input"
              required
              placeholder="0.0.0.0、:: 或指定 IP"
            />
          </div>
          <div class="field">
            <label>监听端口</label
            ><input
              v-model.number="form.listen_port"
              class="input"
              type="number"
              min="1024"
              max="65535"
              required
            />
          </div>
          <div class="field">
            <label>PROXY Protocol</label
            ><label class="checkbox-row"
              ><input
                v-model="form.accept_proxy_protocol"
                type="checkbox"
              />入口接收 PROXY Protocol</label
            ><label class="checkbox-row"
              ><input v-model="form.proxy_protocol" type="checkbox" />向目标服务发送
              PROXY Protocol</label
            >
          </div>
          <div class="field full">
            <label>可信代理 IP / CIDR</label
            ><textarea
              v-model="trustedText"
              class="textarea"
              placeholder="仅在接收 PROXY Protocol 时填写"
            ></textarea>
          </div>
          <div class="form-section">目标服务</div>
          <div class="field full">
            <label>Stream 目标服务池</label
            ><select v-model="form.upstream_pool_id" class="select">
              <option value="">单个目标</option>
              <option
                v-for="pool in pools.filter(
                  (item) => item.protocol === 'stream',
                )"
                :key="pool.id"
                :value="pool.id"
              >
                {{ pool.name }}
              </option>
            </select>
          </div>
          <template v-if="!form.upstream_pool_id"
            ><div class="field">
              <label>目标主机</label
              ><input
                v-model.trim="form.upstream_host"
                class="input"
                required
              />
            </div>
            <div class="field">
              <label>目标端口</label
              ><input
                v-model.number="form.upstream_port"
                class="input"
                type="number"
                min="1"
                max="65535"
                required
              /></div
          ></template>
          <div class="field">
            <label>连接超时（秒）</label
            ><input
              v-model.number="form.connect_timeout_seconds"
              class="input"
              type="number"
              min="1"
              max="600"
            />
          </div>
          <div class="field">
            <label>会话超时（秒）</label
            ><input
              v-model.number="form.proxy_timeout_seconds"
              class="input"
              type="number"
              min="1"
              max="86400"
            />
          </div>
          <div v-if="form.protocol === 'udp'" class="field">
            <label>UDP 响应次数</label
            ><input
              v-model.number="form.udp_responses"
              class="input"
              type="number"
              min="0"
              max="1000"
            />
          </div>
          <div class="form-section">TLS</div>
          <div class="field">
            <label>TLS 模式</label
            ><select
              v-model="form.tls_mode"
              class="select"
              :disabled="form.protocol === 'udp'"
            >
              <option value="off">关闭</option>
              <option value="terminate">TLS 终止</option>
              <option value="passthrough">SNI 透传</option>
            </select>
          </div>
          <div v-if="form.tls_mode === 'terminate'" class="field">
            <label>证书</label
            ><select v-model="form.certificate_id" class="select" required>
              <option value="">请选择</option>
              <option
                v-for="cert in certificates"
                :key="cert.id"
                :value="cert.id"
              >
                {{ cert.name }}
              </option>
            </select>
          </div>
          <template v-if="form.tls_mode === 'passthrough'"
            ><div class="form-section">SNI 分流</div>
            <div
              v-for="(route, index) in form.sni_routes"
              :key="index"
              class="full sni-editor"
            >
              <input
                v-model="routeNames[index]"
                class="input"
                required
                placeholder="域名，多个用逗号分隔"
              /><select v-model="route.upstream_pool_id" class="select">
                <option value="">单个目标</option>
                <option
                  v-for="pool in pools.filter(
                    (item) => item.protocol === 'stream',
                  )"
                  :key="pool.id"
                  :value="pool.id"
                >
                  {{ pool.name }}
                </option></select
              ><input
                v-if="!route.upstream_pool_id"
                v-model.trim="route.upstream_host"
                class="input"
                required
                placeholder="主机"
              /><input
                v-if="!route.upstream_pool_id"
                v-model.number="route.upstream_port"
                class="input"
                type="number"
                min="1"
                max="65535"
              /><button
                type="button"
                class="button danger-ghost small"
                @click="removeRoute(index)"
              >
                删除
              </button>
            </div>
            <div class="full">
              <button
                type="button"
                class="button ghost small"
                @click="addRoute"
              >
                ＋ 添加 SNI 路由
              </button>
            </div></template
          >
          <div class="form-section">安全与日志</div>
          <div class="field">
            <label>日志与连接限制</label
            ><label class="checkbox-row"
              ><input v-model="form.access_log" type="checkbox" />记录 Stream
              访问日志</label
            ><label>单 IP 最大连接数</label
            ><input
              v-model.number="form.max_connections"
              class="input"
              type="number"
              min="0"
              max="1000000"
            />
          </div>
          <div class="field">
            <label>允许 IP / CIDR</label
            ><textarea v-model="allowText" class="textarea"></textarea>
          </div>
          <div class="field">
            <label>拒绝 IP / CIDR</label
            ><textarea v-model="denyText" class="textarea"></textarea>
          </div>
          <footer class="modal-footer full">
            <button type="button" class="button ghost" @click="open = false">
              取消</button
            ><button type="submit" class="button primary" :disabled="busy">
              {{ busy ? "处理中…" : "保存 Stream 规则" }}
            </button>
          </footer>
        </form>
      </div>
    </section>
  </div>
</template>
