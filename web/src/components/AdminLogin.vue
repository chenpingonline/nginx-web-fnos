<script setup lang="ts">
import { ref } from "vue";
import { PhShieldCheck } from "@phosphor-icons/vue";
import { errorMessage, jsonBody, request } from "../api";
import type { AdminSession } from "../Root.vue";

defineProps<{ notice?: string }>();
const emit = defineEmits<{ authenticated: [session: AdminSession] }>();
const username = ref("admin");
const password = ref("");
const busy = ref(false);
const error = ref("");
const version = __APP_VERSION__;
async function login() {
  if (busy.value) return;
  busy.value = true;
  error.value = "";
  try {
    const session = await request<AdminSession>("/auth/login", {
      method: "POST", body: jsonBody({ username: username.value, password: password.value }),
    });
    password.value = "";
    emit("authenticated", session);
  } catch (reason) {
    error.value = errorMessage(reason);
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <main class="auth-shell">
    <section class="card admin-login" aria-labelledby="login-title">
      <div class="login-mark"><PhShieldCheck :size="30" aria-hidden="true" /></div>
      <h1 id="login-title">nginx-web</h1>
      <p class="login-subtitle">登录以管理代理、证书和 Nginx 配置</p>
      <form @submit.prevent="login">
        <div class="field"><label for="admin-user">管理员账号</label><input id="admin-user" v-model="username" class="input" autocomplete="username" maxlength="64" required :disabled="busy" autofocus /></div>
        <div class="field"><label for="admin-password">密码</label><input id="admin-password" v-model="password" class="input" type="password" autocomplete="current-password" required :disabled="busy" /></div>
        <p v-if="error || notice" class="login-feedback" :class="{ 'login-error': error }" role="alert">{{ error || notice }}</p>
        <button type="submit" class="button primary login-submit" :disabled="busy">{{ busy ? '正在登录…' : '登录' }}</button>
      </form>
      <p class="login-version">Docker 版 · v{{ version }}</p>
    </section>
  </main>
</template>

<style scoped>
.admin-login { width: min(100%, 410px); padding: 32px; }
.login-mark { width: 56px; height: 56px; display: grid; place-items: center; color: var(--accent-dark); background: var(--accent-soft); border-radius: 16px; margin-bottom: 20px; }
.admin-login h1 { margin: 0 0 8px; font-size: 27px; letter-spacing: -.7px; }
.login-subtitle { margin: 0 0 26px; color: var(--text-muted); font-size: 14px; line-height: 1.6; }
.admin-login form { display: grid; gap: 18px; }
.admin-login .input { width: 100%; }
.login-submit { width: 100%; margin-top: 4px; }
.login-feedback { margin: 0; font-size: 13px; color: var(--text-muted); line-height: 1.6; overflow-wrap: anywhere; }
.login-error { color: var(--danger); }
.login-version { margin: 24px 0 0; font-size: 12px; color: var(--text-muted); text-align: center; }
@media (max-width: 480px) { .auth-shell { padding: 16px; } .admin-login { padding: 26px 22px; } }
</style>
