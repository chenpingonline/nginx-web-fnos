<script setup lang="ts" generic="T extends string | number">
import { ref, useSlots, useId, nextTick, onBeforeUnmount, type VNode } from 'vue';
defineOptions({ inheritAttrs: false });
const props = defineProps<{ modelValue: T; disabled?: boolean; hideCheck?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: T]; change: [] }>();
const slots = useSlots();
const id = useId();
const trigger = ref<HTMLButtonElement>();
const menu = ref<HTMLElement>();
const open = ref(false);
const active = ref(-1);
const placement = ref<Record<string, string>>({});
type Item = { value: T; label: string; disabled: boolean };
function items(): Item[] {
  const result: Item[] = [];
  const label = (nodes: unknown): string => Array.isArray(nodes) ? nodes.map(label).join('') : typeof nodes === 'object' && nodes ? label((nodes as VNode).children) : typeof nodes === 'string' || typeof nodes === 'number' ? String(nodes) : '';
  const visit = (nodes: VNode[]) => nodes.forEach(node => {
    if (node.type === 'option') result.push({ value: (node.props?.value ?? label(node.children)) as T, label: label(node.children).trim(), disabled: node.props?.disabled === '' || node.props?.disabled === true });
    else if (Array.isArray(node.children)) visit(node.children as VNode[]);
  });
  visit(slots.default?.() ?? []);
  return result;
}
function close(restore = false) {
  open.value = false;
  document.removeEventListener('pointerdown', outside, true);
  window.removeEventListener('resize', dismiss);
  document.removeEventListener('scroll', scroll, true);
  if (restore) trigger.value?.focus();
}
function dismiss() { close(); }
function scroll(event: Event) { if (!menu.value?.contains(event.target as Node)) close(); }
function outside(event: Event) { if (!trigger.value?.contains(event.target as Node) && !menu.value?.contains(event.target as Node)) close(); }
async function show() {
  if (props.disabled || open.value || !trigger.value) return;
  const list = items();
  active.value = list.findIndex(item => item.value === props.modelValue && !item.disabled);
  if (active.value < 0) active.value = list.findIndex(item => !item.disabled);
  const r = trigger.value.getBoundingClientRect();
  const below = window.innerHeight - r.bottom - 12;
  const above = r.top - 12;
  const up = below < 180 && above > below;
  const width = Math.min(r.width, window.innerWidth - 16);
  placement.value = { position: 'fixed', width: 'max-content', minWidth: `${width}px`, maxWidth: `${window.innerWidth - 16}px`, left: `${Math.max(8, Math.min(r.left, window.innerWidth - width - 8))}px`, maxHeight: `${Math.max(40, Math.min(280, up ? above : below))}px`, ...(up ? { bottom: `${window.innerHeight - r.top + 5}px` } : { top: `${r.bottom + 5}px` }) };
  open.value = true;
  document.addEventListener('pointerdown', outside, true);
  window.addEventListener('resize', dismiss);
  document.addEventListener('scroll', scroll, true);
  await nextTick();
  if (!open.value || !menu.value) return;
  // Intrinsic content may widen the panel; keep the final bounds in the viewport.
  const panelWidth = menu.value.getBoundingClientRect().width;
  placement.value.left = `${Math.max(8, Math.min(r.left, window.innerWidth - panelWidth - 8))}px`;
  reveal();
}
function reveal() { menu.value?.querySelector(`[data-index="${active.value}"]`)?.scrollIntoView({ block: 'nearest' }); }
function choose(index: number) {
  const item = items()[index];
  if (!item || item.disabled) return;
  emit('update:modelValue', item.value);
  emit('change');
  close(true);
}
let search = ''; let searchTimer: ReturnType<typeof setTimeout>;
async function key(event: KeyboardEvent) {
  if (event.key === 'Tab') { close(); return; }
  if (event.key === 'Escape') { if (open.value) { event.preventDefault(); event.stopPropagation(); close(true); } return; }
  if (['ArrowDown', 'ArrowUp', 'Home', 'End', 'Enter', ' '].includes(event.key)) {
    event.preventDefault();
    if (!open.value) { await show(); return; }
    if (event.key === 'Enter' || event.key === ' ') { choose(active.value); return; }
    const list = items();
    const enabled = list.map((item, index) => item.disabled ? -1 : index).filter(index => index >= 0);
    if (!enabled.length) return;
    const index = enabled.indexOf(active.value);
    active.value = event.key === 'Home' ? enabled[0] : event.key === 'End' ? enabled[enabled.length - 1] : enabled[(index + (event.key === 'ArrowUp' ? -1 : 1) + enabled.length) % enabled.length];
    await nextTick(); reveal();
  } else if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
    await show(); search += event.key.toLocaleLowerCase(); clearTimeout(searchTimer);
    searchTimer = setTimeout(() => { search = ''; }, 600);
    const index = items().findIndex(item => !item.disabled && item.label.toLocaleLowerCase().startsWith(search));
    if (index >= 0) { active.value = index; await nextTick(); reveal(); }
  }
}
onBeforeUnmount(() => { close(); clearTimeout(searchTimer); });
</script>
<template>
  <button v-bind="$attrs" ref="trigger" type="button" class="app-select" role="combobox" aria-haspopup="listbox" :aria-expanded="open" :aria-controls="open ? id : undefined" :aria-activedescendant="open && active >= 0 ? `${id}-${active}` : undefined" :disabled="disabled" @click="open ? close() : show()" @keydown="key">
    {{ items().find(item => item.value === modelValue)?.label ?? '请选择' }}
  <Teleport to="body">
    <div v-if="open" :id="id" ref="menu" class="app-select-menu" role="listbox" :aria-label="($attrs['aria-label'] as string) || '选项'" :style="placement" @mousedown.prevent>
      <div v-for="(item, index) in items()" :id="`${id}-${index}`" :key="index" :data-index="index" class="app-select-option" :class="{ highlighted: active === index, selected: item.value === modelValue, disabled: item.disabled }" role="option" :aria-selected="item.value === modelValue" :aria-disabled="item.disabled" @pointermove="!item.disabled && (active = index)" @click="choose(index)">
        <span v-if="!hideCheck" class="app-select-check">{{ item.value === modelValue ? '✓' : '' }}</span><span>{{ item.label }}</span>
      </div>
    </div>
  </Teleport>
  </button>
</template>
<style>
.app-select.select { position: relative; padding-right: 30px; }
.app-select::after { content: ""; position: absolute; right: 12px; top: 50%; width: 6px; height: 6px; border-right: 1.5px solid currentColor; border-bottom: 1.5px solid currentColor; transform: translateY(-70%) rotate(45deg); pointer-events: none; }
.app-select { text-align: left; cursor: pointer; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.app-select-menu { z-index: 10000; overflow-y: auto; padding: 5px; border: 1px solid var(--line-strong); border-radius: 9px; background: var(--surface-solid); color: var(--text); box-shadow: 0 8px 24px rgb(0 0 0 / 14%); font-size: 15px; scrollbar-width: thin; }
.app-select-option { min-height: 32px; display: flex; gap: 7px; align-items: center; padding: 6px 9px; border-radius: 5px; cursor: pointer; line-height: 20px; overflow-wrap: anywhere; }
.app-select-option > span:last-child { min-width: 0; }
.app-select-option.highlighted { background: var(--surface-soft); }
.app-select-option.selected { color: var(--accent-dark); background: var(--accent-soft); }
.app-select-option.disabled { opacity: .45; cursor: not-allowed; }
.app-select-check { flex: 0 0 14px; }
</style>
