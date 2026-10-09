#!/bin/sh
set -eu
version=1.30.4
checksum=4261dc90e9e47c1c4041276e9aaa3d48ebe2e664f728e14fa95ae6c67d57a08b
curl -fL --retry 3 --connect-timeout 20 "https://nginx.org/download/nginx-${version}.tar.gz" -o nginx.tar.gz
printf '%s  nginx.tar.gz\n' "$checksum" | sha256sum -c -
tar -xzf nginx.tar.gz
cd "nginx-${version}"
./configure \
  --prefix=.. --conf-path=nginx.conf --pid-path=run/nginx.pid --lock-path=run/nginx.lock \
  --http-log-path=logs/access.log --error-log-path=logs/error.log \
  --http-client-body-temp-path=temp/body --http-proxy-temp-path=temp/proxy \
  --http-fastcgi-temp-path=temp/fastcgi --http-scgi-temp-path=temp/scgi --http-uwsgi-temp-path=temp/uwsgi \
  --with-cc-opt=-Os --with-pcre-jit --with-threads --with-file-aio \
  --with-http_ssl_module --with-http_v2_module --with-http_realip_module \
  --with-http_stub_status_module --with-http_auth_request_module --with-http_addition_module \
  --with-http_sub_module --with-http_dav_module --with-http_gunzip_module \
  --with-http_gzip_static_module --with-http_secure_link_module --with-http_slice_module \
  --with-stream --with-stream_ssl_module --with-stream_ssl_preread_module --with-stream_realip_module
make -j2
mkdir -p /out
strip objs/nginx
cp objs/nginx /out/nginx
cp LICENSE /out/NGINX-LICENSE
./objs/nginx -V 2>/out/nginx-build-info.txt
printf '\nsource=https://nginx.org/download/nginx-%s.tar.gz\nsha256=%s\n' "$version" "$checksum" >>/out/nginx-build-info.txt
