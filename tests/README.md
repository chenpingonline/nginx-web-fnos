# 飞牛测试

- make test：通过核心仓库运行 Go/Vue 测试，并验证本地恢复诊断。
- tests/fpk-lifecycle.sh <file.fpk>：隔离 Linux 中运行安装、启动、停止、保留恢复和删除测试。
- tests/low-ports.sh <server> <helper>：Linux 临时测试环境中的低端口权限与生命周期。
- tests/repair-app-data.sh <helper>：Linux root 临时目录中的权限修复与链接隔离。

业务功能、Docker 和 Linux 服务测试位于 nginx-web 核心仓库。
