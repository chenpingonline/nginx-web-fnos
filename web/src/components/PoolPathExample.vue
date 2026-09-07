<template>
  <section class="pool-example" aria-label="服务组访问路径示例（示例数据）">
    <svg class="example-path" viewBox="0 0 640 150" role="img" aria-label="示例：用户访问 example.com，经 Nginx 代理规则进入后端服务组，再选择服务器 A 或服务器 B 处理请求">
      <g class="path-lines" fill="none" stroke-linecap="round" stroke-linejoin="round">
        <path d="M112 75 H150 M144 71 L150 75 L144 79" />
        <path d="M270 75 H308 M302 71 L308 75 L302 79" />
        <path d="M428 75 H440 C456 75 452 37 470 37 H490 M484 33 L490 37 L484 41" />
        <path d="M428 75 H440 C456 75 452 113 470 113 H490 M484 109 L490 113 L484 117" />
      </g>
      <g class="path-node">
        <rect x="2" y="49" width="110" height="52" rx="8" />
        <text x="57" y="70" class="node-title">用户访问</text>
        <text x="57" y="87" class="node-detail">example.com</text>
      </g>
      <g class="path-node">
        <rect x="150" y="49" width="120" height="52" rx="8" />
        <text x="210" y="70" class="node-title">Nginx 代理规则</text>
        <text x="210" y="87" class="node-detail">接收并转发请求</text>
      </g>
      <g class="path-node pool-node">
        <rect x="308" y="49" width="120" height="52" rx="8" />
        <text x="368" y="70" class="node-title">后端服务组</text>
        <text x="368" y="87" class="node-detail">负载均衡 · 轮询</text>
      </g>
      <g class="path-node">
        <rect x="490" y="11" width="148" height="52" rx="8" />
        <text x="564" y="32" class="node-title">服务器 A</text>
        <text x="564" y="49" class="node-detail">192.168.1.10:8080</text>
        <rect x="490" y="87" width="148" height="52" rx="8" />
        <text x="564" y="108" class="node-title">服务器 B</text>
        <text x="564" y="125" class="node-detail">192.168.1.11:8080</text>
      </g>
    </svg>
  </section>
</template>
<style scoped>
.pool-example { width: 100%; overflow-x: auto; }
.example-path { display: block; width: 100%; min-width: 520px; height: auto; margin: 0 auto; }
.path-lines { stroke: var(--text-muted); stroke-opacity: .55; stroke-width: 1.5; }
.path-node rect { fill: var(--surface-soft); stroke: var(--line-strong); stroke-width: 1; }
.path-node text { text-anchor: middle; }
.node-title { fill: var(--text-muted); font-size: 14px; font-weight: 600; }
.node-detail { fill: var(--text-muted); opacity: .85; font-size: 12px; }
.pool-node rect { fill: var(--surface-soft); stroke: var(--accent); stroke-width: 2; stroke-opacity: .35; }
.pool-node .node-title { fill: #599c7e; }
:global(html[data-theme="dark"] .pool-example .pool-node .node-title) { fill: #83bda3; }
</style>
