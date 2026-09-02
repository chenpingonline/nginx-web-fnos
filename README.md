# nginx-web

nginx-web 是一个面向飞牛 fnOS 的原生 Nginx 反向代理可视化管理应用。

它自带独立的 Nginx Open Source 1.30.4，不读取、不修改、也不会重启飞牛系统 Nginx；不依赖 Docker，管理后台通过 fnOS 统一网关和 Unix Socket 提供。

## 功能

- HTTP 与手动证书 HTTPS 反向代理
- 多域名、独立监听端口、默认站点 `*`
- WebSocket、SSE/流式传输和大文件请求体
- 上游 HTTP/HTTPS、上游 TLS 校验开关
- Nginx 配置生成与 `nginx -t` 预检
- 原子替换、平滑 reload、激活失败自动回滚
- 配置历史与恢复为草稿
- Nginx 访问日志、错误日志和管理服务日志
- fnOS 管理员 Header 校验与变更请求标识
- 普通 `fnproxy` package 用户运行
- x86_64 与 ARM64 原生 FPK，不依赖 Docker

## 架构

```text
fnOS 桌面
   ↓
fnOS 统一网关 /app/nginx-web/
   ↓
TRIM_APPDEST/app.sock
   ↓
nginx-web Go 管理服务
   ↓
配置生成、nginx -t、平滑重载与回滚
   ↓
应用自带的独立 Nginx 1.30.4
   ↓
NAS 服务 / Docker 服务 / 局域网设备
```

默认无规则时，独立 Nginx 监听 `9080` 并返回 404。首版只允许 `1024–65535` 端口，因此不需要 root 权限。

## 与系统 Nginx 的隔离

nginx-web 只使用自己的 `TRIM_APPDEST`、`TRIM_PKGETC`、`TRIM_PKGVAR` 和 `TRIM_PKGTMP` 目录，不会访问 `/etc/nginx`、`/usr/trim/nginx`，也不会执行 `systemctl restart nginx`。

## 构建

要求：Go 1.22+、GNU tar、curl、Python 3、`file` 和 `binutils`。

```bash
make test
make build-x86
make build-arm64
# 或
make build-all
```

输出：

```text
dist/nginx-web-0.1.0-x86.fpk
dist/nginx-web-0.1.0-arm64.fpk
```

ARM64 构建会从 nginx.org 下载固定版本的官方 NGINX 源码、校验摘要，并在隔离的 ARM64 Alpine 容器中编译静态二进制，不添加第三方 NGINX 模块。x86 构建仍使用经过固定摘要校验的预编译包。来源说明位于 `third_party/nginx/`，也可以通过 `NGINX_BINARY=/path/to/nginx` 提供本地二进制。

需要在 ARM fnOS 设备上单独编译并导出官方 NGINX 二进制时，可以复制并执行 `scripts/build-nginx-arm64-on-fnos.sh`。默认产物输出到当前目录的 `nginx-arm64-output/`。

## 测试

```bash
make integration
make release
```

`tests/integration.sh` 会启动临时管理服务、独立 Nginx、HTTP 上游和临时自签名证书，验证 HTTP、HTTPS、配置应用、历史版本及平滑重载。

## 当前限制

- 不支持 32 位 ARMv7。
- 不直接监听 80/443，不申请 root 或 `CAP_NET_BIND_SERVICE`。
- HTTPS 证书目前需要手动导入 PEM，尚未内置 ACME 自动申请和续签。
- 暂未提供 TCP/UDP Stream、复杂 location、正则 rewrite、缓存和任意原始 Nginx 指令编辑。
- 发布前仍需分别在实体 x86_64、ARM64 fnOS 设备上完成安装验收。

## 许可证

nginx-web 源码使用 MIT License。Nginx Open Source 和 ARM64 静态构建所含组件的许可证见 `NGINX_LICENSE`、`NOTICE` 与 `THIRD_PARTY_LICENSES.md`。
