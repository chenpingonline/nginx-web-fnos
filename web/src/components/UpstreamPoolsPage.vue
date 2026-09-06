<script setup lang="ts">
import PoolPathExample from "./PoolPathExample.vue";
import AppSelect from "./AppSelect.vue";
import { reactive, ref, toRaw } from "vue";
import {
  PhArrowClockwise,
  PhPlusCircle,
} from "@phosphor-icons/vue";
import type { UpstreamPool, UpstreamPoolInput, UpstreamServer } from "../types";
const props = defineProps<{
  pools: UpstreamPool[];
  busy: boolean;
}>();
const emit = defineEmits<{
  save: [value: UpstreamPoolInput, id: string, done: (saved?: UpstreamPool, error?: string) => void];
  remove: [pool: UpstreamPool];
  refresh: [];
}>();
const helpOpen = ref(false);
const helpPosition = ref<Record<string, string>>({});
function toggleHelp(event: MouseEvent) {
  if (helpOpen.value) { helpOpen.value = false; return; }
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const width = Math.min(500, window.innerWidth - 32);
  const fitsRight = rect.right + 8 + width <= window.innerWidth - 16;
  helpPosition.value = {
    width: `${width}px`,
    left: `${fitsRight ? rect.right + 8 : Math.max(16, Math.min(rect.left, window.innerWidth - width - 16))}px`,
    top: `${Math.max(16, Math.min(fitsRight ? rect.top : rect.bottom, window.innerHeight - width * 150 / 640 - 36))}px`,
  };
  helpOpen.value = true;
}
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
  saveError.value = "";
  editing.value = pool;
  Object.assign(
    form,
    pool
      ? structuredClone(toRaw(pool))
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
  if (props.busy) return;
  saveError.value = "";
  emit("save", structuredClone(toRaw(form)), editing.value?.id ?? "", savedResult);
}
const saveError = ref("");
function savedResult(saved?: UpstreamPool, error?: string) {
  if (saved) editing.value = saved;
  saveError.value = error ?? "";
  if (saved && !error) open.value = false;
}
</script>
<template>
  <div class="toolbar">
    <div class="notice pool-description">
      后端服务组可以被多个 HTTP 或 TCP/UDP
      规则复用，并统一配置负载均衡与故障恢复。
      <span class="pool-help" @mouseleave="helpOpen = false" @focusout="helpOpen = false" @keydown.esc="helpOpen = false">
        <button type="button" class="pool-help-button" aria-label="查看服务组访问路径示例" :aria-expanded="helpOpen" aria-controls="pool-help-content" @click="toggleHelp">?</button>
        <div v-if="helpOpen" id="pool-help-content" class="pool-help-panel" :style="helpPosition"><PoolPathExample /></div>
      </span>
    </div>
    <span class="spacer"></span>
    <button class="button ghost" :disabled="busy" @click="emit('refresh')">
      <PhArrowClockwise :size="16" aria-hidden="true" />刷新
    </button>
    <button class="button primary" @click="show()">
      <PhPlusCircle :size="17" aria-hidden="true" />添加后端服务组
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
    <div v-else class="empty-state pool-empty">
      <h3>还没有后端服务组</h3>
      <PoolPathExample class="pool-empty-example" />
    </div>
  </article>
  <Teleport to="body">
  <div v-if="open" class="modal-backdrop" @mousedown.self="!busy && (open = false)">
    <section
      class="modal pool-modal"
      role="dialog"
      aria-modal="true"
      aria-labelledby="pool-title"
    >
      <header class="modal-header">
        <div>
          <h2 id="pool-title">{{ editing ? "编辑" : "添加" }}后端服务组</h2>
          <p>保存后立即应用，其他尚未应用的配置修改也会一并生效。</p>
        </div>
        <div class="rule-header-actions">
          <button type="button" class="button ghost" :disabled="busy" @click="open = false">取消</button>
          <button type="submit" form="pool-form" class="button primary" :disabled="busy">
            {{ busy ? "保存并应用中…" : "保存并应用" }}
          </button>
        </div>
        <button class="icon-button modal-close" aria-label="关闭" :disabled="busy" @click="open = false">
          ×
        </button>
      </header>
      <div class="modal-body">
        <div v-if="saveError" class="notice danger" role="alert">{{ saveError }}</div>
        <form id="pool-form" class="form-grid modal-form-grid" @submit.prevent="submit">
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
            ><AppSelect v-model="form.protocol" class="select">
              <option value="http">HTTP/HTTPS</option>
              <option value="stream">TCP/UDP</option>
            </AppSelect>
          </div>
          <div class="field">
            <label>负载均衡算法</label
            ><AppSelect v-model="form.strategy" class="select">
              <option value="round_robin">轮询（Round Robin）</option>
              <option value="least_conn">最少连接（Least Connections）</option>
              <option value="ip_hash">IP 哈希（IP Hash）</option>
              <option value="hash">哈希（Hash）</option>
              <option value="random">
                随机二选一最少连接（Random Two Least Conn）
              </option>
            </AppSelect
            ><span class="field-help">权重在下方每个服务器节点中单独设置。</span>
          </div>
          <div v-if="form.strategy === 'hash'" class="field">
            <label>Hash Key</label
            ><AppSelect v-model="form.hash_key" class="select">
              <option value="$request_uri">请求 URI</option>
              <option value="$remote_addr">客户端 IP</option>
              <option value="$host">Host</option>
            </AppSelect>
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

        </form>
      </div>
    </section>
  </div>
  </Teleport>
</template>

<style scoped>
.pool-description { position: relative; }
.pool-help { position: relative; display: inline-flex; vertical-align: middle; margin-left: 5px; }
.pool-help::after { content: ""; position: absolute; left: 100%; top: 0; width: 10px; height: 100%; }
.pool-help-button { width: 18px; height: 18px; border: 1px solid var(--text-muted); border-radius: 50%; color: var(--text-muted); background: transparent; font-size: 14px; padding: 0; line-height: 16px; }
.pool-help-button:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.pool-help-panel { position: fixed; z-index: 50; padding: 9px; width: 500px; background: var(--surface-solid); border: 1px solid var(--line-strong); border-radius: 10px; box-shadow: 0 8px 24px rgb(0 0 0 / 14%); }
.pool-help-panel :deep(.example-path) { min-width: 0; }
.pool-empty { padding: 36px 24px; }
.pool-empty h3 { color: var(--text-muted); font-weight: 500; }
.pool-empty-example { max-width: 640px; margin: 16px auto 20px; }
.pool-help-panel { max-height: calc(100vh - 32px); overflow-y: auto; }
</style>
