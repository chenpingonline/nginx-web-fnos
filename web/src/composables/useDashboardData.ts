import {
  computed,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
  type Ref,
} from "vue";
import { errorMessage, request } from "../api";
import type { DashboardData, Overview } from "../types";

export function useDashboardData(options: {
  minutes: Ref<number>;
  selected: Ref<string>;
  updatedAt: () => string;
  onOverview: (value: Overview) => void;
}) {
  const data = ref<DashboardData | null>(null);
  const error = ref("");
  const fetching = ref(false);
  const loadedMinutes = ref<number | null>(null);
  const loadedRule = ref<string | null>(null);
  let timer: ReturnType<typeof setInterval> | undefined;
  let controller: AbortController | undefined;
  let generation = 0;

  async function load() {
    const current = ++generation;
    const minutes = options.minutes.value;
    const rule = options.selected.value;
    controller?.abort();
    controller = new AbortController();
    fetching.value = true;
    try {
      const result = await request<DashboardData>(
        `/dashboard?minutes=${minutes}&rule=${encodeURIComponent(rule)}`,
        { signal: controller.signal },
      );
      if (current !== generation) return;
      data.value = result;
      loadedMinutes.value = minutes;
      loadedRule.value = rule;
      error.value = "";
      options.onOverview(result.overview);
    } catch (cause) {
      if (
        current === generation &&
        !(cause instanceof DOMException && cause.name === "AbortError")
      ) {
        error.value = errorMessage(cause);
      }
    } finally {
      if (current === generation) fetching.value = false;
    }
  }
  function refreshVisible() {
    if (!document.hidden && !fetching.value) void load();
  }
  onMounted(() => {
    void load();
    timer = setInterval(refreshVisible, 5000);
    document.addEventListener("visibilitychange", refreshVisible);
  });
  onBeforeUnmount(() => {
    generation++;
    controller?.abort();
    clearInterval(timer);
    document.removeEventListener("visibilitychange", refreshVisible);
  });
  watch(
    [options.minutes, options.selected, options.updatedAt],
    () => void load(),
  );

  const stats = computed(() =>
    loadedMinutes.value === options.minutes.value &&
    loadedRule.value === options.selected.value
      ? data.value?.metrics
      : undefined,
  );
  const rules = computed(() =>
    (data.value?.rules ?? []).map((rule) =>
      loadedMinutes.value === options.minutes.value
        ? rule
        : { ...rule, counts: null },
    ),
  );
  return { data, error, fetching, stats, rules, load };
}
