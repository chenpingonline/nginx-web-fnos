PERMISSION_MODE ?= standard

.PHONY: core frontend-install frontend-typecheck frontend-build test build-x86 build-arm64 build-all integration release clean
core:
	@python3 scripts/resolve-core.py
frontend-install:
	@CORE="$$(python3 scripts/resolve-core.py)"; $(MAKE) -C "$$CORE" frontend-install
frontend-typecheck:
	@CORE="$$(python3 scripts/resolve-core.py)"; $(MAKE) -C "$$CORE" frontend-typecheck
frontend-build:
	@CORE="$$(python3 scripts/resolve-core.py)"; $(MAKE) -C "$$CORE" FRONTEND_MODE=fnos PERMISSION_MODE=$(PERMISSION_MODE) frontend-build
test:
	@CORE="$$(python3 scripts/resolve-core.py)"; $(MAKE) -C "$$CORE" FRONTEND_MODE=fnos PERMISSION_MODE=$(PERMISSION_MODE) test
	./tests/install-restore.sh
build-x86:
	./scripts/build.sh --mode $(PERMISSION_MODE) x86
build-arm64:
	./scripts/build.sh --mode $(PERMISSION_MODE) arm64
build-all:
	./scripts/build.sh --mode all x86 arm64
integration:
	./tests/integration.sh
release:
	./scripts/release.sh
clean:
	rm -rf .build .build-* .cache dist
