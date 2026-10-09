# Docker 版

Docker 版将 Vue 页面、Go 管理服务和 Nginx 1.30.4 放在同一个镜像中，通过独立管理员账户访问，不需要飞牛桌面或网关。构建支持 `linux/amd64` 和 `linux/arm64`；FPK 构建和网关认证继续使用原有方式。此版本不包含 WAF 实验功能。

Docker 专用文件统一放在本目录：`Dockerfile`、`Dockerfile.dockerignore`、`compose.yaml`、`build-nginx.sh`、`tests/` 和本说明；离线镜像保存在 `docker/dist/`，不提交 Git。Go/Vue 源码继续由项目共享。

除“构建与验证”节另有说明外，下文命令均在 `docker/` 目录执行。从项目根目录先执行 `cd docker`。

## 加载已打包镜像

`dist/` 中的镜像归档可以复制到 NAS 后加载，无需在 NAS 上编译源码。x86_64 设备使用 AMD64 包，ARM64 设备使用 ARM64 包。以 AMD64 为例：

```bash
docker load -i dist/nginx-web-0.3.9-docker-amd64.tar.gz
docker tag nginx-web:docker-amd64 nginx-web:docker
```

ARM64 对应使用 `nginx-web-0.3.9-docker-arm64.tar.gz` 和 `nginx-web:docker-arm64`。同时复制项目的 `compose.yaml`，按下节准备密码文件，然后执行 `docker compose up -d --no-build`。离线包更新时重新加载镜像并执行 `docker compose up -d --no-build --force-recreate`。

## 使用 Compose

在 `docker/` 目录准备首次登录密码，再构建并启动：

```bash
mkdir -p secrets
chmod 700 secrets
openssl rand -base64 24 > secrets/admin_password.txt
chmod 444 secrets/admin_password.txt
docker compose up -d --build
```

打开 `http://NAS地址:8080`，账号为 `admin`，密码保存在 `secrets/admin_password.txt`。用 `cat secrets/admin_password.txt` 在本机查看密码。也可以将文件内容改为自己的密码，长度为 8–72 字节；末尾允许一个换行。Secret 目录为 700，文件为 444，方便容器 UID 10001 读取 Compose 挂载的文件，同时限制宿主其他用户访问目录。

Compose 发布管理端口 `8080` 和代理端口 `80`、`443`、`9080`。没有规则时默认 `9080` 返回 404，这是正常行为。在页面添加 HTTP 规则、设置监听端口为 80、填写域名和容器可达的后端，保存并应用后即可转发。HTTPS 规则需先导入证书或配置 ACME DNS-01。

代理规则的监听端口与容器端口一致。增加端口时同时修改 Compose 的 `ports`；UDP 映射必须带 `/udp`，例如 `5353:5353/udp`。`EXPOSE` 不会自动发布端口。宿主端口已被占用时，可修改映射左侧，例如 `18080:8080`、`9081:80`、`9444:443`。

同一 Docker 网络内的后端可以填服务名；NAS 或局域网服务填容器可达的 IP。容器内的 `127.0.0.1` 指向本容器，不能用于访问宿主。管理端口必须独占，不能同时作为代理规则的监听端口。

默认以 UID/GID `10001:10001` 运行，根文件系统只读，移除全部 capabilities。Compose 设置容器网络命名空间的 `net.ipv4.ip_unprivileged_port_start=0`，允许应用用户监听低端口，无需特权容器或 Docker Socket。此配置适用于 bridge 网络，不适用于 host 网络。

## 数据与升级

命名卷 `nginx-web-data` 挂到 `/data`：

| 路径 | 内容 |
| --- | --- |
| `/data/etc/nginx` | 已应用配置、自定义配置与历史 |
| `/data/var/fnproxy.json` | 配置与未应用草稿 |
| `/data/var/applied-state.json` | 已应用快照 |
| `/data/var/certificates` | 证书与私钥 |
| `/data/var/acme` | ACME 账户、DNS 凭据与续期任务 |
| `/data/var/admin.json` | 管理员账户与 bcrypt 摘要 |
| `/data/var/logs` | Nginx 与管理服务日志 |

启动恢复已应用配置，不自动发布保存的草稿。退出时结束管理请求和后台任务，再停止 Nginx。代理配置有问题时管理页面仍会启动，供检查和修复。健康检查验证管理 HTTP 与 Socket；页面主动停止 Nginx 不影响管理服务健康。

更新代码后执行 `docker compose up -d --build`。`docker compose down` 保留卷；`docker compose down -v` 会删除数据。一个数据卷只应由一个运行中的实例使用。

Compose 默认项目名固定为 `nginx-web-fnos`，与本项目原根目录启动方式一致，目录迁移后仍使用同一命名卷。如果此前通过 `-p` 或 `COMPOSE_PROJECT_NAME` 自定义项目名，迁移后继续使用原项目名；原密码文件和外部挂载也需移到 `docker/`，或改为指向原位置。

页面 JSON 备份可迁移代理、证书和设置，但不包含独立管理员账户、ACME 账户、DNS 凭据或续期任务。完整备份应停止容器后备份整个 `/data` 和外部挂载目录。FPK JSON 备份恢复为草稿，外部路径需改为容器路径后再应用。备份含私钥和代理认证摘要，应妥善保管。

使用宿主目录代替命名卷时，先创建目录并赋予 UID 10001 所有权：

```bash
sudo mkdir -p /path/to/nginx-web-data
sudo chown -R 10001:10001 /path/to/nginx-web-data
```

然后把 Compose 挂载改为 `/path/to/nginx-web-data:/data`。静态目录需要可遍历和可读取；WebDAV 目录还需 UID 10001 的写权限。

## 管理认证与密码重设

首次启动必须提供密码，没有默认密码。只保存 bcrypt 摘要。会话 Cookie 为 HttpOnly、SameSite=Strict，最长 12 小时；退出或容器重启使旧会话失效。写 API 需要会话对应的 CSRF token。飞牛身份 Header 和 `FNPROXY_DEV_ALLOW` 在独立模式中不能绕过认证。代理 Basic Auth 回调只经私有 Unix Socket 调用。

修改密码文件后重建容器：

```bash
docker compose up -d --force-recreate
```

仅执行 `restart` 不会更新被替换的 Secret 文件挂载。可使用 `FNPROXY_ADMIN_PASSWORD` 代替密码文件，但二者不能同时设置。首次初始化后可移除 Compose 密码变量和 Secret 挂载，继续使用保存的摘要。提供新的启动密码会更新摘要。修改 `FNPROXY_ADMIN_USER` 时须同时提供密码。

管理入口通过 HTTPS 反向代理访问时，设置 `FNPROXY_COOKIE_SECURE=1`，让外层代理保留原始 Host。管理服务不信任客户端 `X-Forwarded-*` 或 `X-Trim-*` 身份信息。

## 外部目录与文件

增加挂载和允许访问的目录：

```yaml
environment:
  FNPROXY_ALLOWED_PATHS: /mnt/www:/mnt/auth
volumes:
  - ./www:/mnt/www:ro
  - ./auth:/mnt/auth:ro
```

页面填容器内路径，例如 `/mnt/www` 或 `/mnt/auth/.htpasswd`。只有允许目录内的路径可用于静态站点、外部 Basic Auth 文件和客户端 CA；符号链接不能跳出允许目录。不要把配置数据卷或根目录授权为静态目录。Docker 版不显示飞牛目录选择或访问权限入口。

## 环境变量

| 变量 | 默认值 / 用途 |
| --- | --- |
| `FNPROXY_MODE` | 镜像默认 `standalone`；FPK 默认 `fnos` |
| `FNPROXY_LISTEN` | `:8080`，独立管理 HTTP 地址 |
| `FNPROXY_ADMIN_USER` | 初次默认 `admin`，后续默认保存的账号 |
| `FNPROXY_ADMIN_PASSWORD_FILE` | 初始化 / 重设密码的文件路径 |
| `FNPROXY_ADMIN_PASSWORD` | 密码文件的替代方式 |
| `FNPROXY_COOKIE_SECURE` | 默认 `0`；HTTPS 管理入口设为 `1` |
| `FNPROXY_ALLOWED_PATHS` | 默认空；外部目录以冒号分隔 |

镜像路径默认为 `/data/etc`、`/data/var`、`/tmp/nginx-web`、`/run/nginx-web/app.sock`，通常无需修改对应 `FNPROXY_*` 路径变量。

## 构建与验证

从源码构建需要 Docker BuildKit 和 Buildx 插件，以及 Docker Compose v2。单架构构建默认使用本机架构；跨架构构建 Nginx 需要模拟器或对应架构的构建节点。缺少 Buildx 时请按 [Docker 官方安装说明](https://github.com/docker/buildx#installing) 配置插件。

以下命令在项目根目录执行：

```bash
docker build -f docker/Dockerfile -t nginx-web:docker .
# 或
make docker-build

# 双架构本地 OCI 归档，需要 Buildx
mkdir -p docker/dist
docker buildx build -f docker/Dockerfile --platform linux/amd64,linux/arm64 \
  --output type=oci,dest=docker/dist/nginx-web-multiarch.tar .

python3 docker/tests/integration.py --image nginx-web:docker
```

构建上下文必须是项目根目录，不能使用 `docker/` 作为上下文：`COPY` 的源路径相对于上下文解析。Compose 已配置 `context: ..` 和 `dockerfile: docker/Dockerfile`；手动构建请使用上面的 `-f` 命令。忽略规则使用 Docker 支持的 [Dockerfile 专用忽略文件](https://docs.docker.com/build/concepts/context/#filename-and-location)，排除密码、运行数据和离线镜像。

多阶段构建编译 Vue 页面与静态 Go 程序，从官方 Nginx 1.30.4 源码校验 SHA-256 后编译代理核心，不使用本地 fnOS 二进制。构建上下文排除 Secret、运行数据、构建产物和实验二进制。

构建网络无法访问默认 Go 下载源时可覆盖 `GOPROXY`，例如 `GOPROXY=https://goproxy.cn,direct docker compose up -d --build`，或给 `docker build` 加 `--build-arg GOPROXY=https://goproxy.cn,direct`。依赖仍由 `go.sum` 和 Go 校验机制验证，不关闭摘要检查。

集成测试使用随机宿主端口、独立网络和临时卷，检查登录/CSRF/退出、非 root 身份、HTTP/HTTPS 低端口、TCP/UDP、静态授权、代理 Basic Auth、备份、重启草稿保留、Nginx 停止/启动与健康检查。它需要拉取 `python:3.12-alpine` 作为临时上游，不访问生产代理或 DNS 账户。`--keep` 保留浏览器验证环境，元数据记录临时资源名和密码文件路径，验证后应清理。

UDP 测试从 Docker 的 Linux 宿主网络访问发布端口，避免桌面 Docker 虚拟机的 UDP 转发限制。ARM64 主机上的 AMD64 模拟运行不能替代原生 AMD64 设备验证。

构建与 Secret 挂载方式参考 [Docker 多平台构建文档](https://docs.docker.com/build/building/multi-platform/) 和 [Compose Secrets 文档](https://docs.docker.com/compose/how-tos/use-secrets/)。
