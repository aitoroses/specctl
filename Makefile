.PHONY: all build build-all install test test-go dashboard-install dashboard-build dashboard-typecheck dashboard-dev clean

all: build

build-all: build dashboard-build

build:
	go build -o specctl ./cmd/specctl

install:
	go install ./cmd/specctl

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
