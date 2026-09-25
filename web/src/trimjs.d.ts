// SDK 0.4.2 declarations reference unpublished @fn workspace packages.
// Keep a narrow declaration of the documented SDK APIs we actually consume.
declare module '@trimjs/web-app' {
  export class TrimApp {
    constructor();
    isWeb: boolean;
    isStandaloneWeb: boolean;
    ready(): Promise<void>;
    openAppSetting(): Promise<unknown>;
    pickFile(params: { directory: boolean; multiple?: boolean; title?: string; okText?: string }): Promise<string[] | undefined>;
    getPlatformConfig(): Promise<{ theme: 'dark' | 'light' }>;
    $off(event: 'os/theme', callback: (theme: 'dark' | 'light') => void): Promise<void>;
    $on(event: 'os/theme', callback: (theme: 'dark' | 'light') => void): Promise<void>;
  }
}
