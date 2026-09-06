# 从源码构建

[返回首页](../README.md)

先克隆仓库并进入项目目录：

```bash
git clone https://github.com/chenpingonline/nginx-web-fnos.git
cd nginx-web-fnos
```


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

要求：Go 1.26+、Node.js 20.19+ 或 22.12+、npm、tar（GNU 或 macOS BSD tar）、Python 3 和 `file`。

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

`web/dist/` 是本地生成目录，不纳入版本控制。首次克隆或修改前端后，直接运行 `go build`、`go run` 或 `go test` 前须先执行 `make frontend-build`；`make test`、集成测试脚本和 FPK 打包会自动先构建前端。

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


集成与生命周期测试须在对应架构的 Linux 环境运行；macOS 可构建 FPK，但不能直接执行包内 Linux 二进制。`make release` 生成本地安装包和校验文件，不会自动创建 GitHub Release。

## 仓库文件约定

源码、测试、依赖锁文件、正式图标及第三方来源和许可证资料纳入版本控制。前端产物、FPK、构建缓存、本地运行数据及临时设计稿不提交。
