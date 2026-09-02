# AMD64 官方 NGINX 二进制

这个目录保存 AMD64 FPK 使用的 NGINX 来源记录。本地二进制不会提交到 Git。

## 文件说明

| 文件 | 用途 |
| --- | --- |
| `nginx` | AMD64 静态链接的 NGINX 1.30.4 可执行文件，参与 FPK 打包但被 Git 忽略 |
| `SOURCES.txt` | 记录官方源码下载地址、文件大小、版本和 SHA-256 |
| `SHA256SUMS.txt` | 记录官方源码压缩包的 SHA-256，用于来源审计 |
| `README.md` | 当前目录的使用说明，不参与应用运行 |

## 准备二进制

在 AMD64 fnOS 主机上执行：

```bash
./scripts/build-nginx-on-fnos.sh
```

脚本默认生成：

```text
nginx-amd64-output/nginx-1.30.4-x86_64-linux
```

将它取回项目并复制到当前目录，文件名必须是 `nginx`：

```bash
cp nginx-1.30.4-x86_64-linux third_party/nginx/x86_64/nginx
```

然后构建 AMD64 FPK：

```bash
./scripts/build.sh x86
```

打包过程不会下载或编译 NGINX，只会读取当前目录的 `nginx`，并检查其架构、静态链接属性和版本。
