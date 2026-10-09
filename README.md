# nginx-web for fnOS

本仓库负责 nginx-web 的飞牛 FPK 打包、桌面入口、访问权限及安装/升级/卸载集成。

**功能源码统一在 [nginx-web 核心仓库](https://github.com/chenpingonline/nginx-web) 维护。** Linux、Docker、Go 后端和 Vue 页面都在核心仓库；这里通过 core.lock 固定源码提交，避免维护两套代码。

## 安装

飞牛安装包仍使用 appname nginx-web，标准版与全端口版继续兼容已有应用数据。安装包见 [本仓库 Releases](https://github.com/chenpingonline/nginx-web-fnos/releases)。仓库拆分不发布新 FPK，也不会自动升级设备。

- 标准版监听 1024–65535。
- 全端口版为 Nginx 提供绑定低端口的专用权限，管理服务仍以应用用户运行。
- 管理页面继续使用飞牛网关身份，外部目录通过飞牛“访问权限”授权。

[功能手册](https://github.com/chenpingonline/nginx-web/blob/main/docs/user-guide.md) · [Linux / Docker](https://github.com/chenpingonline/nginx-web)

## 构建 FPK

```bash
make core             # 取得 core.lock 固定的核心源码
make build-all        # 标准 / 全端口，AMD64 / ARM64
```

需要 Go 1.26、Node.js 22、npm、Python 3、Git、tar 和 file。脚本自动获取核心源码；Nginx 及许可证由核心仓库提供。

输出 dist/nginx-web-<version>-<standard|full-ports>-<x86_64|arm64>.fpk。程序、前端和 manifest 的版本均取自核心 VERSION，packaging/fnos/manifest.template 只保存平台元数据。

## 开发：代码只改核心一处

```bash
# 在核心仓库修改 Go / Vue 功能后，直接用本地源码打包
NGINX_WEB_CORE=/path/to/nginx-web make build-arm64

# 测试、提交并推送核心代码后，固定正式打包版本
python3 scripts/pin-core.py /path/to/nginx-web
make build-all
```

显式指定 NGINX_WEB_CORE 可使用未提交源码，FPK 的 CORE_SOURCE.json 会标记 dirty；正式 release 要求核心干净且与 core.lock 一致。默认缓存源码位于 .cache/core/<commit>，不应在那里修改功能。

核心引用的变更只修改 core.lock；功能代码始终只改核心仓库。固定的提交必须已推送到核心仓库，其他机器及 CI 才能获取它。现有用户数据、应用标识和权限流程保持兼容。

详细说明见 [构建文档](docs/build.md)。
