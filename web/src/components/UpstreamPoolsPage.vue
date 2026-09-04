<script setup lang="ts">
import { reactive, ref, toRaw, watch } from "vue";
import {
  PhArrowClockwise,
  PhCheckCircle,
  PhPlusCircle,
} from "@phosphor-icons/vue";
import type { UpstreamPool, UpstreamPoolInput, UpstreamServer } from "../types";
const props = defineProps<{
  pools: UpstreamPool[];
  busy: boolean;
  dirty: boolean;
}>();
const emit = defineEmits<{
  save: [value: UpstreamPoolInput, id: string];
  remove: [pool: UpstreamPool];
  refresh: [];
  apply: [];
}>();
const editing = ref<UpstreamPool | null>(null),
  open = ref(false);
const blankServer = (): UpstreamServer => ({
  host: "127.0.0.1",
  port: 8080,
  weight: 1,
  max_fails: 1,
  fail_timeout_seconds: 10,
  backup: false,
  down: false,
});
const form = reactive<UpstreamPoolInput>({
  name: "",
  protocol: "http",
  strategy: "round_robin",
  hash_key: "$request_uri",
  keepalive: 32,
  keepalive_requests: 1000,
  keepalive_time_seconds: 3600,
  keepalive_timeout_seconds: 60,
  servers: [blankServer()],
});
function show(pool: UpstreamPool | null = null) {
  editing.value = pool;
  Object.assign(
    form,
    pool
      ? structuredClone(pool)
      : {
          name: "",
          protocol: "http",
          strategy: "round_robin",
          hash_key: "$request_uri",
          keepalive: 32,
          keepalive_requests: 1000,
          keepalive_time_seconds: 3600,
          keepalive_timeout_seconds: 60,
          servers: [blankServer()],
        },
  );
  open.value = true;
}
function addServer() {
  form.servers.push(blankServer());
}
function removeServer(index: number) {
  if (form.servers.length > 1) form.servers.splice(index, 1);
}
function submit() {
  emit("save", structuredClone(toRaw(form)), editing.value?.id ?? "");
}
watch(
  () => props.pools,
  () => {
    if (
      open.value &&
      props.pools.some(
        (pool) =>
          pool.id === editing.value?.id ||
          (!editing.value && pool.name === form.name),
      )
    )
      open.value = false;
  },
);
</script>
<template>
  <div class="toolbar">
    <div class="notice">
      服务器池可以被多个 HTTP 或 TCP/UDP
      规则复用，并统一配置负载均衡与故障恢复。
    </div>
    <span class="spacer"></span>
    <button class="button ghost" :disabled="busy" @click="emit('refresh')">
      <PhArrowClockwise :size="16" aria-hidden="true" />刷新
    </button>
    <button
      class="button"
      :class="dirty ? 'primary' : 'secondary'"
      :disabled="busy"
      @click="emit('apply')"
    >
      <PhCheckCircle :size="16" aria-hidden="true" />{{
        dirty ? "保存并应用" : "重新应用"
      }}
    </button>
    <button class="button primary" @click="show()">
      <PhPlusCircle :size="17" aria-hidden="true" />添加后端服务池
    </button>
  </div>
  <article class="card">
    <div v-if="pools.length" class="table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>名称</th>
            <th>协议</th>
            <th>算法</th>
            <th>节点</th>
            <th>连接池</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="pool in pools" :key="pool.id">
            <td>
              <div class="rule-name">{{ pool.name }}</div>
              <div class="rule-sub">{{ pool.id }}</div>
            </td>
            <td>
              <span class="badge info">{{ pool.protocol.toUpperCase() }}</span>
            </td>
            <td>{{ pool.strategy }}</td>
            <td>
              <div
                v-for="server in pool.servers"
                :key="`${server.host}:${server.port}`"
                class="rule-sub"
              >
                {{ server.host }}:{{ server.port }} · 权重 {{ server.weight
                }}<template v-if="server.backup"> · 备份</template
                ><template v-if="server.down"> · 停用</template>
              </div>
            </td>
            <td>{{ pool.keepalive || "关闭" }}</td>
            <td>
              <div class="table-actions">
                <button class="button ghost small" @click="show(pool)">
                  编辑</button
                ><button
                  class="button danger-ghost small"
                  @click="emit('remove', pool)"
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
      <div class="empty-icon">⇶</div>
      <h3>还没有后端服务池</h3>
      <p>单节点规则可以继续直接填写主机和端口；多节点服务建议创建服务器池。</p>
      <button class="button primary" @click="show()">
        <PhPlusCircle :size="17" aria-hidden="true" />添加后端服务池
      </button>
    </div>
  </article>
  <div v-if="open" class="modal-backdrop" @mousedown.self="open = false">
    <section
      class="modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="pool-title"
    >
      <header class="modal-header">
        <div>
          <h2 id="pool-title">{{ editing ? "编辑" : "添加" }}后端服务池</h2>
          <p>结构化配置负载均衡、节点权重与连接复用。</p>
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
            <label>协议用途</label
            ><select v-model="form.protocol" class="select">
              <option value="http">HTTP/HTTPS</option>
              <option value="stream">TCP/UDP</option>
            </select>
          </div>
          <div class="field">
            <label>负载均衡算法</label
            ><select v-model="form.strategy" class="select">
              <option value="round_robin">轮询（Round Robin）</option>
              <option value="least_conn">最少连接（Least Connections）</option>
              <option value="ip_hash">IP 哈希（IP Hash）</option>
              <option value="hash">哈希（Hash）</option>
              <option value="random">
                随机二选一最少连接（Random Two Least Conn）
              </option>
            </select
            ><span class="field-help">权重在下方每个服务器节点中单独设置。</span>
          </div>
          <div v-if="form.strategy === 'hash'" class="field">
            <label>Hash Key</label
            ><select v-model="form.hash_key" class="select">
              <option value="$request_uri">请求 URI</option>
              <option value="$remote_addr">客户端 IP</option>
              <option value="$host">Host</option>
            </select>
          </div>
          <div class="form-section">服务器节点</div>
          <div class="full server-editor-head" aria-hidden="true">
            <span>主机 / IP</span>
            <span>端口</span>
            <span>权重</span>
            <span>最大失败次数</span>
            <span>恢复时间（秒）</span>
            <span>备份</span>
            <span>停用</span>
            <span>操作</span>
          </div>
          <div
            v-for="(server, index) in form.servers"
            :key="index"
            class="full server-editor"
          >
            <input
              v-model.trim="server.host"
              class="input"
              required
              placeholder="主机/IP"
            /><input
              v-model.number="server.port"
              class="input"
              type="number"
              min="1"
              max="65535"
              required
              placeholder="端口"
            /><input
              v-model.number="server.weight"
              class="input"
              type="number"
              min="1"
              max="1000"
              aria-label="权重"
              title="权重"
            /><input
              v-model.number="server.max_fails"
              class="input"
              type="number"
              min="0"
              max="100"
              aria-label="最大失败次数"
              title="最大失败次数"
            /><input
              v-model.number="server.fail_timeout_seconds"
              class="input"
              type="number"
              min="1"
              max="86400"
              aria-label="故障恢复时间（秒）"
              title="故障恢复秒数"
            /><label class="checkbox-row"
              ><input v-model="server.backup" type="checkbox" />备份</label
            ><label class="checkbox-row"
              ><input v-model="server.down" type="checkbox" />停用</label
            ><button
              type="button"
              class="button danger-ghost small"
              :disabled="form.servers.length === 1"
              @click="removeServer(index)"
            >
              删除
            </button>
          </div>
          <div class="full">
            <button type="button" class="button ghost small" @click="addServer">
              ＋ 添加节点
            </button>
          </div>
          <div class="form-section">连接池</div>
          <div class="field">
            <label>Keepalive 数量</label
            ><input
              v-model.number="form.keepalive"
              class="input"
              type="number"
              min="0"
              max="4096"
            />
          </div>
          <div class="field">
            <label>单连接最大请求数</label
            ><input
              v-model.number="form.keepalive_requests"
              class="input"
              type="number"
              min="1"
              max="100000"
            />
          </div>
          <div class="field">
            <label>单连接最长时间（秒）</label
            ><input
              v-model.number="form.keepalive_time_seconds"
              class="input"
              type="number"
              min="1"
              max="86400"
            />
          </div>
          <div class="field">
            <label>空闲超时（秒）</label
            ><input
              v-model.number="form.keepalive_timeout_seconds"
              class="input"
              type="number"
              min="1"
              max="3600"
            />
          </div>
          <footer class="modal-footer full">
            <button type="button" class="button ghost" @click="open = false">
              取消</button
            ><button type="submit" class="button primary" :disabled="busy">
              {{ busy ? "处理中…" : "保存后端服务池" }}
            </button>
          </footer>
        </form>
      </div>
    </section>
  </div>
</template>
