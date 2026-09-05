// SDK 0.4.2 declarations reference unpublished @fn workspace packages.
// Keep a narrow declaration of the documented theme API we actually consume.
declare module '@trimjs/web-app' {
  export class TrimApp {
    constructor();
    isWeb: boolean;
    isStandaloneWeb: boolean;
    ready(): Promise<void>;
    getPlatformConfig(): Promise<{ theme: 'dark' | 'light' }>;
    $off(event: 'os/theme', callback: (theme: 'dark' | 'light') => void): Promise<void>;
    $on(event: 'os/theme', callback: (theme: 'dark' | 'light') => void): Promise<void>;
  }
}
