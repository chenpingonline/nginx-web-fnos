<p align="center">
  <img src="packaging/fnos/ICON_256.PNG" width="104" alt="nginx-web 图标">
</p>

<h1 align="center">nginx-web for fnOS</h1>

<p align="center">在飞牛 NAS 上，通过可视化界面管理反向代理、证书和流量。</p>

<p align="center">
  <a href="https://github.com/chenpingonline/nginx-web-fnos/releases/latest"><img src="https://img.shields.io/github/v/release/chenpingonline/nginx-web-fnos?label=Release" alt="最新版本"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/fnOS-x86__64%20%7C%20ARM64-009688" alt="支持 x86_64 和 ARM64">
  <img src="https://img.shields.io/badge/NGINX-1.30.4-009639?logo=nginx" alt="NGINX 1.30.4">
</p>

<p align="center">
  <a href="https://github.com/chenpingonline/nginx-web-fnos/releases/latest">下载安装</a> ·
  <a href="#快速上手">快速上手</a> ·
  <a href="docs/features.md">功能文档</a> ·
  <a href="docs/build.md">源码构建</a> ·
  <a href="https://github.com/chenpingonline/nginx-web-fnos/issues">反馈问题</a>
</p>

## 项目介绍

**nginx-web** 是面向飞牛 fnOS 的原生 Nginx 管理应用。用表单配置 HTTP/HTTPS 反向代理、TCP/UDP 转发、SSL 证书和后端服务组，让 NAS 应用与局域网服务拥有统一的访问入口。

应用自带独立的 **Nginx Open Source 1.30.4**，通过 fnOS 桌面和统一网关访问管理界面，安装后运行无需 Docker。配置、日志和进程均由应用独立管理，不读取、修改或重启飞牛系统 Nginx。

仓库名为 `nginx-web-fnos`；fnOS 内的应用名称和安装标识保持为 `nginx-web`。

## 主要功能

| 能力 | 说明 |
| --- | --- |
| HTTP / HTTPS 代理 | 域名、路径转发、WebSocket、SSE、HTTP/2、静态文件与跳转 |
| 规则分组 | 分组管理代理，共享协议、端口、证书和 HTTP/2 默认值；各项可独立覆盖 |
| TCP / UDP 转发 | 四层代理、TLS 终止、SNI 分流与 PROXY Protocol |
| 后端服务组 | 多节点、权重、备用节点及多种负载均衡策略 |
| SSL / TLS 证书 | PEM 导入、ACME DNS-01 自动签发与续期，接入 39 个 DNS 服务商适配器 |
| 访问控制 | IP/CIDR 规则、Basic Auth、限流策略、连接数及下载速度限制 |
| 流量与日志 | HTTP 请求趋势、4xx/5xx 错误分析、规则维度统计及运行日志 |
| 配置与恢复 | 配置校验、平滑重载、失败回滚、历史快照和 JSON 备份恢复 |
| fnOS 集成 | 管理员身份校验、统一网关访问、自动跟随平台亮暗主题 |

更多路径处理、缓存、Header、TLS 和运行参数见 [功能与使用说明](docs/features.md)。

## 安装

要求 **fnOS 1.1.3100 或更新版本**。按 NAS 的 CPU 架构，从 [最新 Release](https://github.com/chenpingonline/nginx-web-fnos/releases/latest) 下载 FPK：

| NAS 架构 | 下载文件 |
| --- | --- |
| Intel / AMD，x86_64（AMD64） | `nginx-web-<版本>-x86.fpk` |
| ARM64（aarch64） | `nginx-web-<版本>-arm64.fpk` |

1. 下载匹配架构的 `.fpk`；Release 同时提供 `SHA256SUMS.txt` 供核对文件完整性。
2. 在 fnOS 应用中心使用手动安装，选择下载的 FPK 并完成安装。
3. 使用 fnOS 管理员账号，从桌面打开 **nginx-web**。

不支持 32 位 ARMv7。平台自动主题需要 fnOS 1.2.0401 / App 1.34.0 及以上；旧系统或独立浏览器会跟随浏览器主题。

## 快速上手

以把 `nas.example.com:9080` 转发到局域网服务 `192.168.1.10:3000` 为例：

1. 确保 NAS 能访问目标服务，并将域名解析到可访问的 NAS 地址。
2. 打开 **代理 HTTP(S)**，添加规则：协议选 HTTP，域名填 `nas.example.com`，监听端口填 `9080`。
3. 后端协议选 HTTP，主机填 `192.168.1.10`，端口填 `3000`；按需开启 WebSocket 或流式传输。
4. 保存并应用配置，在总览确认规则已生效、Nginx 正在运行，然后访问 `http://nas.example.com:9080`。

需要 HTTPS 时，先在 **SSL/TLS 证书** 导入证书，或通过 ACME 申请，再为规则选择 HTTPS、证书及空闲监听端口（例如 `9443`）。通配符证书和 DNS 凭据填写方式见 [DNS 服务商说明](docs/dns-providers.md)。

> 应用以普通用户运行，监听端口范围为 **1024–65535**。如需使用外部 80/443，可在路由器将其映射到应用实际监听的高位端口。无规则时，默认 `9080` 返回 404 属于正常行为。

## 配置应用与升级

配置通过结构化数据生成，应用前先执行 `nginx -t`；校验通过后替换正式配置，并在 Nginx 运行时平滑重载。应用失败会尝试恢复上一份有效配置。保存草稿、恢复备份或历史快照后，应检查界面上的待应用状态。

升级前可在 **备份与恢复** 下载 JSON 备份，再通过 fnOS 安装新版本 FPK。备份包含证书私钥，应妥善保管；**不包含 ACME 账户、DNS 凭据或自动续期任务**，迁移到新设备时需重新配置续期，外部引用文件也需单独迁移。

从旧版本升级后，如首页提示统计入口未启用，需要检查并应用一次当前草稿，才能开始采集新的统计数据。

## 常见问题

**会影响飞牛自带的 Nginx 吗？**

应用只使用自己的安装、配置、数据和临时目录，不操作系统 Nginx。仍需选择未被其他服务占用的监听端口。

**需要 Docker 吗？**

安装和运行 FPK 不需要。只有从官方源码编译内置 Nginx 时，构建脚本使用 Docker 隔离编译依赖。

**可以直接编辑 nginx.conf 吗？**

当前提供生成配置的只读预览，配置修改通过表单完成，不开放任意原始指令编辑。

**为什么没有流量数据？**

统计从开始采集后积累；缺失历史保留为空。关闭访问日志会停止规则日志统计。WebSocket、SSE 和长下载在请求结束后才计入完成请求数。TCP/UDP 会话不计入 HTTP 指标。

**支持哪些证书验证方式？**

ACME 当前支持 DNS-01，尚不支持 HTTP-01、IP 地址签发或飞牛系统证书同步。已接入适配器不代表每个 DNS 服务商都经过真实账号验证。

## 文档与开发

- [功能与使用说明](docs/features.md)：代理、证书、统计、备份和全局设置。
- [DNS 服务商说明](docs/dns-providers.md)：支持列表及凭据配置入口。
- [从源码构建](docs/build.md)：准备 Nginx、构建前端与双架构 FPK。
- [测试说明](tests/README.md)：单元测试、真实 Nginx 集成与生命周期检查。

技术栈为 **Go + Vue 3 + TypeScript + Vite**。版本唯一来源是 [`packaging/fnos/manifest`](packaging/fnos/manifest)。

```bash
git clone https://github.com/chenpingonline/nginx-web-fnos.git
cd nginx-web-fnos
make frontend-install
make test
```

构建安装包还需准备对应架构的 Nginx 二进制，详见 [构建文档](docs/build.md)。

## 当前边界

内置 Nginx 未包含 HTTP/3/QUIC、Brotli、Lua/OpenResty、JWT、headers-more、GeoIP2、ModSecurity/WAF、第三方主动健康检查或 Prometheus 模块。Basic Auth 密码文件和客户端 CA 等外部文件需自行维护，并确保应用用户可读。

每个 Release 的测试范围以发布说明为准；构建与包校验不能替代实体 fnOS 设备的安装、升级及使用验证。

## 参与贡献

欢迎提交 [Issue](https://github.com/chenpingonline/nginx-web-fnos/issues) 或 Pull Request。报告问题时请提供应用版本、fnOS 版本、CPU 架构、复现步骤和脱敏日志；请勿附上私钥、DNS Token 或完整配置备份。代码修改请附相关测试结果。

## 许可证与致谢

项目源码采用 [MIT License](LICENSE)。内置 Nginx 及相关组件的许可证与来源见 [NGINX_LICENSE](NGINX_LICENSE)、[NOTICE](NOTICE) 和 [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)。

感谢 [NGINX](https://nginx.org/)、[lego](https://github.com/go-acme/lego)、[Vue](https://github.com/vuejs/core) 等开源项目。README 的组织方式参考了 [Nginx Proxy Manager](https://github.com/NginxProxyManager/nginx-proxy-manager) 和 [Nginx UI](https://github.com/0xJacky/nginx-ui)。
