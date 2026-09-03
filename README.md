# nginx-web

nginx-web 是一个面向飞牛 fnOS 的原生 Nginx 反向代理可视化管理应用，FPK 模板集中保存在 `packaging/fnos/`。

它自带独立的 Nginx Open Source 1.30.4，不读取、不修改、也不会重启飞牛系统 Nginx；不依赖 Docker，管理后台通过 fnOS 统一网关和 Unix Socket 提供。

## 页面功能总览

管理页面使用 Vue 3、TypeScript 和 Vite，共有九个主要页面。所有配置都通过结构化表单生成，不允许直接提交任意 Nginx 指令。

### 总览

- 查看独立 Nginx 的运行状态、PID、版本和监听端口。
- 查看 HTTP/HTTPS 与 TCP/UDP 规则总数、启用数量、证书数量和草稿状态。
- 显示最近一次应用时间、当前启用的代理和最近一条 Nginx 错误。
- 快速添加 HTTP/HTTPS 代理、导入证书、校验配置和查看日志。
- 启动、停止或平滑重载应用自带的 Nginx。

### HTTP/HTTPS 代理规则

- 创建、编辑、启用、停用、搜索和删除代理规则。
- 配置规则名称、一个或多个域名/IP、监听端口及 `*` 默认站点。
- 配置 HTTP 或 HTTPS 入口、手动选择证书及 HTTP/2。
- 使用单个 HTTP/HTTPS 目标服务，或选择可复用的 HTTP 目标服务池。
- 配置目标服务 TLS 证书校验、Host 保留、WebSocket、SSE/流式传输、请求体大小及连接/读取/发送超时。
- 按客户端 IP 限制每秒请求数、突发请求、并发连接数和下载速度。
- 为根路径和额外 Location 分别选择前缀、精确或正则匹配，并为每个路径配置不同处理方式。
- Location 后端支持 HTTP 反向代理、静态文件、固定返回/跳转、gRPC、FastCGI、uWSGI、SCGI、Memcached 和 Stub Status。
- 静态文件支持 `root`/`alias`、Index、目录浏览、Expires 和 Try Files。
- 支持 HTTP 跳转 HTTPS，以及 `last`、`break`、临时跳转和永久跳转 Rewrite。
- 支持代理缓存区、磁盘上限、未访问失效、响应有效期、自定义缓存 Key、变量绕过缓存、故障使用过期缓存和大文件 Slice。
- 支持 IP/CIDR 允许与拒绝、Basic Auth、Auth Request、Secure Link、Referer 防盗链，以及静态 Location 的有限 WebDAV。
- 支持添加、覆盖或清空目标服务请求 Header，以及通过原生 `add_header` 添加响应 Header。
- 支持 Sub Filter 内容替换、Addition 响应前后追加、Mirror 请求镜像和 SSI。

### TCP/UDP 代理

- 创建、编辑、启用、停用和删除 TCP/UDP 四层代理规则。
- 配置监听地址、监听端口、单个目标服务或 Stream 目标服务池。
- 配置连接超时、会话超时和 UDP 响应次数。
- 支持入口接收和向目标服务发送 PROXY Protocol，并配置可信代理地址。
- TCP 支持关闭 TLS、TLS 终止和 TLS SNI 透传；TLS 终止可选择已导入证书。
- SNI 透传可按多个域名分流到不同单节点目标服务或 Stream 目标服务池。
- 支持 Stream 访问日志、单 IP 最大连接数及 IP/CIDR 允许与拒绝。
- 适用于 SSH、数据库、MQTT、游戏服务和 HTTPS 四层透传等场景。

### 目标服务池

- 分别创建供 HTTP/HTTPS 或 TCP/UDP 使用的服务器池，并在多个规则间复用。
- 管理多个服务器节点的主机、端口、权重、最大失败次数、故障恢复时间、备份和停用状态。
- HTTP 池支持 Round Robin、Least Connections、IP Hash、Hash 和 Random Two Least Connections。
- Stream 池支持 Round Robin、Least Connections、Hash 和 Random Two Least Connections。
- 配置 Keepalive 数量、单连接最大请求数、单连接最长时间和空闲超时。
- 删除前检查规则引用，避免留下无效配置。

### HTTPS 证书

- 手动导入 PEM 证书链与私钥，并校验证书、私钥是否匹配。
- 查看证书主体、SAN 域名/IP、序列号、有效期、状态和 SHA-256 指纹。
- 私钥不会通过 API 返回浏览器；证书目录为 `0700`，私钥文件为 `0600`。
- 删除前检查 HTTP 和 Stream 规则引用。

### 运行日志

- 查看最近的 Nginx 错误日志、HTTP 访问日志、Stream 访问日志和管理服务日志。
- 页面每次读取最近 500 行，避免浏览器一次加载整个日志文件。
- Nginx 日志支持按配置大小自动轮转、保留指定数量，并可在页面立即轮转。

### 配置历史

- 每次“保存并应用”成功后自动保存配置快照。
- 查看快照时间、说明、规则数量和启用数量。
- 将历史版本恢复为草稿，检查后再决定是否应用。
- 删除不再需要的历史记录，并配置最多保留 1～100 个版本。

### Nginx 配置

- 只读查看当前实际使用的 `nginx.conf` 和生成的 HTTP/Stream 配置片段。
- 在多个配置文件标签间切换，并复制当前文件内容。
- 配置文件来自结构化数据，不暴露任意原始指令编辑入口。

### 全局设置

- 设置默认 HTTP/HTTPS 端口和配置历史保留数量。
- 设置 Worker 数量、Worker Connections、文件句柄上限、Multi Accept、文件 AIO 和线程池。
- 文件句柄未手动指定时，会根据 fnOS 当前软限制自动降低 Worker Connections，避免资源限制警告。
- 配置 Real IP Header、可信代理网段和递归代理链解析。
- 配置 Gzip 开关、压缩级别、最小响应大小、MIME 类型、Gzip Static 和 Gunzip。
- 配置 TLS 1.2/1.3、加密套件、会话缓存、会话超时、OCSP Stapling 和客户端证书校验。
- 配置访问日志开关、错误日志级别、自定义访问日志格式、缓冲、刷新周期和轮转策略。
- 使用 Map、Geo 和 Split Clients 创建可供 Header、Rewrite 等配置引用的动态变量和灰度分流变量。
- 一键清理 nginx-web 自己的全部 HTTP 代理缓存。

### 应用、校验与安全保护

- 页面顶部可随时刷新状态、运行 `nginx -t`，或保存并应用全部草稿。
- 应用配置时先在隔离候选目录运行 `nginx -t`，通过后再原子替换正式配置。
- 已运行时使用平滑 Reload；启动或重载失败时自动恢复上一份有效配置。
- 校验重复域名、端口冲突、证书/目标服务池引用、IP/CIDR、路径和指令参数范围。
- 管理接口要求 fnOS 管理员身份，并为变更请求校验专用请求标识。
- 管理服务和 Nginx 均以普通 `nginx-web` package 用户运行，不申请 root 权限。
- 提供 AMD64 与 ARM64 原生 FPK；安装后的应用运行不依赖 Docker。

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

页面只保存结构化配置，不接受任意 Nginx 指令。Basic Auth 使用 fnOS 上已有的 htpasswd 文件，页面只记录绝对路径，不保存明文密码。例如可在隔离环境生成后复制到应用可读目录：

```bash
htpasswd -c /vol1/appdata/nginx-web/.htpasswd admin
```

响应 Header 使用 Nginx 原生 `add_header`，不等同于未编译的第三方 `headers-more` 模块。

## 源码结构

```text
cmd/nginx-web/       命令入口
internal/app/        服务生命周期与诊断
internal/domain/     配置模型和校验规则
internal/httpapi/    HTTP API 与管理权限
internal/nginx/      Nginx 配置生成和进程管理
internal/platform/   fnOS 与开发环境路径
internal/service/    应用业务逻辑
internal/store/      状态持久化
packaging/fnos/      fnOS FPK 模板
scripts/             构建、验证和发布脚本
third_party/nginx/   Nginx 来源与摘要记录
web/                 Vue 3 + TypeScript + Vite 管理页面
```

## 与系统 Nginx 的隔离

nginx-web 只使用自己的 `TRIM_APPDEST`、`TRIM_PKGETC`、`TRIM_PKGVAR` 和 `TRIM_PKGTMP` 目录，不会访问 `/etc/nginx`、`/usr/trim/nginx`，也不会执行 `systemctl restart nginx`。

## 构建

### 1. 在 fnOS 上编译 NGINX

FPK 打包不会自动下载或编译 NGINX。请先在对应架构的 fnOS 设备或虚拟机上安装并启动 Docker，然后执行：

```bash
chmod +x scripts/build-nginx-on-fnos.sh
./scripts/build-nginx-on-fnos.sh
```

脚本会自动识别当前主机是 ARM64 还是 AMD64，下载并校验官方 NGINX 1.30.4 源码，然后在 `alpine:3.21` 容器中编译静态二进制。Docker 容器和临时编译目录会在完成后清理，不会向 fnOS 本体安装编译依赖。

默认产物：

| fnOS 架构 | NGINX 二进制 |
| --- | --- |
| ARM64 | `nginx-arm64-output/nginx-1.30.4-aarch64-linux` |
| AMD64 | `nginx-amd64-output/nginx-1.30.4-x86_64-linux` |

如需指定输出目录，可以把目录作为第一个参数：

```bash
./scripts/build-nginx-on-fnos.sh /path/to/output
```

每次编译还会生成对应的 `.sha256` 和 `.build-info.txt` 文件，用于核对二进制摘要、源码来源和编译参数。

### 2. 将二进制放入项目

把 fnOS 上生成的二进制取回项目，并按架构放到固定位置：

```text
ARM64  → third_party/nginx/arm64/nginx
AMD64  → third_party/nginx/x86_64/nginx
```

例如 ARM64：

```bash
cp nginx-1.30.4-aarch64-linux third_party/nginx/arm64/nginx
```

例如 AMD64：

```bash
cp nginx-1.30.4-x86_64-linux third_party/nginx/x86_64/nginx
```

这两个本地二进制已被 `.gitignore` 排除，不会提交到 Git。FPK 打包时会检查 NGINX 的目标架构、静态链接属性和版本。

### 3. 构建 FPK

要求：Go 1.22+、Node.js 20.19+ 或 22.12+、npm、GNU tar、Python 3 和 `file`。

首次构建先安装前端依赖：

```bash
make frontend-install
```

开发管理页面时可以使用：

```bash
npm --prefix web run dev
npm --prefix web run typecheck
npm --prefix web run build
```

Vite 开发服务器适合检查页面布局；需要调用真实 API 时，应使用 Go 管理服务提供的页面。`scripts/build.sh` 会在每次 FPK 打包前自动执行前端类型检查和生产构建，并将 `web/dist/` 嵌入 `nginx-web-server`。

```bash
make test
make build-x86
make build-arm64
# 或
make build-all
```

输出：

```text
dist/nginx-web-0.1.5-x86.fpk
dist/nginx-web-0.1.5-arm64.fpk
```

## 测试

```bash
make integration
make release
```

`tests/integration.sh` 会启动临时管理服务、独立 Nginx、HTTP 目标服务和临时自签名证书，验证 HTTP、HTTPS、配置应用、历史版本及平滑重载。

## 当前限制

- 不支持 32 位 ARMv7。
- 不直接监听 80/443，不申请 root 或 `CAP_NET_BIND_SERVICE`。
- HTTPS 证书目前需要手动导入 PEM，尚未内置 ACME 自动申请和续签。
- 不提供任意原始 Nginx 指令编辑，以避免配置注入和应用无法启动。
- 当前二进制未包含 HTTP/3/QUIC、Brotli、Lua/OpenResty、JWT、headers-more、GeoIP2、ModSecurity/WAF、第三方主动健康检查及 Prometheus 模块；页面不会伪装提供这些功能。
- 客户端 CA 与 Basic Auth 密码文件由用户维护并确保应用运行用户可读。
- 发布前仍需分别在实体 x86_64、ARM64 fnOS 设备上完成安装验收。

## 许可证

nginx-web 源码使用 MIT License。Nginx Open Source 和 ARM64/AMD64 静态构建所含组件的许可证见 `NGINX_LICENSE`、`NOTICE` 与 `THIRD_PARTY_LICENSES.md`。
