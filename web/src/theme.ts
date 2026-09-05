import { TrimApp } from '@trimjs/web-app';

// The fnOS host theme takes priority over the browser's preference.
export function followSystemTheme(): () => void {
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  let hostTheme = false;
  let disposed = false;
  let receivedEvent = false;
  let sdk: TrimApp | undefined;
  const onHostTheme = (theme: 'dark' | 'light') => {
    receivedEvent = true; hostTheme = true; apply(theme);
  };
  const apply = (theme: unknown) => {
    if (disposed || (theme !== 'dark' && theme !== 'light')) return;
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
  };
  const fallback = () => { if (!hostTheme) apply(media.matches ? 'dark' : 'light'); };
  fallback();
  media.addEventListener('change', fallback);
  void (async () => {
    try {
      sdk = new TrimApp();
      await sdk.ready();
      if (disposed) return;
      if (sdk.isStandaloneWeb) return;
      if (sdk.isWeb) {
        await sdk.$on('os/theme', onHostTheme).catch(() => {});
      }
      const config = await sdk.getPlatformConfig();
      if (!receivedEvent && (config.theme === 'dark' || config.theme === 'light')) {
        hostTheme = true;
        apply(config.theme);
      }
    } catch {
      // Older hosts do not expose the SDK. Keep the browser preference active.
      fallback();
    }
  })();
  return () => {
    disposed = true;
    media.removeEventListener('change', fallback);
    if (sdk?.isWeb && !sdk.isStandaloneWeb) void sdk.$off('os/theme', onHostTheme).catch(() => {});
  };
}
