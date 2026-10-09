<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import App from "./App.vue";
import AdminLogin from "./components/AdminLogin.vue";
import { errorMessage, request, setCSRFToken } from "./api";

export interface AdminSession {
  mode: "fnos" | "standalone";
  authenticated: boolean;
  username?: string;
  csrf_token?: string;
}
const session = ref<AdminSession | null>(null);
const loading = ref(true);
const error = ref("");
const loginNotice = ref("");
const loggingOut = ref(false);
let disposed = false;

function acceptSession(value: AdminSession) {
  session.value = value;
  setCSRFToken(value.csrf_token ?? "");
  loginNotice.value = "";
}
async function loadSession() {
  loading.value = true;
  error.value = "";
  try {
    const value = await request<AdminSession>("/auth/session");
    if (!disposed) acceptSession(value);
  } catch (reason) {
    if (!disposed) error.value = errorMessage(reason);
  } finally {
    if (!disposed) loading.value = false;
  }
}
function sessionExpired() {
  if (session.value?.mode !== "standalone") return;
  session.value = { mode: "standalone", authenticated: false };
  setCSRFToken("");
  loginNotice.value = "会话已结束，请重新登录。";
}
async function logout() {
  if (loggingOut.value) return;
  loggingOut.value = true;
  try {
    await request("/auth/logout", { method: "POST" });
    acceptSession({ mode: "standalone", authenticated: false });
  } catch (reason) {
    window.alert(errorMessage(reason));
  } finally {
    loggingOut.value = false;
  }
}
onMounted(() => {
  window.addEventListener("nginx-web:session-expired", sessionExpired);
  void loadSession();
});
onBeforeUnmount(() => {
  disposed = true;
  window.removeEventListener("nginx-web:session-expired", sessionExpired);
});
</script>

<template>
  <main v-if="loading || error" class="auth-shell">
    <section class="card auth-status" aria-live="polite">
      <template v-if="loading"><div class="spinner"></div><p>正在连接管理服务…</p></template>
      <template v-else><h1>无法连接管理服务</h1><p role="alert">{{ error }}</p><button class="button primary" @click="loadSession">重新连接</button></template>
    </section>
  </main>
  <AdminLogin v-else-if="session?.mode === 'standalone' && !session.authenticated" :notice="loginNotice" @authenticated="acceptSession" />
  <App v-else :admin-username="session?.mode === 'standalone' ? session.username : undefined" :logging-out="loggingOut" @logout="logout" />
</template>

<style>
.auth-shell { min-height: 100dvh; display: grid; place-items: center; padding: 24px; background: var(--frame); }
.auth-status { width: min(100%, 420px); padding: 36px; text-align: center; }
.auth-status h1 { font-size: 22px; }
.auth-status p { color: var(--text-muted); overflow-wrap: anywhere; }
.auth-status .spinner { margin: 0 auto; }
</style>
