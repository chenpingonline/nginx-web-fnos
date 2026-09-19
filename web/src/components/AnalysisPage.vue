<script setup lang="ts">
import { ref } from 'vue';
import type { Overview } from '../types';
import ErrorDetailsPage from './ErrorDetailsPage.vue';
import StreamAnalysisPage from './StreamAnalysisPage.vue';
defineProps<{ overview: Overview; busy: boolean; updatedAt: string; initialMinutes: number; initialRule: string }>();
const emit = defineEmits<{ overview: [value: Overview]; edit: [id: string] }>();
const protocol = ref('http');
</script>
<template>
  <div class="analysis-page">
    <header class="details-heading">
      <h1 :aria-label="`${protocol === 'http' ? 'HTTP / HTTPS' : 'TCP / UDP'} 分析`">
        <span class="range-buttons protocol-switch" aria-label="分析协议">
          <button :class="{ active: protocol === 'http' }" :aria-pressed="protocol === 'http'" @click="protocol = 'http'">HTTP / HTTPS</button>
          <button :class="{ active: protocol === 'stream' }" :aria-pressed="protocol === 'stream'" @click="protocol = 'stream'">TCP / UDP</button>
        </span>
        <span>分析</span>
      </h1>
      <p>{{ protocol === 'http' ? '分析 HTTP / HTTPS 请求量、响应性能、错误和后端表现' : '分析已结束会话、收发流量、会话时长和后端异常' }}</p>
    </header>
    <ErrorDetailsPage v-if="protocol === 'http'" :overview="overview" :busy="busy" :updated-at="updatedAt" :initial-minutes="initialMinutes" :initial-rule="initialRule" @overview="emit('overview', $event)" @edit="emit('edit', $event)" />
    <StreamAnalysisPage v-else :updated-at="updatedAt" :initial-minutes="initialMinutes" @overview="emit('overview', $event)" />
  </div>
</template>
<style scoped>
.analysis-page { display:grid; grid-template-columns:minmax(0,1fr); gap:12px; min-width:0; }
.details-heading { padding:2px 0 4px; }
.details-heading h1 { display:flex; align-items:center; gap:8px; }
.protocol-switch { width:fit-content; }
.protocol-switch button { padding:2px 7px; font-size:inherit; font-weight:inherit; line-height:1.4; }
</style>

<style scoped src="./analysis-common.css"></style>
