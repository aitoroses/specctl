# Architecture Governance Extension Ideas for specctl

## Problem statement

`specctl` already governs behavioral intent and verification, but architecture intent is still mostly implicit in prose and code structure. That creates a gap for agents that need to reason continuously about:

- system boundaries
- component responsibilities
- allowed dependencies and data flow
- architecture-level tradeoffs over time

The goal of this proposal is to add a low-entropy architecture layer that is explicit, diffable, and enforceable without creating heavy documentation burden.

## Design principles (low-entropy first)

1. **Structure over narrative**: capture decisions in compact typed fields, not long freeform text.
2. **Incremental mutation**: architecture state changes through commands, similar to requirements.
3. **Context-first reads**: architecture state should appear in `context` output as first-class guidance.
4. **Policy and evidence together**: each architecture claim should tie to checks, traces, or known exceptions.
5. **Few stable primitives**: keep the model small so agents can reason reliably.

## Proposed architecture model

Add a new charter-level artifact:

```text
.specs/<charter>/ARCH.yaml
```

with four primitives:

- **components**: unique ID, responsibility, owner scope
- **interfaces**: producer/consumer, contract surface, stability level
- **constraints**: allowed/disallowed dependencies and invariants
- **decisions**: ADR-like records with status and supersession chain

### Example sketch

```yaml
format: architecture/v1
components:
  - id: cli
    responsibility: command orchestration and UX surface
    owner: specctl:cli
  - id: app_core
    responsibility: workflow semantics and lifecycle rules
    owner: specctl:cli
interfaces:
  - id: cli_to_app
    from: cli
    to: app_core
    kind: function_api
    stability: stable
constraints:
  - id: no_infra_from_domain
    rule: "domain -> infrastructure = forbidden"
    severity: error
decisions:
  - id: dec-transport-split
    title: "MCP adapter remains a transport boundary"
    status: accepted
    supersedes: []
```

## CLI and MCP surface additions

### New command group

```bash
specctl arch context <charter:slug>
specctl arch component add|update|remove ...
specctl arch interface add|update|remove ...
specctl arch constraint add|update|remove ...
specctl arch decision add|supersede|deprecate ...
specctl arch check <charter:slug>
specctl arch diff <charter:slug>
```

### `context` integration

Extend existing `specctl context` output with an `architecture` block:

- active decision set
- broken/at-risk constraints
- affected components for current deltas
- architecture `next` guidance (e.g., `review_decision`, `record_constraint_exception`)

### MCP tools

Add parallel tool set:

- `specctl_arch_context`
- `specctl_arch_mutate`
- `specctl_arch_check`
- `specctl_arch_diff`

This keeps agent loops consistent between behavioral and architectural governance.

## How this reduces entropy

### 1) Typed deltas for architecture changes

Introduce architecture deltas linked to behavioral deltas:

- optional `arch_delta_id` on existing delta records
- or explicit linkage table (`delta_links`)

Agents can then answer: *"what architecture intent changed with this behavior change?"* without rereading full history.

### 2) Constraint checks as first-class verification

Support checks that validate architecture constraints from code evidence:

- dependency graph checks (import/layer rules)
- forbidden edge checks (e.g., domain -> infrastructure)
- optional runtime topology checks (service call map)

Persist check results in revision artifacts so architecture claims are auditable.

### 3) Decision lifecycle, not static ADR docs

Treat architecture decisions as mutable lifecycle entities:

- proposed -> accepted -> deprecated
- explicit supersession chain
- mandatory rationale fields kept short and typed

This prevents large stale ADR archives and keeps active truth concise.

### 4) Exception windows for pragmatic drift

Allow bounded exceptions:

- scope (component/interface/constraint)
- expiration revision/date
- owner and remediation plan

`context` should highlight expiring exceptions so agents proactively repair architecture debt.


## Validation and consistency model

Validation should happen in three tiers so architecture stays trustworthy without being brittle:

1. **Schema validation (always-on, fast)**
   - enforce required fields, IDs, enum values, and reference integrity (`interface.from`/`to` must point to known components)
   - fail architecture mutations immediately on malformed state
2. **Static consistency checks (default-on)**
   - map architecture constraints to repository analyzers (imports, package boundaries, forbidden edges)
   - report `pass | fail | unknown` to avoid false confidence when a checker cannot infer enough evidence
3. **Governance checks (revision-time)**
   - require no unresolved `error` severity violations (unless covered by active exception windows)
   - keep `warn` severity non-blocking but visible in `context` and `rev bump` output

### Consistency invariants worth enforcing

- every accepted decision must reference at least one affected component or interface
- every `error` severity constraint must have at least one executable check binding
- every active exception must include owner, expiry, and remediation note
- no orphaned links (deleted component cannot be referenced by interface/constraint)

These invariants keep `ARCH.yaml` as an operational artifact instead of passive documentation.

## Integration with the current specctl workflow

To avoid parallel processes, architecture should reuse existing loop semantics:

- `specctl context` remains the primary entrypoint and includes an architecture summary
- architecture mutations are linked to normal deltas (no separate ticketing model)
- `req verify` and `arch check` results are shown together before `delta close`
- `rev bump` uses a single readiness decision that includes behavior + architecture health

### Backward-compatible rollout

- if a charter has no `ARCH.yaml`, `context` reports `architecture: untracked` with a suggested next step, not an error
- architecture gating is opt-in per charter until teams enable strict mode
- existing behavioral governance remains unchanged for charters that do not adopt architecture tracking yet

## Preventing documentation burden

The system should optimize for **minimum required structure**:

- keep architecture primitives intentionally small (components/interfaces/constraints/decisions only)
- prefer short typed fields over prose (`status`, `severity`, `supersedes`, `owner`, `expiry`)
- auto-suggest updates from detected code graph changes (agent accepts/rejects suggestions)
- support `specctl arch sync --propose` to generate patch proposals instead of hand-writing records
- block narrative bloat by putting length limits on free-text fields (for example rationale summaries)

### Lightweight operating policy

Only require an architecture update when one of these triggers occurs:

- a new boundary is introduced (new component/interface)
- a dependency rule is added/removed/temporarily violated
- a prior accepted decision is superseded

All other code changes can proceed without architecture edits.

## Suggested implementation phases

### Phase 1: Read model + context visibility

- add `ARCH.yaml` reader/writer and schema validation
- add `specctl arch context`
- include summarized architecture state in main `context`

### Phase 2: Mutation commands + diff

- add component/interface/constraint/decision mutate commands
- add architecture diff surface and machine-readable output

### Phase 3: Automated checks

- integrate static dependency checks
- persist architecture check status into revision flow
- gate `rev bump` on unresolved architecture violations (configurable)

### Phase 4: Skill guidance updates

- extend `skills/specctl/SKILL.md` with architecture-aware loop
- teach agent to call `arch context` whenever behavior touches new boundaries

## Minimal agent loop (future)

```text
context -> arch context -> delta -> arch decision/constraint update
-> implementation -> req verify + arch check -> close -> bump
```

This makes architecture reasoning continuous instead of a separate ceremony.


## Brownfield workflow: exact operating model

This is how architecture governance can be introduced into an **existing** spec-governed system without stopping delivery.

### Phase 0: Observe only (no gates)

Goal: create architecture visibility with zero blocking.

1. Team continues normal loop:
   - `specctl context <charter>`
   - `specctl diff <charter>`
   - `specctl delta ...`, `specctl req ...`, `specctl rev bump`
2. Add `ARCH.yaml` with only top-level components and 1-3 high-value constraints.
3. Run `specctl arch check <charter>` in report-only mode.
4. Surface results in CI as non-blocking annotations.

Exit criteria:

- architecture state exists for top-level boundaries
- check noise is acceptable (low false positives)

### Phase 1: Soft governance (warn-level enforcement)

Goal: make architecture part of daily decisions, still low friction.

Workflow on each change:

1. `specctl context <charter>` shows behavior + architecture summary.
2. If change crosses a boundary, add/update one architecture record (component/interface/decision).
3. Run:

```bash
specctl req verify <charter:slug>
specctl arch check <charter:slug>
```

4. Violations at `warn` do not block, but require an owner note or planned follow-up delta.

Exit criteria:

- most cross-boundary deltas are linked to architecture updates
- teams treat architecture warnings as actionable backlog, not ignored noise

### Phase 2: Targeted hard gates (only critical constraints)

Goal: enforce what matters most in brownfield systems.

1. Promote a small subset of constraints to `error` severity (usually layer boundaries and forbidden dependencies).
2. Keep all other rules at `warn` while signal matures.
3. Gate `rev bump` on unresolved `error` violations unless an active time-bounded exception exists.

This avoids the common brownfield failure mode where too many immature rules block delivery.

### Phase 3: Steady-state

Goal: architecture and behavior evolve together by default.

Normal loop becomes:

```text
context (behavior + arch) -> delta -> req update -> code -> req verify + arch check -> close -> bump
```

If a team intentionally violates a rule, they create an exception window with:

- scope
- owner
- expiry
- remediation delta reference

Expired exceptions fail the next readiness decision.

## Brownfield change examples

### Example A: Internal refactor, no boundary change

- Code changes stay within existing component boundaries.
- Architecture edit required: **none**.
- Expected checks: no new architecture violations.

### Example B: New integration path between existing modules

- Add/modify interface record (`from`, `to`, `kind`, `stability`).
- If this creates a previously forbidden edge, either:
  - adjust constraint via accepted decision, or
  - add temporary exception with expiry and remediation.

### Example C: Legacy dependency already violates target layering

- Record the intended rule as constraint (`warn` first).
- Register current debt as explicit exception window.
- Add remediation milestone delta.
- Later promote constraint to `error` once debt is reduced.

## Brownfield anti-burden defaults

To keep maintenance low in legacy repos:

- start with 5-12 components max (coarse topology first)
- define only 2-5 critical constraints initially
- require architecture edits only for boundary/rule/decision changes
- prefer generated proposals (`arch sync --propose`) over manual YAML edits
- cap free-text fields so reviews stay fast

## CI and review integration for brownfield repos

Recommended PR checks:

1. `specctl req verify <charter>`
2. `specctl arch check <charter>`
3. policy check:
   - fail on unresolved `error`
   - fail on expired exceptions
   - allow `warn` with owner attribution

Recommended reviewer prompts:

- Does this PR introduce a new boundary or dependency edge?
- If yes, is architecture intent captured (record or exception)?
- Are any exceptions time-bounded and tied to remediation work?


## Cross-charter architecture: is it needed?

Short answer: **yes, for multi-surface systems**. If charters are truly isolated, local architecture is enough. But once boundaries or dependencies span charters, cross-charter governance is needed to prevent hidden coupling.

### When cross-charter governance becomes necessary

Add cross-charter support when any of these are true:

- one charter exports an interface consumed by another charter
- shared platform constraints apply to multiple charters (security, tenancy, data residency)
- releases require compatibility coordination across charter boundaries
- drift in one charter can break another without local violations

If none apply, keep scope local and avoid extra complexity.

## Proposed cross-charter model (minimal)

Keep current charter-local `ARCH.yaml`, then add a lightweight global index:

```text
.specs/architecture/CROSS_CHARTER.yaml
```

with three primitives:

- **exports**: interfaces intentionally exposed by a source charter
- **imports**: declared consumer dependencies on exported interfaces
- **global_constraints**: rules that span multiple charters

### Example sketch

```yaml
format: architecture-cross/v1
exports:
  - id: mcp.tooling.v1
    charter: specctl:mcp
    component: mcp_server
    stability: stable
imports:
  - id: cli-uses-mcp-tooling
    from_charter: specctl:cli
    to_export: mcp.tooling.v1
    compatibility: "^1"
global_constraints:
  - id: no-direct-cli-to-infra
    rule: "specctl:cli -> infrastructure = forbidden"
    severity: error
```

## Consistency and validation for cross-charter state

Validation should include:

1. **Reference integrity**
   - every import points to an existing export
   - exported interfaces map to existing charter-local components/interfaces
2. **Compatibility checks**
   - importing charter declares compatible version/stability window
   - breaking export changes require either coordinated updates or transition exceptions
3. **Global constraint checks**
   - evaluate rules across the combined dependency graph
   - report owning charter(s) for each violation to avoid ambiguous accountability

## Workflow integration (without heavy burden)

### New optional commands

```bash
specctl arch cross context
specctl arch cross check
specctl arch cross diff
```

### How it fits existing loop

- charter-local work remains unchanged for most deltas
- if a delta changes an exported interface, agent also runs `arch cross check`
- `rev bump` in a producer charter warns/fails if it introduces cross-charter incompatibility
- consumers can pin to export compatibility ranges to decouple release timing

## Ownership and exception policy across charters

To prevent coordination deadlocks:

- each export has a **producer owner**
- each import has a **consumer owner**
- each violation names both producer and impacted consumers
- exceptions must include: owner, expiry, and migration target revision

This preserves accountability and avoids permanent cross-charter waivers.

## Recommended rollout

1. start with visibility only (`arch cross context`) for a few high-value interfaces
2. add compatibility warnings (non-blocking)
3. enforce only critical global constraints as `error`
4. keep all other cross-charter checks at `warn` until signal quality is high

This keeps cross-charter governance proportional to actual system coupling.

## Practical success metrics

- % of deltas linked to architecture decisions
- mean time from architecture drift detection to recorded decision/exception
- count of expired exceptions at revision bump
- number of violations caught pre-merge by `arch check`

## Recommended first increment for specctl

If implementing only one increment, start with:

1. `ARCH.yaml` typed model
2. `specctl arch context`
3. architecture summary embedded into existing `specctl context`

This gives immediate value to agents (better planning context) with minimal operational burden.



## Linting profile and conventions

For explicit linting rules, severity policy, naming/granularity conventions, and alternatives/tradeoff analysis focused on agent simplicity, see:

- [`docs/architecture-linting-and-conventions.md`](./architecture-linting-and-conventions.md)

## Dogfooding in this repository

For a concrete rollout plan scoped to this repository and its existing charters (`cli`, `mcp`, `skill`, `dashboard`), see:

- [`docs/architecture-dogfood-plan.md`](./architecture-dogfood-plan.md)

## PR checklist for architecture changes

When architecture-governed changes are introduced, a PR should include:

- what boundary/interface/constraint/decision changed
- whether the change is charter-local or cross-charter
- `specctl arch check` result summary (`pass | fail | unknown`)
- any active exception windows (owner + expiry + remediation link)
- migration/compatibility impact on downstream consumers

This keeps review quality high without requiring long narrative writeups.
