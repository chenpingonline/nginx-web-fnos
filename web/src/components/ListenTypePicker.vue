<script setup lang="ts">
import { computed } from "vue";
import type { ListenType } from "../types";
const props = defineProps<{ modelValue?: ListenType; disabled?: boolean }>();
const emit = defineEmits<{ "update:modelValue": [value: ListenType] }>();
const ipv4 = computed(() => props.modelValue !== "ipv6");
const ipv6 = computed(() => props.modelValue === "ipv6" || props.modelValue === "dual");
function toggle(family: "ipv4" | "ipv6", checked: boolean) {
  const v4 = family === "ipv4" ? checked : ipv4.value;
  const v6 = family === "ipv6" ? checked : ipv6.value;
  if (v4 || v6) emit("update:modelValue", v4 && v6 ? "dual" : v4 ? "ipv4" : "ipv6");
}
</script>
<template>
  <div class="listen-types" role="group" aria-label="监听类型">
    <label class="checkbox-row"><input type="checkbox" :checked="ipv4 && ipv6" :indeterminate="ipv4 !== ipv6" :disabled="disabled" @change="emit('update:modelValue', ($event.target as HTMLInputElement).checked ? 'dual' : 'ipv4')" />全选</label>
    <label class="checkbox-row"><input type="checkbox" :checked="ipv4" :disabled="disabled || (ipv4 && !ipv6)" @change="toggle('ipv4', ($event.target as HTMLInputElement).checked)" />IPv4</label>
    <label class="checkbox-row"><input type="checkbox" :checked="ipv6" :disabled="disabled || (ipv6 && !ipv4)" @change="toggle('ipv6', ($event.target as HTMLInputElement).checked)" />IPv6</label>
    <span class="field-help">至少选择一种</span>
  </div>
</template>
<style scoped>
.listen-types { display: flex; flex-wrap: wrap; align-items: center; gap: 12px 20px; }
.listen-types .field-help { margin: 0; }
</style>
