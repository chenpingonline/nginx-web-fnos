<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, useId } from "vue";
import { PhQuestion } from "@phosphor-icons/vue";

const props = defineProps<{ text?: string; label?: string; wide?: boolean }>();
const open = ref(false);
const trigger = ref<HTMLButtonElement>();
const panel = ref<HTMLElement>();
const position = ref<Record<string, string>>({ visibility: "hidden" });
const id = `help-hint-${useId()}`;
let closeTimer: number | undefined;

function cancelClose() {
  if (closeTimer) window.clearTimeout(closeTimer);
  closeTimer = undefined;
}

function close() {
  cancelClose();
  open.value = false;
  document.removeEventListener("pointerdown", outside, true);
  document.removeEventListener("scroll", dismissOnScroll, true);
  window.removeEventListener("resize", close);
}

function scheduleClose() {
  cancelClose();
  closeTimer = window.setTimeout(close, 160);
}

function outside(event: PointerEvent) {
  const target = event.target as Node;
  if (!trigger.value?.contains(target) && !panel.value?.contains(target)) close();
}

function dismissOnScroll(event: Event) {
  if (!panel.value?.contains(event.target as Node)) close();
}

async function show() {
  position.value = { visibility: "hidden" };
  open.value = true;
  document.addEventListener("pointerdown", outside, true);
  document.addEventListener("scroll", dismissOnScroll, true);
  window.addEventListener("resize", close);
  await nextTick();
  if (!open.value || !trigger.value || !panel.value) return;

  const margin = 8;
  const gap = 8;
  const anchor = trigger.value.getBoundingClientRect();
  const tooltip = panel.value.getBoundingClientRect();
  const spaceBelow = window.innerHeight - anchor.bottom - margin;
  const placeBelow = spaceBelow >= tooltip.height + gap || anchor.top < tooltip.height + gap + margin;
  const idealTop = placeBelow ? anchor.bottom + gap : anchor.top - tooltip.height - gap;
  const idealLeft = anchor.left + anchor.width / 2 - tooltip.width / 2;
  position.value = {
    top: `${Math.max(margin, Math.min(idealTop, window.innerHeight - tooltip.height - margin))}px`,
    left: `${Math.max(margin, Math.min(idealLeft, window.innerWidth - tooltip.width - margin))}px`,
  };
}

function toggle() {
  if (open.value) close();
  else void show();
}

onBeforeUnmount(close);
</script>

<template>
  <span class="help-hint-wrap" @mouseenter="cancelClose" @mouseleave="scheduleClose">
    <button
      ref="trigger"
      type="button"
      class="help-hint"
      :aria-label="label || text || '查看说明'"
      :aria-expanded="open"
      :aria-describedby="open ? id : undefined"
      @click.stop="toggle"
      @keydown.esc.stop="close"
    >
      <PhQuestion :size="12" weight="bold" aria-hidden="true" />
    </button>
  </span>
  <Teleport to="body">
    <div
      v-if="open"
      :id="id"
      ref="panel"
      class="help-hint-popover"
      :class="{ wide }"
      :style="position"
      role="tooltip"
      @mouseenter="cancelClose"
      @mouseleave="scheduleClose"
    >
      <slot>{{ text }}</slot>
    </div>
  </Teleport>
</template>

<style scoped>
.help-hint-wrap { display: inline-flex; vertical-align: middle; }
.help-hint { width: 16px; height: 16px; min-width: 16px; padding: 0; display: inline-grid; place-items: center; border: 1px solid var(--muted); border-radius: 50%; background: transparent; color: var(--muted); cursor: pointer; }
.help-hint:hover,.help-hint[aria-expanded="true"],.help-hint:focus-visible { border-color: var(--accent); color: var(--accent); outline: none; }
.help-hint:focus-visible { box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 18%, transparent); }
.help-hint-popover { position: fixed; z-index: 10000; width: max-content; max-width: min(320px, calc(100vw - 16px)); max-height: calc(100vh - 16px); overflow: auto; padding: 8px 10px; border: 1px solid var(--line-strong); border-radius: 8px; background: var(--surface-solid); color: var(--text); box-shadow: 0 10px 28px rgb(0 0 0 / 20%); font-size: 12px; font-weight: 400; line-height: 1.5; text-align: left; white-space: normal; overflow-wrap: anywhere; }
.help-hint-popover.wide { width: min(500px, calc(100vw - 16px)); }
.help-hint-popover :deep(.example-path) { min-width: 0; }
</style>
