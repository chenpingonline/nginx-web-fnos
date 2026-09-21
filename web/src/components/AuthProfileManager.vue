<script setup lang="ts">
import { reactive, ref } from "vue";
import { PhEye, PhEyeSlash, PhPlusCircle, PhShieldCheck, PhTrash, PhX } from "@phosphor-icons/vue";
import { errorMessage, jsonBody, request } from "../api";
import type { AuthProfile, AuthProfileInput, State } from "../types";

const props = defineProps<{ profiles: AuthProfile[]; disabled?: boolean; prominent?: boolean; hideTrigger?: boolean }>();
const emit = defineEmits<{ changed: [profiles: AuthProfile[]] }>();
const open = ref(false);
const saving = ref(false);
const message = ref("");
const passwordVisible = ref<boolean[]>([]);
const form = reactive<AuthProfileInput>({ name: "", realm: "", users: [] });

function reset() {
  form.name = "";
  form.realm = "";
  form.users = [];
  passwordVisible.value = [];
  message.value = "";
}
function show() {
  open.value = true;
  reset();
}
function showNew() {
  open.value = true;
  reset();
}
defineExpose({ show, showNew });
function addUser() {
  form.users.push({ username: "", password: "", enabled: true });
  passwordVisible.value.push(false);
}
function removeUser(index: number) {
  form.users.splice(index, 1);
  passwordVisible.value.splice(index, 1);
}
async function refresh() {
  const state = await request<State>("/state");
  emit("changed", state.auth_profiles ?? []);
}
async function save() {
  saving.value = true;
  message.value = "";
  try {
    await request<AuthProfile>("/auth-profiles", {
      method: "POST",
      body: jsonBody(form),
    });
    await refresh();
    open.value = false;
  } catch (error) {
    message.value = errorMessage(error);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <button v-if="!hideTrigger" type="button" class="button compact auth-manager-trigger" :class="prominent ? 'primary' : 'ghost'" :disabled="disabled" @click="show"><PhShieldCheck :size="16" />管理认证策略</button>
  <Teleport to="body">
    <div v-if="open" class="auth-manager-backdrop" @mousedown.self="!saving && (open = false)">
      <section class="auth-manager" role="dialog" aria-modal="true" aria-labelledby="auth-manager-title">
        <header class="auth-manager-header">
          <div><h2 id="auth-manager-title">添加认证策略</h2><p>创建可供代理规则复用的用户名密码认证。</p></div>
          <button type="button" class="icon-button" aria-label="关闭" :disabled="saving" @click="open = false"><PhX :size="20" /></button>
        </header>
        <div class="auth-manager-body">
          <form class="auth-profile-form" @submit.prevent="save">
            <div class="auth-form-grid">
              <label class="field"><span>策略名称</span><input v-model.trim="form.name" class="input" required maxlength="60" placeholder="例如：家庭成员" /></label>
              <label class="field"><span>浏览器认证提示</span><input v-model.trim="form.realm" class="input" required maxlength="100" placeholder="例如：Restricted" /></label>
            </div>
            <div class="auth-users-title"><div><strong>认证用户</strong><p>密码至少 8 个字符，保存时仅存储 bcrypt 哈希。</p></div><button type="button" class="button ghost auth-action-button" @click="addUser"><PhPlusCircle :size="16" />添加用户</button></div>
            <div v-if="form.users.length" class="auth-users">
              <div v-for="(user, index) in form.users" :key="user.id || index" class="auth-user-row">
                <label class="auth-user-field-label" :for="`auth-username-${index}`">用户名</label>
                <input :id="`auth-username-${index}`" v-model.trim="user.username" class="input" required maxlength="64" placeholder="请输入用户名" autocomplete="username" />
                <label class="auth-user-field-label" :for="`auth-password-${index}`">密码</label>
                <div class="auth-password-field">
                  <input :id="`auth-password-${index}`" v-model="user.password" class="input" required minlength="8" maxlength="128" :type="passwordVisible[index] ? 'text' : 'password'" placeholder="至少 8 个字符" autocomplete="new-password" />
                  <button type="button" class="auth-password-toggle" :aria-label="passwordVisible[index] ? '隐藏密码' : '显示密码'" :aria-pressed="passwordVisible[index]" @click="passwordVisible[index] = !passwordVisible[index]">
                    <PhEyeSlash v-if="passwordVisible[index]" :size="18" aria-hidden="true" />
                    <PhEye v-else :size="18" aria-hidden="true" />
                  </button>
                </div>
                <label class="checkbox-row"><input v-model="user.enabled" type="checkbox" />启用</label>
                <button type="button" class="icon-button danger-icon" aria-label="删除用户" @click="removeUser(index)"><PhTrash :size="18" /></button>
              </div>
            </div>
            <div v-else class="auth-empty">尚未添加用户。启用认证的策略至少需要一个已启用用户。</div>
            <p v-if="message" class="auth-message">{{ message }}</p>
            <footer class="auth-manager-footer">
              <span></span>
              <button type="button" class="button ghost auth-footer-button" :disabled="saving" @click="open = false">关闭</button>
              <button type="submit" class="button primary auth-footer-button" :disabled="saving">{{ saving ? "保存中…" : "保存策略" }}</button>
            </footer>
          </form>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.auth-manager-trigger { justify-content: center; white-space: nowrap; }
.auth-manager-backdrop { position: fixed; inset: 0; z-index: 1200; display: grid; place-items: center; padding: 28px; background: rgba(11, 21, 17, .58); }
.auth-manager { width: min(920px, 100%); max-height: min(760px, calc(100vh - 56px)); overflow: hidden; border: 1px solid var(--line); border-radius: 18px; background: var(--surface); box-shadow: 0 24px 70px rgba(0,0,0,.24); }
.auth-manager-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 14px 18px; border-bottom: 1px solid var(--line); }
.auth-manager-header > div { min-width: 0; display: flex; align-items: baseline; gap: 14px; }
.auth-manager-header h2 { flex: 0 0 auto; margin: 0; font-size: 20px; }.auth-manager-header p { margin: 0; }.auth-manager-header p,.auth-users-title p { color: var(--muted); font-size: 13px; }
.auth-manager-header .icon-button { width: 32px; height: 32px; }
.auth-manager-body { min-height: 430px; max-height: calc(100vh - 160px); overflow: hidden; }
.auth-profile-list { padding: 12px 10px; overflow: auto; border-right: 1px solid var(--line); background: var(--surface-soft); }.auth-new { width: 100%; justify-content: center; margin-bottom: 8px; }
.auth-action-button { min-height: 34px; height: 34px; padding-inline: 11px; font-size: 13px; }
.auth-profile-item { width: 100%; min-height: 38px; display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 7px 10px; border: 1px solid transparent; border-radius: 8px; background: transparent; color: var(--text); text-align: left; cursor: pointer; }.auth-profile-item + .auth-profile-item { margin-top: 4px; }.auth-profile-item strong { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.auth-profile-item span { flex: 0 0 auto; color: var(--muted); font-size: 12px; white-space: nowrap; }.auth-profile-item.active { border-color: color-mix(in srgb, var(--accent) 40%, var(--line)); background: color-mix(in srgb, var(--accent) 10%, var(--surface)); }
.auth-profile-form { min-width: 0; padding: 22px; overflow: auto; }.auth-form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }.field { display: grid; gap: 7px; }.field > span { font-weight: 620; }
.auth-users-title { display: flex; justify-content: space-between; align-items: center; margin: 24px 0 10px; }.auth-user-row { display: grid; grid-template-columns: auto minmax(120px,.8fr) auto minmax(180px,1.2fr) auto auto; align-items: center; gap: 10px; margin-bottom: 10px; }.auth-user-field-label { font-size: 14px; font-weight: 620; white-space: nowrap; }.auth-password-field { position: relative; min-width: 0; }.auth-password-field .input { width: 100%; padding-right: 40px; }.auth-password-toggle { position: absolute; top: 2px; right: 2px; width: 30px; height: 30px; padding: 0; display: grid; place-items: center; border: 0; border-radius: 7px; background: transparent; color: var(--muted); cursor: pointer; }.auth-password-toggle:hover,.auth-password-toggle:focus-visible { background: var(--surface-soft); color: var(--text); outline: none; }.danger-icon { color: var(--danger); }.auth-empty { padding: 24px; border: 1px dashed var(--line); border-radius: 10px; color: var(--muted); text-align: center; }.auth-message { color: var(--muted); font-size: 13px; }
.auth-manager-footer { display: flex; gap: 10px; align-items: center; margin-top: 20px; padding-top: 14px; border-top: 1px solid var(--line); }
.auth-manager-footer > span { flex: 1 1 auto; }
.auth-footer-button { width: 86px; min-height: 34px; height: 34px; padding-inline: 10px; font-size: 13px; }
@media (max-width: 720px) { .auth-manager-header > div { display: grid; gap: 3px; }.auth-manager-body { overflow: auto; }.auth-form-grid { grid-template-columns: 1fr; }.auth-user-row { grid-template-columns: auto minmax(0, 1fr) auto; }.auth-password-field { grid-column: 2 / -1; }.auth-user-row > .checkbox-row { grid-column: 2; } }
</style>
