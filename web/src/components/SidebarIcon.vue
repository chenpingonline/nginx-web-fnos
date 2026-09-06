<script setup lang="ts">
defineProps<{
  name:
    | "overview"
    | "http"
    | "stream"
    | "upstreams"
    | "rate-limit"
    | "certificate"
    | "logs"
    | "requests"
    | "backup"
    | "history"
    | "config"
    | "settings";
}>();
</script>

<template>
  <svg
    viewBox="1.5 1.5 21 21"
    fill="none"
    stroke="currentColor"
    stroke-width="1.75"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <g v-if="name === 'requests'">
      <path d="M5 3h10l4 4v7M15 3v5h4M5 3v18h7M8 8h3M8 12h6M8 16h3" />
      <circle cx="17" cy="17" r="3" /><path d="m19.3 19.3 2.2 2.2" />
    </g>
    <g v-else-if="name === 'backup'">
      <path d="M4 8h16v12H4zM3 4h18v4H3zM9 12h6M12 12v5m-2-2 2 2 2-2" />
    </g>
    <g v-else-if="name === 'overview'">
      <rect x="3" y="3" width="7" height="7" rx="1.4" />
      <rect x="14" y="3" width="7" height="7" rx="1.4" />
      <rect x="3" y="14" width="7" height="7" rx="1.4" />
      <rect x="14" y="14" width="7" height="7" rx="1.4" />
    </g>

    <g v-else-if="name === 'http'">
      <path d="M4 8h16M8 4 4 8l4 4" />
      <path d="M20 16H4m12-4 4 4-4 4" />
    </g>

    <g v-else-if="name === 'stream'">
      <path d="m8.1 12-2.7 0m6-2.8 4.2-3.7m-4.2 9.3 4.2 3.7M12 12h5" />
      <circle cx="9.7" cy="12" r="2.1" />
      <circle cx="3.5" cy="12" r="1.8" />
      <circle cx="17.4" cy="4" r="1.8" />
      <circle cx="17.4" cy="20" r="1.8" />
      <circle cx="20.5" cy="12" r="1.8" />
    </g>

    <g v-else-if="name === 'upstreams'">
      <rect x="3" y="3.5" width="15.5" height="5" rx="1.3" />
      <rect x="3" y="9.5" width="15.5" height="5" rx="1.3" />
      <rect x="3" y="15.5" width="15.5" height="5" rx="1.3" />
      <path d="M18.5 6h1A1.5 1.5 0 0 1 21 7.5v9a1.5 1.5 0 0 1-1.5 1.5h-1" />
      <circle cx="6" cy="6" r="0.55" fill="currentColor" stroke="none" />
      <circle cx="6" cy="12" r="0.55" fill="currentColor" stroke="none" />
      <circle cx="6" cy="18" r="0.55" fill="currentColor" stroke="none" />
      <path d="M9 6h6.5M9 12h6.5M9 18h6.5" />
    </g>

    <g v-else-if="name === 'rate-limit'">
      <path d="M3.5 18a8.5 8.5 0 1 1 17 0H3.5Z" />
      <path d="m12 12 4.6-4.6" />
      <circle cx="12" cy="12" r="1.6" />
      <path d="M6.2 15.5h.1M17.7 15.5h.1M8 9.2h.1M12 7.5h.1" />
    </g>

    <g v-else-if="name === 'certificate'">
      <rect x="3" y="3.5" width="15" height="17" rx="1.4" />
      <path d="M6 7.5h9M6 11h7" />
      <circle cx="17" cy="16.5" r="3" />
      <path d="m15.5 19-.4 2.5 1.9-1 1.9 1-.4-2.5" />
    </g>

    <g v-else-if="name === 'logs'">
      <rect x="3" y="3" width="18" height="18" rx="1.5" />
      <path d="m6.5 8 2.5 2.3-2.5 2.3M11.5 13h5.5M6.5 17h10.5" />
    </g>

    <g v-else-if="name === 'history'">
      <path d="M4.5 7.5A9 9 0 1 1 3 14M4.5 3.5v4h4" />
      <path d="M12 7v5l3.5 2" />
    </g>

    <g v-else-if="name === 'config'">
      <path d="m8.5 6.5-5 5.5 5 5.5M15.5 6.5l5 5.5-5 5.5M13.5 4 10.5 20" />
    </g>

    <g v-else>
      <path d="M3 6h4m4 0h10M3 12h10m4 0h4M3 18h2m4 0h12" />
      <circle cx="9" cy="6" r="2" />
      <circle cx="15" cy="12" r="2" />
      <circle cx="7" cy="18" r="2" />
    </g>
  </svg>
</template>
