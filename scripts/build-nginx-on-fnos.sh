#!/usr/bin/env bash
set -euo pipefail

NGINX_VERSION="${NGINX_VERSION:-1.30.4}"
NGINX_SOURCE_SHA256="${NGINX_SOURCE_SHA256:-4261dc90e9e47c1c4041276e9aaa3d48ebe2e664f728e14fa95ae6c67d57a08b}"
NGINX_SOURCE_URL="https://nginx.org/download/nginx-${NGINX_VERSION}.tar.gz"
BUILD_IMAGE="${NGINX_BUILD_IMAGE:-alpine:3.21}"
APK_MIRROR="${NGINX_APK_MIRROR:-https://mirrors.aliyun.com/alpine}"

die() {
  echo "错误：$*" >&2
  exit 1
}

case "$(uname -m)" in
  aarch64|arm64)
    TARGET_ARCH="arm64"
    PLATFORM="linux/arm64"
    ARCH_LABEL="ARM64"
    OUTPUT_ARCH="aarch64"
    FILE_PATTERN='ARM aarch64|ARM64|aarch64'
    ;;
  x86_64|amd64)
    TARGET_ARCH="amd64"
    PLATFORM="linux/amd64"
    ARCH_LABEL="AMD64"
    OUTPUT_ARCH="x86_64"
    FILE_PATTERN='x86-64|x86_64'
    ;;
  *) die "仅支持 ARM64 和 AMD64 主机，当前架构：$(uname -m)" ;;
esac

OUTPUT_DIR="${1:-$PWD/nginx-${TARGET_ARCH}-output}"
OUTPUT_NAME="nginx-${NGINX_VERSION}-${OUTPUT_ARCH}-linux"

for command_name in curl tar sha256sum file; do
  command -v "$command_name" >/dev/null 2>&1 || die "缺少命令 $command_name"
done

if docker info >/dev/null 2>&1; then
  DOCKER=(docker)
elif command -v sudo >/dev/null 2>&1 && sudo docker info >/dev/null 2>&1; then
  DOCKER=(sudo docker)
else
  die "Docker 不可用，请先在 fnOS 安装并启动 Docker"
fi

mkdir -p "$OUTPUT_DIR"
OUTPUT_DIR="$(cd "$OUTPUT_DIR" && pwd)"
WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/nginx-official-${TARGET_ARCH}.XXXXXX")"
trap 'rm -rf "$WORK_DIR"' EXIT

SOURCE_ARCHIVE="$WORK_DIR/nginx-${NGINX_VERSION}.tar.gz"
echo "[1/6] 下载 NGINX 官方源码：$NGINX_SOURCE_URL"
curl -fL --retry 3 --retry-delay 2 --connect-timeout 20 \
  "$NGINX_SOURCE_URL" -o "$SOURCE_ARCHIVE"

echo "[2/6] 校验官方源码 SHA-256"
ACTUAL_SOURCE_SHA256="$(sha256sum "$SOURCE_ARCHIVE" | awk '{print $1}')"
[[ "$ACTUAL_SOURCE_SHA256" == "$NGINX_SOURCE_SHA256" ]] || {
  echo "期望：$NGINX_SOURCE_SHA256" >&2
  echo "实际：$ACTUAL_SOURCE_SHA256" >&2
  die "源码摘要不匹配，已停止编译"
}

mkdir -p "$WORK_DIR/src" "$WORK_DIR/out"
tar -xzf "$SOURCE_ARCHIVE" -C "$WORK_DIR/src"

echo "[3/6] 准备 $ARCH_LABEL Alpine 编译环境：$BUILD_IMAGE"
"${DOCKER[@]}" pull --platform "$PLATFORM" "$BUILD_IMAGE"

echo "[4/6] 编译官方 NGINX ${NGINX_VERSION} $ARCH_LABEL 静态二进制"
"${DOCKER[@]}" run --rm --platform "$PLATFORM" \
  -e APK_MIRROR="$APK_MIRROR" \
  -e HOST_UID="$(id -u)" \
  -e HOST_GID="$(id -g)" \
  -v "$WORK_DIR:/work" \
  -w "/work/src/nginx-${NGINX_VERSION}" \
  "$BUILD_IMAGE" sh -euxc '
    if [ -n "$APK_MIRROR" ]; then
      printf "%s/v3.21/main\n%s/v3.21/community\n" "$APK_MIRROR" "$APK_MIRROR" > /etc/apk/repositories
    fi
    apk add --no-cache \
      build-base linux-headers \
      openssl-dev openssl-libs-static \
      pcre2-dev \
      zlib-dev zlib-static
    ./configure \
      --prefix=.. \
      --conf-path=nginx.conf \
      --pid-path=run/nginx.pid \
      --lock-path=run/nginx.lock \
      --http-log-path=logs/access.log \
      --error-log-path=logs/error.log \
      --http-client-body-temp-path=temp/body \
      --http-proxy-temp-path=temp/proxy \
      --http-fastcgi-temp-path=temp/fastcgi \
      --http-scgi-temp-path=temp/scgi \
      --http-uwsgi-temp-path=temp/uwsgi \
      --with-cc-opt="-Os -static" \
      --with-ld-opt="-static" \
      --with-pcre-jit \
      --with-threads \
      --with-file-aio \
      --with-http_ssl_module \
      --with-http_v2_module \
      --with-http_realip_module \
      --with-http_stub_status_module \
      --with-http_auth_request_module \
      --with-http_addition_module \
      --with-http_sub_module \
      --with-http_dav_module \
      --with-http_gunzip_module \
      --with-http_gzip_static_module \
      --with-http_secure_link_module \
      --with-http_slice_module \
      --with-stream \
      --with-stream_ssl_module \
      --with-stream_ssl_preread_module \
      --with-stream_realip_module
    make -j"$(getconf _NPROCESSORS_ONLN)"
    strip objs/nginx
    install -m 755 objs/nginx /work/out/nginx
    ./objs/nginx -V 2> /work/out/nginx-build-info.txt
    chown "$HOST_UID:$HOST_GID" /work/out/nginx /work/out/nginx-build-info.txt
  '

echo "[5/6] 导出编译产物"
install -m 755 "$WORK_DIR/out/nginx" "$OUTPUT_DIR/$OUTPUT_NAME"
{
  cat "$WORK_DIR/out/nginx-build-info.txt"
  printf '\nsource_url=%s\nsource_sha256=%s\nbuild_image=%s\napk_mirror=%s\nplatform=%s\n' \
    "$NGINX_SOURCE_URL" "$NGINX_SOURCE_SHA256" "$BUILD_IMAGE" "$APK_MIRROR" "$PLATFORM"
} > "$OUTPUT_DIR/${OUTPUT_NAME}.build-info.txt"
(cd "$OUTPUT_DIR" && sha256sum "$OUTPUT_NAME" > "${OUTPUT_NAME}.sha256")

echo "[6/6] 验证 $ARCH_LABEL、静态链接、版本和第三方模块"
file "$OUTPUT_DIR/$OUTPUT_NAME" | grep -Eq "$FILE_PATTERN" || die "产物不是 $ARCH_LABEL"
file "$OUTPUT_DIR/$OUTPUT_NAME" | grep -Fq 'statically linked' || die "产物不是静态链接"
grep -aFq "nginx version: nginx/${NGINX_VERSION}" "$OUTPUT_DIR/$OUTPUT_NAME" || die "版本字符串不正确"
if grep -aEq 'nginx-auth-jwt|nginx-keyval|echo-nginx-module|headers-more-nginx-module|set-misc-nginx-module' "$OUTPUT_DIR/$OUTPUT_NAME"; then
  die "产物中发现非官方第三方 NGINX 模块"
fi

echo
echo "编译完成："
echo "  二进制：$OUTPUT_DIR/$OUTPUT_NAME"
echo "  摘要：  $OUTPUT_DIR/${OUTPUT_NAME}.sha256"
echo "  信息：  $OUTPUT_DIR/${OUTPUT_NAME}.build-info.txt"
