.PHONY: all build build-all install test test-go dashboard-install dashboard-build dashboard-typecheck dashboard-dev clean

# VERSION is embedded into the binary via -ldflags so unexpected-error
# envelopes (and the report_issue hint) include the actual build, not "dev".
# Override at the command line: `make build VERSION=v1.2.3`.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/aitoroses/specctl/internal/presenter.BuildVersion=$(VERSION)

all: build

build-all: build dashboard-build

build:
	go build -ldflags "$(LDFLAGS)" -o specctl ./cmd/specctl

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/specctl

test: test-go dashboard-typecheck

test-go:
	go test ./...

dashboard-install:
	cd dashboard && pnpm install --frozen-lockfile

dashboard-build: dashboard-install
	cd dashboard && pnpm build

dashboard-typecheck: dashboard-install
	cd dashboard && pnpm exec tsc --noEmit

dashboard-dev:
	cd dashboard && pnpm dev

clean:
	rm -f specctl application.test coverage.out
	rm -rf dashboard/dist dashboard/node_modules
