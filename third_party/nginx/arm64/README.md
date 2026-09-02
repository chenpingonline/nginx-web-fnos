# ARM64 official NGINX source build

The executable is intentionally not committed. `scripts/fetch-nginx.sh arm64`
downloads the pinned official NGINX 1.30.4 source archive from nginx.org,
verifies SHA-256, and invokes `scripts/build-nginx.sh` to compile a static Linux
AArch64 binary without third-party NGINX modules.
