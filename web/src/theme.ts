import { TrimApp } from '@trimjs/web-app';

type Theme = 'dark' | 'light';

function parseHostTheme(payload: unknown): Theme | undefined {
  if (payload === 'dark' || payload === 'light') return payload;
  if (!payload || typeof payload !== 'object' || !('theme' in payload)) return undefined;
  const theme = payload.theme;
  return theme === 'dark' || theme === 'light' ? theme : undefined;
}

// The fnOS host theme takes priority over the browser's preference.
export function followSystemTheme(): () => void {
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  let hostTheme = false;
  let disposed = false;
  let hostRevision = 0;
  let hostReady = false;
  let subscribed = false;
  let sdk: TrimApp | undefined;
  const onHostTheme = (payload: unknown) => {
    const theme = parseHostTheme(payload);
    if (!theme) {
      console.warn('[nginx-web] 无法识别飞牛主题事件。', payload);
      return;
    }
    hostRevision += 1;
    hostTheme = true;
    apply(theme);
  };
  const apply = (theme: unknown) => {
    if (disposed || (theme !== 'dark' && theme !== 'light')) return;
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
  };
  const fallback = () => { if (!hostTheme) apply(media.matches ? 'dark' : 'light'); };
  const refreshHostTheme = async () => {
    if (!sdk || !hostReady || disposed) return;
    const revision = hostRevision;
    try {
      const config = await sdk.getPlatformConfig();
      if (disposed || revision !== hostRevision) return;
      if (config.theme === 'dark' || config.theme === 'light') {
        hostTheme = true;
        apply(config.theme);
      }
    } catch (error) {
      console.warn('[nginx-web] 无法读取飞牛主题，继续使用浏览器主题。', error);
      fallback();
    }
  };
  const refreshVisibleHostTheme = () => {
    if (!document.hidden) void refreshHostTheme();
  };
  fallback();
  media.addEventListener('change', fallback);
  window.addEventListener('focus', refreshVisibleHostTheme);
  document.addEventListener('visibilitychange', refreshVisibleHostTheme);
  void (async () => {
    try {
      sdk = new TrimApp();
      await sdk.ready();
      if (disposed) return;
      if (sdk.isStandaloneWeb) return;
      hostReady = true;
      await refreshHostTheme();
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
    } catch (error) {
      // Older hosts do not expose the SDK. Keep the browser preference active.
      console.warn('[nginx-web] 飞牛宿主 SDK 不可用，继续使用浏览器主题。', error);
      fallback();
    }
  })();
  return () => {
    disposed = true;
    media.removeEventListener('change', fallback);
    window.removeEventListener('focus', refreshVisibleHostTheme);
    document.removeEventListener('visibilitychange', refreshVisibleHostTheme);
    if (subscribed && sdk) void sdk.$off('os/theme', onHostTheme).catch(() => {});
  };
}
