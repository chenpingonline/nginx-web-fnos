#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARCH="${1:-arm64}"
DEST="${2:-$ROOT/dist/nginx-1.30.4-aarch64-linux}"
VERSION="${NGINX_VERSION:-1.30.4}"
SOURCE_SHA="4261dc90e9e47c1c4041276e9aaa3d48ebe2e664f728e14fa95ae6c67d57a08b"
SOURCE_URL="https://nginx.org/download/nginx-${VERSION}.tar.gz"
BUILD_IMAGE="${NGINX_BUILD_IMAGE:-alpine:3.21}"
APK_REPOSITORY="${NGINX_APK_REPOSITORY:-}"

case "$ARCH" in
  arm|arm64|aarch64) PLATFORM="linux/arm64"; OUTPUT_ARCH="aarch64" ;;
  *) echo "官方源码构建目前仅支持 arm64" >&2; exit 1 ;;
esac

for cmd in curl docker tar; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "缺少 $cmd" >&2; exit 1; }
done

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}

CACHE_DIR="$ROOT/.cache/nginx/source"
ARCHIVE="$CACHE_DIR/nginx-${VERSION}.tar.gz"
WORK="$ROOT/.build/nginx-official-$OUTPUT_ARCH"
mkdir -p "$CACHE_DIR" "$(dirname "$DEST")"

if [[ ! -f "$ARCHIVE" ]]; then
  curl -fL --retry 3 --retry-delay 2 --connect-timeout 20 "$SOURCE_URL" -o "$ARCHIVE.tmp"
  mv "$ARCHIVE.tmp" "$ARCHIVE"
fi
ACTUAL_SHA="$(sha256_file "$ARCHIVE")"
[[ "$ACTUAL_SHA" == "$SOURCE_SHA" ]] || {
  echo "Nginx 官方源码包 SHA-256 校验失败" >&2
  echo "期望：$SOURCE_SHA" >&2
  echo "实际：$ACTUAL_SHA" >&2
  exit 1
}

rm -rf "$WORK"
mkdir -p "$WORK/src" "$WORK/out"
tar -xzf "$ARCHIVE" -C "$WORK/src"

docker run --rm --platform "$PLATFORM" \
  -e APK_REPOSITORY="$APK_REPOSITORY" \
  -v "$WORK:/work" \
  -w "/work/src/nginx-${VERSION}" \
  "$BUILD_IMAGE" sh -euxc '
    if [ -n "$APK_REPOSITORY" ]; then
      printf "%s/v3.21/main\n%s/v3.21/community\n" "$APK_REPOSITORY" "$APK_REPOSITORY" > /etc/apk/repositories
    fi
    apk add --no-cache build-base linux-headers openssl-dev openssl-libs-static pcre2-dev zlib-dev zlib-static
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
      --with-http_ssl_module \
      --with-http_v2_module \
      --with-http_realip_module \
      --with-http_stub_status_module \
      --with-http_auth_request_module
    make -j"$(getconf _NPROCESSORS_ONLN)"
    strip objs/nginx
    install -m 755 objs/nginx /work/out/nginx
    ./objs/nginx -V 2> /work/out/nginx-version.txt
  '

install -m 755 "$WORK/out/nginx" "$DEST"
cp "$WORK/out/nginx-version.txt" "${DEST}.build-info"
printf 'source_url=%s\nsource_sha256=%s\nbuild_image=%s\n' "$SOURCE_URL" "$SOURCE_SHA" "$BUILD_IMAGE" >> "${DEST}.build-info"
echo "$DEST"
