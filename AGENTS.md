# 飞牛打包仓库

- 本仓库只维护 fnOS 清单、宿主集成、生命周期、权限修复和 FPK 打包。
- 通用 Go/Vue/Nginx 功能只修改核心仓库 `https://github.com/chenpingonline/nginx-web`。本地通常位于 `/Users/chenping/Project/codex/nginx-web`。
- 禁止在本仓库重新引入核心后端、前端或 Docker 副本。
- 正式构建使用 `core.lock` 固定的核心提交；本地开发使用 `NGINX_WEB_CORE=/path/to/nginx-web`。更新核心引用通过 `scripts/pin-core.py`。
- 应用版本从核心 `VERSION` 生成 manifest；不要手工维护第二份版本号。
- 验证 FPK 架构、权限模式、生命周期及核心来源。不能以包结构校验代替真实 fnOS 安装验证。
