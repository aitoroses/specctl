# specctl Extraction Checklist

Use this when moving `tools/specctl/` into its own private repository.

## Repository root setup

- [ ] Add `LICENSE` (or explicitly mark the repo private/proprietary)
- [ ] Add `.github/workflows/ci.yml`
- [ ] Add `.github/CODEOWNERS` if needed
- [ ] Add branch protection / required checks
- [ ] Choose release/versioning policy

## Files to keep

- [ ] `cmd/`
- [ ] `internal/`
- [ ] `dashboard/`
- [ ] `.specs/`
- [ ] `skills/specctl/`
- [ ] `test/`
- [ ] `testdata/`
- [ ] `SPEC.md`
- [ ] `internal/mcp/SPEC.md`
- [ ] `SPEC_FORMAT.md`
- [ ] `KNOWN_PROBLEMS.md`
- [ ] `README.md`
- [ ] `ARCHITECTURE.md`
- [ ] `CONTRIBUTING.md`
- [ ] `CHANGELOG.md`
- [ ] `Makefile`
- [ ] `go.mod`, `go.sum`

## Files to exclude from the new repo

- [ ] build artifacts (`specctl`, `application.test`)
- [ ] local runtime state (`.omc/`)
- [ ] dashboard install/build artifacts (`dashboard/node_modules`, `dashboard/dist`)

## Follow-up boundary cleanup

- [ ] Decide final ownership for `internal/cli/mcp.go`
- [ ] Decide whether dashboard remains first-class or optional
- [ ] Re-check spec scopes after extraction to remove any leftover monorepo assumptions

## Recommended first CI checks

- [ ] `go test ./...`
- [ ] `pnpm --dir dashboard run typecheck`
- [ ] `pnpm --dir dashboard run build`
- [ ] `make build`

## Release readiness

- [ ] Confirm install instructions in `README.md`
- [ ] Confirm MCP startup instructions
- [ ] Confirm self-governance docs still point to valid paths
- [ ] Confirm spec files open/close/rev-bump cleanly in the new repo root
