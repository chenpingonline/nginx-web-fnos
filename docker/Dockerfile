# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN npm --prefix web ci
COPY packaging/fnos/manifest ./packaging/fnos/manifest
COPY web ./web
RUN FNPROXY_FRONTEND_MODE=standalone FNPROXY_PERMISSION_MODE=full-ports npm --prefix web run build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY ["cmd/nginx-web/", "/src/cmd/nginx-web/"]
COPY internal ./internal
COPY packaging/fnos/manifest packaging/fnos/version.go ./packaging/fnos/
COPY web/embed.go ./web/embed.go
COPY --from=frontend /src/web/dist ./web/dist
ARG TARGETOS
ARG TARGETARCH
ARG GO_BUILD_PARALLELISM=1
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOGC=50 GOMEMLIMIT=512MiB go build -p $GO_BUILD_PARALLELISM -tags full_ports -trimpath -buildvcs=false -ldflags='-s -w' -o /out/nginx-web-server ./cmd/nginx-web

# Build the official source for the target architecture. No local fnOS binary
# or experimental WAF libraries can enter the Docker build context.
FROM alpine:3.21 AS nginx-build
RUN apk add --no-cache build-base linux-headers openssl-dev pcre2-dev zlib-dev curl
WORKDIR /build
COPY docker/build-nginx.sh ./build-nginx.sh
RUN sh ./build-nginx.sh

FROM alpine:3.21 AS runtime
RUN apk add --no-cache ca-certificates openssl pcre2 zlib tzdata tini \
    && addgroup -g 10001 nginx-web && adduser -D -H -u 10001 -G nginx-web nginx-web \
    && mkdir -p /opt/nginx-web/bin /opt/nginx-web/etc /opt/nginx-web/licenses /data/etc /data/var /run/nginx-web \
    && chown -R 10001:10001 /data /run/nginx-web
COPY --from=backend /out/nginx-web-server /opt/nginx-web/bin/nginx-web-server
COPY --from=nginx-build /out/nginx /opt/nginx-web/bin/nginx
COPY --from=nginx-build /out/nginx-build-info.txt /opt/nginx-web/licenses/nginx-build-info.txt
COPY --from=nginx-build /out/NGINX-LICENSE /opt/nginx-web/licenses/NGINX-LICENSE
COPY third_party/nginx/mime.types /opt/nginx-web/etc/mime.types
COPY LICENSE THIRD_PARTY_LICENSES.md /opt/nginx-web/licenses/
ENV FNPROXY_MODE=standalone FNPROXY_LISTEN=:8080 \
    FNPROXY_APPDEST=/opt/nginx-web FNPROXY_ETC=/data/etc FNPROXY_VAR=/data/var \
    FNPROXY_TMP=/tmp/nginx-web FNPROXY_SOCKET=/run/nginx-web/app.sock
LABEL org.opencontainers.image.title="nginx-web" \
      org.opencontainers.image.description="Standalone Nginx proxy manager" \
      org.opencontainers.image.source="https://github.com/chenpingonline/nginx-web-fnos" \
      org.opencontainers.image.licenses="GPL-3.0-only"
USER 10001:10001
WORKDIR /opt/nginx-web
VOLUME ["/data"]
EXPOSE 8080/tcp 9080/tcp 9443/tcp
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD ["/opt/nginx-web/bin/nginx-web-server", "healthcheck"]
ENTRYPOINT ["/sbin/tini", "--", "/opt/nginx-web/bin/nginx-web-server"]
CMD ["serve"]
