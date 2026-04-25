# specctl Architecture Governance Dogfooding Plan

## Objective

Dogfood architecture governance inside this repository with the smallest safe rollout, using existing charter boundaries (`cli`, `mcp`, `skill`, `dashboard`) and current specctl workflows.

Success means we can answer, from governed state alone:

- what components/interfaces exist in each charter
- which constraints are enforced vs advisory
- what changed architecturally in each revision
- where cross-charter coupling is intentional vs accidental

## Current repo shape (baseline)

Existing governed surfaces in `.specs/specctl/CHARTER.yaml`:

- `specctl:cli` (core behavior)
- `specctl:mcp` (transport adapter over cli)
- `specctl:skill` (agent-facing packaged skill)
- `specctl:dashboard` (UI + API surface)

This is a good brownfield target because dependencies already exist (e.g., `mcp` and `skill` depend on `cli`).

## Rollout strategy

### Stage 1 — Bootstrap architecture state (visibility only)

Create charter-local architecture files:

- `.specs/specctl/cli.arch.yaml`
- `.specs/specctl/mcp.arch.yaml`
- `.specs/specctl/skill.arch.yaml`
- `.specs/specctl/dashboard.arch.yaml`

Populate only:

- 3-8 coarse components per charter
- explicit interfaces between those components
- 1-2 high-value constraints per charter (start at `warn`)
- 1-3 accepted decisions capturing existing architecture facts

No blocking checks yet; objective is signal collection.

Use the canonical baseline from [`docs/architecture-linting-and-conventions.md`](./architecture-linting-and-conventions.md) and start by enabling only structural integrity rules as `error`.

### Stage 2 — Integrate into the normal loop

For any delta that crosses boundaries (new dependency edge, new public interface, transport changes):

1. update relevant `*.arch.yaml`
2. run architecture checks in CI (non-blocking)
3. include architecture diff summary in PR description

Continue using normal loop:

```text
context -> delta -> req/spec update -> implementation -> verify/check -> close -> bump
```

Architecture becomes additional evidence, not a separate process.

### Stage 3 — Enable targeted enforcement

Promote only mature, high-confidence rules to `error`:

- layer boundaries in `cli` (e.g., domain cannot depend on infrastructure)
- adapter boundary in `mcp` (transport stays isolated from core semantics)
- contract compatibility for `skill` setup interface

Keep all other checks at `warn` until false positives are low.

Promotion from `warn` to `error` should follow the lint policy maturity guidance in the linting/conventions document.

### Stage 4 — Add cross-charter visibility and selective gates

Add a global cross-charter index:

- `.specs/architecture/CROSS_CHARTER.yaml`

Track only high-value exports/imports first:

- `cli` APIs consumed by `mcp`
- setup/config interfaces consumed by `skill`
- dashboard API dependencies on core governance outputs

Gate only on critical compatibility breaks.

## Proposed first dogfood cycle (2 weeks)

### Week 1: Modeling and read surfaces

- model coarse architecture for all four charters
- run report-only checks in CI
- collect noise/false positives
- produce first architecture snapshot report

### Week 2: Light governance and one enforced rule

- require architecture updates for boundary-changing deltas
- add PR checklist usage for architecture changes
- enforce exactly one `error` constraint in `cli`
- evaluate friction and coverage with team retro

## Candidate charter seeds

### `specctl:cli`

Initial components:

- `cli_surface` (`cmd/specctl`, `internal/cli`)
- `application_core` (`internal/application`)
- `domain_model` (`internal/domain`)
- `infrastructure_adapters` (`internal/infrastructure`)

Initial constraints:

- `domain_model -> infrastructure_adapters = forbidden` (`error` once stable)
- `cli_surface -> domain_model direct calls = warn` (prefer via application layer)

### `specctl:mcp`

Initial components:

- `mcp_server`
- `tool_registry`
- `cli_bridge`

Initial constraints:

- `mcp_server` cannot embed core semantics owned by `cli`
- MCP tool exposure must remain allowlist-driven

### `specctl:skill`

Initial components:

- `skill_doc_surface`
- `setup_script`
- `client_config_adapters`

Initial constraints:

- setup behavior must be idempotent
- guidance must remain aligned with observable CLI/MCP behavior

### `specctl:dashboard`

Initial components:

- `dashboard_cli_entry`
- `dashboard_api`
- `dashboard_spa`
- `governance_projections`

Initial constraints:

- API payload contracts must remain backward-compatible for SPA views
- dashboard read model should not mutate tracking state

## Operational mechanics

For each architecture-affecting PR:

- include charter-local architecture diff
- declare whether cross-charter exports/imports changed
- include check outcomes (`pass | fail | unknown`)
- include exception window (if any): owner, expiry, remediation issue

For each revision bump:

- fail on unresolved `error` architecture violations
- fail on expired exceptions
- allow `warn` with owner attribution

## Metrics for dogfood evaluation

Track for 4-6 weeks:

- architecture update coverage on boundary-changing deltas
- false-positive rate of architecture checks
- number of cross-charter incompatibilities caught pre-merge
- median PR overhead (time/lines) for architecture updates
- count of lingering exceptions older than one revision

## Exit criteria to formalize in product

Promote architecture governance from proposal to shipped behavior when:

1. architecture checks show stable signal (low noise) in this repo
2. at least one useful violation is caught pre-merge
3. PR overhead remains acceptable
4. core team agrees to keep model minimal and typed

At that point, implement the CLI/MCP architecture surfaces incrementally behind the same workflow used here.
