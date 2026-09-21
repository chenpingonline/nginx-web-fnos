import { TrimApp } from '@trimjs/web-app';

type Theme = 'dark' | 'light';
const THEME_STORAGE_KEY = 'nginx-web:host-theme';

export interface SystemThemeFollower {
  ready: Promise<void>;
  stop: () => void;
}

function parseHostTheme(payload: unknown): Theme | undefined {
  if (payload === 'dark' || payload === 'light') return payload;
  if (!payload || typeof payload !== 'object' || !('theme' in payload)) return undefined;
  const theme = payload.theme;
  return theme === 'dark' || theme === 'light' ? theme : undefined;
}

function rememberHostTheme(theme: Theme) {
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // Storage can be unavailable in restricted WebViews.
  }
}

// The fnOS host theme takes priority over the browser's preference.
export function followSystemTheme(): SystemThemeFollower {
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  const embedded = window.parent !== window;
  let hostTheme = false;
  let disposed = false;
  let hostRevision = 0;
  let hostReady = false;
  let subscribed = false;
  let sdk: TrimApp | undefined;
  let resolveReady: (() => void) | undefined;
  let readyTimer: number | undefined;
  const ready = new Promise<void>((resolve) => { resolveReady = resolve; });
  const markReady = () => {
    if (!resolveReady) return;
    if (readyTimer !== undefined) window.clearTimeout(readyTimer);
    resolveReady();
    resolveReady = undefined;
  };
  const onHostTheme = (payload: unknown) => {
    const theme = parseHostTheme(payload);
    if (!theme) {
      console.warn('[nginx-web] 无法识别飞牛主题事件。', payload);
      return;
    }
    hostRevision += 1;
    hostTheme = true;
    apply(theme);
    rememberHostTheme(theme);
    markReady();
  };
  const apply = (theme: unknown) => {
    if (disposed || (theme !== 'dark' && theme !== 'light')) return;
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
  };
  const fallback = () => { if (!hostTheme) apply(media.matches ? 'dark' : 'light'); };
  const finishWithFallback = () => {
    fallback();
    markReady();
  };
  // A real host normally answers immediately. Give its theme priority over the
  // operating-system preference; older hosts still fall back after SDK probing.
  readyTimer = window.setTimeout(finishWithFallback, embedded ? 1800 : 100);
  const refreshHostTheme = async () => {
    if (!sdk || !hostReady || disposed) return;
    const revision = hostRevision;
    try {
      const config = await sdk.getPlatformConfig();
      if (disposed || revision !== hostRevision) return;
      if (config.theme === 'dark' || config.theme === 'light') {
        hostTheme = true;
        apply(config.theme);
        rememberHostTheme(config.theme);
      }
    } catch (error) {
      console.warn('[nginx-web] 无法读取飞牛主题，等待事件或使用浏览器主题兜底。', error);
    }
  };
  const onMediaTheme = () => {
    if (!resolveReady) fallback();
  };
  const refreshVisibleHostTheme = () => {
    if (!document.hidden) void refreshHostTheme();
  };
  media.addEventListener('change', onMediaTheme);
  window.addEventListener('focus', refreshVisibleHostTheme);
  document.addEventListener('visibilitychange', refreshVisibleHostTheme);
  void (async () => {
    try {
      sdk = new TrimApp();
      await sdk.ready();
      if (disposed) return;
      if (sdk.isStandaloneWeb) {
        finishWithFallback();
        return;
      }
      hostReady = true;
      await refreshHostTheme();
      if (hostTheme) markReady();
      else if (!embedded) finishWithFallback();
      if (disposed) return;
      if (sdk.isWeb) {
        try {
          await sdk.$on('os/theme', onHostTheme);
          if (disposed) {
            await sdk.$off('os/theme', onHostTheme).catch(() => {});
            return;
          }
          subscribed = true;
        } catch (error) {
          console.warn('[nginx-web] 无法监听飞牛主题变化，将在窗口激活时刷新。', error);
        }
      }
      await refreshHostTheme();
      if (hostTheme) markReady();
    } catch (error) {
      // Older hosts do not expose the SDK. Keep the browser preference active.
      console.warn('[nginx-web] 飞牛宿主 SDK 不可用，继续使用浏览器主题。', error);
      finishWithFallback();
    }
  })();
  return {
    ready,
    stop: () => {
      disposed = true;
      if (readyTimer !== undefined) window.clearTimeout(readyTimer);
      markReady();
      media.removeEventListener('change', onMediaTheme);
      window.removeEventListener('focus', refreshVisibleHostTheme);
      document.removeEventListener('visibilitychange', refreshVisibleHostTheme);
      if (subscribed && sdk) void sdk.$off('os/theme', onHostTheme).catch(() => {});
    },
  };
}
