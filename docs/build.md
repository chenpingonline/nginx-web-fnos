# 飞牛打包与核心引用

## 固定版本构建

```bash
make core
make test
make build-all
```

core.lock 保存核心仓库地址、完整提交 SHA 和应用版本。scripts/resolve-core.py 优先复用包含该提交的本地核心仓库，否则克隆 GitHub；它会生成固定提交的独立缓存工作树，拒绝使用被修改的固定源码。

所有通用 Go / Vue / Nginx 源码、静态 Nginx、mime.types 和第三方来源资料均来自核心。此仓库仅有平台专用权限修复 helper，其 Go 依赖单独锁定。

## 本地开发

```bash
NGINX_WEB_CORE=/path/to/nginx-web make frontend-build
NGINX_WEB_CORE=/path/to/nginx-web make build-arm64
```

修改核心仓库后直接构建，不必复制文件。开发构建可使用未提交源码，包中记录 dirty 状态。

## 升级核心

在核心仓库测试、提交并推送后：

```bash
python3 scripts/pin-core.py /path/to/nginx-web
make build-all
```

检查 core.lock 的变更并提交。应用版本唯一来源是核心 VERSION；FPK manifest 从 manifest.template 自动生成。改变核心版本后，无需手工改飞牛 manifest。

## 交付与测试

scripts/build.sh --mode all x86 arm64 创建四个 FPK，并校验架构、权限模式和包结构。每个包记录 CORE_SOURCE.json 和 Nginx 摘要。make release 还运行集成及原生架构的生命周期测试；它不会自动创建 GitHub Release。

生命周期测试必须在隔离 Linux 容器或临时 CI runner 上运行，不能指向已安装的生产应用。包结构校验不代表真实 fnOS 安装/升级通过。

Nginx 重建入口 scripts/build-nginx-on-fnos.sh 转调核心 scripts/build-nginx.sh；输出和二进制维护属于核心仓库。
