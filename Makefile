.PHONY: frontend-install frontend-typecheck frontend-build test build-x86 build-arm64 build-all integration release clean

frontend-install:
	npm --prefix web ci

frontend-typecheck:
	npm --prefix web run typecheck

frontend-build:
	npm --prefix web run build

test: frontend-typecheck
	go test ./...

build-x86:
	./scripts/build.sh x86

build-arm64:
	./scripts/build.sh arm64

build-all: build-x86 build-arm64

integration:
	./tests/integration.sh

release:
	./scripts/release.sh

clean:
	rm -rf .build .build-* .cache dist .fnproxy-dev
