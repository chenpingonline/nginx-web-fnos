# nginx-web

nginx-web 是一个面向飞牛 fnOS 的原生 Nginx 反向代理可视化管理应用，FPK 模板集中保存在 `packaging/fnos/`。

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
- 普通 `nginx-web` package 用户运行
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
dist/nginx-web-0.1.1-x86.fpk
dist/nginx-web-0.1.1-arm64.fpk
```

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

nginx-web 源码使用 MIT License。Nginx Open Source 和 ARM64/AMD64 静态构建所含组件的许可证见 `NGINX_LICENSE`、`NOTICE` 与 `THIRD_PARTY_LICENSES.md`。
