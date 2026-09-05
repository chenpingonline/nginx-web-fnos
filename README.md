# nginx-web

nginx-web 是一个面向飞牛 fnOS 的原生 Nginx 反向代理可视化管理应用，FPK 模板集中保存在 `packaging/fnos/`。

它自带独立的 Nginx Open Source 1.30.4，不读取、不修改、也不会重启飞牛系统 Nginx；不依赖 Docker，管理后台通过 fnOS 统一网关和 Unix Socket 提供。

## 页面功能总览

管理页面使用 Vue 3、TypeScript 和 Vite，共有九个主要页面。所有配置都通过结构化表单生成，不允许直接提交任意 Nginx 指令。

### 总览

- 浅色绿色总览：独立运行状态卡展示 Nginx 运行时长、进程 ID、工作进程数、活动连接与请求/秒。运行时长及工作进程数由 Linux 实际进程信息提供，无法读取时显示缺失。
- 紧凑指标栏展示 HTTP 请求速率、响应速率、当前连接数（含空闲 Keepalive）、完成请求及错误率（4xx + 5xx），没有可靠带宽数据时不显示带宽。
- 折线图支持最近 15 分钟、1 小时、5 小时、1 天、7 天、1 月（最近 30 天），请求与响应曲线叠加显示；上方短横线图例可分别显隐曲线，支持悬停及键盘查看同一时刻的多项数据。
- 点击“错误率 →”进入详情，延续所选时段与规则，分别查看总错误率、4xx、5xx 趋势及受影响规则。
- 规则表格展示入口、监听地址、转发目标、配置状态、时段平均请求速率和错误率；支持搜索、协议与配置状态筛选、请求数/错误数排序和分页。名称打开该规则的响应趋势，错误率箭头打开该规则的错误详情。
- 明确区分已生效、待应用、已停用及待删除的规则。草稿中删除但尚未应用的规则仍保留在首页，不把“启用”视为服务健康检测结果。
- 检查已生效配置和启用草稿引用的 HTTPS / Stream TLS 证书，提前 30 天提醒到期，并展示受影响规则；已过期或尚未生效单独提醒。
- 快速添加代理、校验配置、应用草稿、启动或平滑重载 Nginx。

#### 首页统计的数据范围

- Go 服务每 5 秒采样，按分钟汇总并保存最近 30 天；页面关闭后继续采集。长时间范围自动汇总曲线，未采集到的历史保留为空；升级前已过期的历史无法恢复。历史与日志读取位置每分钟原子保存，正常退出时再次保存。
- HTTP 连接与累计请求由应用私有 Unix Socket 上的 `stub_status` 提供，没有新增公网统计端口；采集请求自身从请求速率和连接数中扣除。
- 响应速率来自最近一次连续日志采样间隔的已完成请求数，趋势按分钟覆盖时间汇总。首次采样、日志关闭或采集中断时显示缺失，避免把补读历史当作当前响应速率。
- 规则归属和错误率来自独立的 `http-metrics.log`，固定 JSON 格式包含时间、规则 ID、状态码和响应体字节数，不记录 URL、客户端 IP 或凭据，不受自定义访问日志格式影响。
- 错误率为 400–599 状态码请求数除以已完成请求数；详情分别展示 4xx（请求错误）与 5xx（服务端错误）。旧历史没有采集过的 4xx 保持未知，相关总错误率显示空白，既有 5xx 历史继续保留。
- 关闭访问日志时，同时停止规则日志统计；HTTP 状态采样继续。统计日志复用日志大小与保留数量设置参与轮转。
- 升级已有安装后，需要保存并应用一次配置才能启用新的统计入口。此操作也会应用当前草稿，首页会明确提醒。
- 没有历史数据或采集中断时保留空白，不补造历史、不将缺失值显示成零；显示采样时间与所选时段的日志覆盖时间。异常退出后的已保存数据与日志位置一起恢复；已被清理的未处理日志无法恢复，会提示统计不完整。
- HTTP 日志在请求结束后记录，长下载、WebSocket、SSE 尚未结束的请求不会提前计入完成请求与错误率。规则趋势是已完成请求的速率，与全局接收请求速率口径不同。
- TCP/UDP 首版展示配置与转发目标，其会话指标不混入 HTTP 请求数。P95 耗时、即时带宽及主动健康探测不属于这一版统计范围。

### HTTP/HTTPS 代理规则

- 创建、编辑、启用、停用、搜索和删除代理规则。
- 按 HTTP/HTTPS 协议、启用状态与名称/域名/端口/目标组合筛选，显示匹配数量并支持一键重置。
- 配置规则名称、一个或多个域名/IP、监听端口及 `*` 默认站点。
- 配置 HTTP 或 HTTPS 入口、手动选择证书及 HTTP/2。
- 使用单个 HTTP/HTTPS 后端服务，或选择可复用的 HTTP 后端服务组。
- 配置后端服务 TLS 证书校验、Host 保留、WebSocket、SSE/流式传输、请求体大小及连接/读取/发送超时。
- 按客户端 IP 限制每秒请求数、突发请求、并发连接数和下载速度。
- 为根路径和额外 Location 分别选择前缀、精确或正则匹配，并为每个路径配置不同处理方式。
- Location 后端支持 HTTP 反向代理、静态文件、固定返回/跳转、gRPC、FastCGI、uWSGI、SCGI、Memcached 和 Stub Status。
- 静态文件支持 `root`/`alias`、Index、目录浏览、Expires 和 Try Files。
- 支持 HTTP 跳转 HTTPS，以及 `last`、`break`、临时跳转和永久跳转 Rewrite。
- 支持代理缓存区、磁盘上限、未访问失效、响应有效期、自定义缓存 Key、变量绕过缓存、故障使用过期缓存和大文件 Slice。
- 支持 IP/CIDR 允许与拒绝、Basic Auth、Auth Request、Secure Link、Referer 防盗链，以及静态 Location 的有限 WebDAV。
- 支持添加、覆盖或清空后端服务请求 Header，以及通过原生 `add_header` 添加响应 Header。
- 支持 Sub Filter 内容替换、Addition 响应前后追加、Mirror 请求镜像和 SSI。

### TCP/UDP 代理

- 创建、编辑、启用、停用和删除 TCP/UDP 四层代理规则。
- 支持 TCP/UDP 协议与启用状态组合筛选，按名称、监听地址、端口、目标服务或 SNI 域名搜索，显示匹配数量并支持一键重置。
- 配置监听地址、监听端口、单个后端服务或 Stream 后端服务组。
- 配置连接超时、会话超时和 UDP 响应次数。
- 支持入口接收和向后端服务发送 PROXY Protocol，并配置可信代理地址。
- TCP 支持关闭 TLS、TLS 终止和 TLS SNI 透传；TLS 终止可选择已导入证书。
- SNI 透传可按多个域名分流到不同单节点后端服务或 Stream 后端服务组。
- 支持 Stream 访问日志、单 IP 最大连接数及 IP/CIDR 允许与拒绝。
- 适用于 SSH、数据库、MQTT、游戏服务和 HTTPS 四层透传等场景。

### 后端服务组

- 分别创建供 HTTP/HTTPS 或 TCP/UDP 使用的后端服务组，并在多个规则间复用。
- 管理多个服务器节点的主机、端口、权重、最大失败次数、故障恢复时间、备份和停用状态。
- HTTP 池支持 Round Robin、Least Connections、IP Hash、Hash 和 Random Two Least Connections。
- Stream 池支持 Round Robin、Least Connections、Hash 和 Random Two Least Connections。
- 配置 Keepalive 数量、单连接最大请求数、单连接最长时间和空闲超时。
- 删除前检查规则引用，避免留下无效配置。

### SSL/TLS 证书

- 支持上传 PEM 证书链与私钥文件、从服务器绝对路径导入或粘贴 PEM，并校验证书与私钥是否匹配。
- 路径导入会将证书复制到应用目录，源文件更新后需要重新导入。
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
- 校验重复域名、端口冲突、证书/后端服务组引用、IP/CIDR、路径和指令参数范围。
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

版本号只需修改 `packaging/fnos/manifest` 的 `version` 字段（格式为 `主版本.次版本.补丁版本`）。后端通过 Go embed 读取，前端在 Vite 启动或构建时读取，打包和发布脚本也从此文件读取；不再使用 `VERSION` 环境变量覆盖。改版本后重新构建，开发预览需重启 Vite。

输出：

```text
dist/nginx-web-<version>-x86.fpk
dist/nginx-web-<version>-arm64.fpk
```

## 测试

```bash
make integration
make release
```

`tests/integration.sh` 会启动临时管理服务、独立 Nginx、HTTP 后端服务和临时自签名证书，验证 HTTP、HTTPS、配置应用、历史版本及平滑重载。

## 当前限制

- 不支持 32 位 ARMv7。
- 不直接监听 80/443，不申请 root 或 `CAP_NET_BIND_SERVICE`。
- SSL/TLS 证书目前需要手动导入 PEM，尚未内置 ACME 自动申请和续签。
- 不提供任意原始 Nginx 指令编辑，以避免配置注入和应用无法启动。
- 当前二进制未包含 HTTP/3/QUIC、Brotli、Lua/OpenResty、JWT、headers-more、GeoIP2、ModSecurity/WAF、第三方主动健康检查及 Prometheus 模块；页面不会伪装提供这些功能。
- 客户端 CA 与 Basic Auth 密码文件由用户维护并确保应用运行用户可读。
- 发布前仍需分别在实体 x86_64、ARM64 fnOS 设备上完成安装验收。

## 许可证

nginx-web 源码使用 MIT License。Nginx Open Source 和 ARM64/AMD64 静态构建所含组件的许可证见 `NGINX_LICENSE`、`NOTICE` 与 `THIRD_PARTY_LICENSES.md`。

### 自动主题

应用通过飞牛官方 `@trimjs/web-app` SDK 读取平台主题并监听 `os/theme`，无需手动设置。暗色侧栏与外框为 `#0C0C0D`，主内容区独立滚动，四周留边。平台主题 API 要求 fnOS 1.2.0401 / App 1.34.0 及以上；旧系统、独立浏览器或 SDK 不可用时跟随浏览器 `prefers-color-scheme`。移动 App 的 SDK 不支持主题变更事件，重新打开页面时读取当前主题。
