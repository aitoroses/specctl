---
name: specctl
description: >
  Specification governance engine for agent-driven development. Use when
  asked to govern specs, track requirements, manage behavioral contracts,
  set up spec-first development, adopt specctl in a project, plan E2E
  testing from specs, or implement the Agent-First verification pattern.
  Also use when you see .specs/ directories, specctl.yaml configs,
  SPEC.md behavioral documents, or when drift is detected between specs
  and code. Covers greenfield setup, brownfield adoption, and the full
  spec-to-verified-tests pipeline.
compatibility: >
  Requires specctl binary (Go). Run scripts/setup.sh to install and
  configure the MCP server for Claude-style JSON config or Codex TOML
  config. Works with Claude Code, Cursor, Codex,
  and any agent supporting the Agent Skills format.
allowed-tools: Bash(specctl:*) Bash(go:*) Read Write Edit
---

# specctl — Specification Governance for Agents

**Agent = Model + Harness.** specctl is the governance layer of
your agent harness — it defines what behaviors matter, tracks their
verification state, detects when reality drifts from spec, and
guides the agent back to alignment. Combined with an MCP-based
verification surface (the sensor layer), specctl enables any
persistent agent loop to incrementally build and verify software.

**specctl is:** a spec registry, requirement tracker, change-management
system, and lifecycle state machine with agent-first guidance.

**specctl is not:** a planning system, execution system, test runner,
or documentation generator. It governs — other tools execute.

## Core Concepts

- **Charter** — namespace grouping related specs (e.g., `runtime`, `ui`)
- **Spec** — tracking file governing one behavioral surface
- **Delta** — unit of intentional change (add/change/remove/repair) with FSM
- **Requirement** — one observable behavior anchored by a Gherkin block
- **Revision** — checkpointed milestone after all requirements verified
- **SPEC.md** — human-owned design document (prose, data model, contracts, invariants, Gherkin)

**Ownership split:** SPEC.md owns meaning. specctl owns collaboration
state (tracking YAML). Agents never hand-edit tracking YAML.

## How to Interact

specctl runs as an **MCP server**. Use `specctl_context` as your
entry point — it returns the spec's health and explicit `next` actions.
Every response is JSON with `{state, focus, next}`:

```json
{
  "state": { "slug": "my-spec", "charter": "core", "status": "active" },
  "focus": { "delta": { "id": "D-001", "intent": "add", "status": "open" } },
  "next": {
    "mode": "sequence",
    "steps": [
      { "action": "write_spec_section", "kind": "edit_file",
        "mcp": { "available": false, "reason": "unsupported_in_v1" } },
      { "action": "add_requirement",
        "mcp": { "tool": "specctl_requirement_add", "input": { "spec": "core:my-spec", "delta_id": "D-001" } } }
    ]
  }
}
```

**Follow `next` actions.** Each step has an `mcp` field:
- `mcp.available: true` → call `mcp.tool` with `mcp.input` directly
- `mcp.available: false` → agent-owned action. Perform it yourself
  (edit a file, run a command, commit code), then continue to the next step.

For committed drift, `specctl_context` is still the triage surface, not
the full review surface. When it marks drift as `review_required`, treat
its `review_diff` action as the canonical handoff to `specctl_diff`
before deciding whether the change needs semantic tracking or only a
checkpoint sync.

**`write_spec_section` is the most common agent-owned action.** It means:
open the SPEC.md file, write behavioral prose following the five-layer
structure in `references/spec-format.md` (prose, data model, contracts,
invariants, Gherkin), then call the next MCP tool in the sequence
(typically `specctl_requirement_add`). Never skip this step — specs
are documents, not inventories.

**Config operations require CLI.** Commands like `config add-tag` and
`config add-prefix` are not available via MCP in v1. When `next`
suggests a config operation, use the Bash tool: `specctl config add-tag <tag>`.

For any tool's parameters and error codes, use `specctl <cmd> --help`.

### When to Load References

Load reference documents on demand to conserve context:
- Writing a new SPEC.md → load `references/spec-format.md`
- Creating journey files → load `references/journey-format.md`
- Planning E2E convergence → load `references/convergence-pattern.md`
- Implementing tests → load `references/agent-first-e2e.md`
- Setting up a test surface → load `references/verification-surfaces.md`
- Designing/building E2E infrastructure → load `references/framework-architecture.md`
- Serious brownfield adoption → load `references/brownfield-crystallization.md`

## Setup

Run the setup script from this skill's `scripts/` directory:

```bash
bash scripts/setup.sh
```

If the path is not resolved automatically, locate the skill directory
and run `bash <absolute-path>/scripts/setup.sh`. The script installs
the specctl binary via `go install` and configures MCP targets.

Supported targets:

```bash
# project-local JSON MCP config
bash scripts/setup.sh

# Claude Code global MCP config
bash scripts/setup.sh --claude-global

# Codex global TOML MCP config
bash scripts/setup.sh --codex-global

# both Claude + Codex global config
bash scripts/setup.sh --global
```

The setup script is intended to be idempotent:
- reruns repair stale `specctl` entries
- unrelated config is preserved
- Codex TOML config is updated without duplicating the `mcp_servers.specctl` block

After setup, call `specctl_context` to check if governance exists, or
run `specctl example` to see the tool's own governed spec as a reference.

## Greenfield Workflow

Use when the repo has no `.specs/` directory.

### Decision Checklist

1. **Source code exists?** No → spec-first (write SPEC.md before code). Yes → continue.
2. **Clear domain structure?** No → identify behavioral surfaces first. Yes → continue.
3. **Single or multi-team?** Single → one charter. Multi → one charter per domain.
4. **What surfaces exist?** UI routes, API endpoints, MCP tools, jobs → each becomes a spec.
5. **Existing tests?** Can link as verification evidence immediately.
6. **CI pipeline?** If yes, verification loop can close end-to-end.

### Sequence

1. Call `specctl_init` → creates `.specs/` + config with auto-detected prefixes
2. Call `specctl_charter_create` → creates the namespace
3. Call `specctl_spec_create` → creates tracking file + links to SPEC.md
3. Write SPEC.md following `references/spec-format.md` — the five-layer
   behavioral surface template (prose, data model, contracts, invariants, Gherkin)
4. Follow `next` actions — specctl guides you through:
   `specctl_delta_add` → `write_spec_section` (edit SPEC.md) →
   `specctl_requirement_add` → implement + test →
   `specctl_requirement_verify` → `specctl_delta_close` → `specctl_revision_bump`

**Format template:** for new projects, start by reading the embedded
format template via `specctl example`. It shows the full five-layer
structure applied to a real system. See `references/spec-format.md`
for the portable version.

## Brownfield Conversion

Use when the repo has existing code, docs, or tests but no governance.

**Key principle: govern in waves, not all at once.** Start with the
strongest surface (clearest docs, existing tests, lowest overlap risk),
govern it fully, verify, freeze, then widen through an explicit review gate.

### Quick-Start Steps

1. **Ground** — call `specctl_init`, then `specctl_context`. Scan for
   existing artifacts: docs, ADRs, OpenAPI, tests, code.

2. **Clarify model** — decide charter grouping (by contract family),
   spec grouping (by external behavioral surface), and first-wave targets.

3. **Freeze review** — before governing, approve: which surfaces, which
   docs own meaning, what contradictions exist, what's deferred.

4. **Govern sequentially** — for each approved surface:
   `specctl_charter_create` → `specctl_spec_create` → write SPEC.md →
   follow `next` → `specctl_requirement_verify` → `specctl_delta_close` →
   `specctl_revision_bump`

5. **Re-verify** — `specctl_context` + run tests after each surface.

6. **Hold** — don't widen automatically. New surfaces need a new review gate.

### Source Precedence

When docs, code, and tests disagree:
1. Governed `.specs/` docs (already-governed surfaces)
2. Primary docs (not-yet-governed)
3. ADR invariants
4. Tests (verification evidence only — never meaning owners)
5. Code (implementation evidence only)

### Verification Maturity

Not all requirements have the same evidence quality:
- **verified** — specific test maps clearly to the requirement
- **partially verified** — some scenarios covered, gaps remain
- **unverified** — no test evidence exists
- **manual-only** — verified by inspection (`@manual` tag)

Never mark verified just because a test file exists nearby.

### Edge Cases

- **`.specs/` exists** → call `specctl_context` first. Resume, don't reinitialize.
- **Monorepo** → multiple charters (one per service/domain).
- **Tests exist, no specs** → extract behavioral contracts from assertions, write SPEC.md.
- **OpenAPI/protobuf** → convert to behavioral format with prose and invariants.
- **Partial governance** → incremental. Start where drift hurts most.
- **Sources contradict** → defer, open delta, mark drift, or create repair task. Never silently reconcile.

See `references/brownfield-crystallization.md` for the full 8-phase
gated methodology with freeze-evidence packages, req→doc matching
rules, and wave-based widening.

## The Pipeline: Spec to Verified Tests

specctl governance is step 1. The full pipeline:

```
SPEC.md → JOURNEYS.md → Convergence Plan → Implementation → Verified
```

1. **Spec** — behavioral surfaces governed by specctl
2. **Journeys** — natural user stories from requirements (see `references/journey-format.md`)
3. **Plan** — current-to-target delta (see `references/convergence-pattern.md`)
4. **Implement** — Agent-First: GATE → SEED → DISCOVERY → WRITE (see `references/agent-first-e2e.md`)
5. **Verify** — `specctl_requirement_verify` with test evidence, then `specctl_revision_bump`

### Building the Verification Harness

specctl is the governance layer (guides). The verification harness
you build is the sensor layer (feedback). Together they form a
complete Agent-First TDD harness that any persistent loop can drive.

The harness answers three questions:
1. Can the agent boot an environment with a given state (scenario)?
2. What tools does the agent have to inspect and drive the system?
3. How does the agent encode its learnings as durable test code?

Spec the harness with specctl before building it — the harness IS a
product. The agent builds it following the governance loop, then uses
it to verify everything else. This is the ouroboros: specctl governs
the harness that verifies the product that specctl also governs.

See `references/framework-architecture.md` for the full design guide:
what an agent harness is, how persistent loops consume it, layered
architecture, MCP server design, seed client patterns, phase + proof
system, journey test artifact shape, and the DISCOVERY → WRITE workflow.

### E2E Framework Qualities

A well-designed test framework needs these qualities. Spec each one:

**Seeding Architecture**
- Semantic seed API: `s.devUser()`, `s.agent(name, config)`, `s.project(name, opts)`
- Entity-relationship aware: seed order respects dependencies
- Scoped cleanup: each test's seed data is isolated and torn down
- Seed client as the contract boundary between framework and domain

**Timing and Budgets**
- Phase-level budgets: expected duration (from discovery) + max timeout (failfast)
- Budget enforcement via Promise.race or equivalent — automatic, not manual
- Discovery feeds timing: real measurements become budget values
- Relative timing in specs (fast/moderate/slow), absolute only in implementation

**Observability**
- Auto screenshots on phase enter/exit (UI surfaces)
- Action timeline: what happened, in what order, how long
- Failure classification: product bug vs framework bug vs infra failure
- Structured artifacts per test: failure summary, DOM snapshot, state dump
- Attempt-scoped evidence: distinguish primary failure from retry noise

**Proof System**
- Domain-owned verification: proof factories define HOW to verify
- Framework-owned lifecycle: polling, timeout, reporting
- Proof descriptors: metadata (kind, path, expected) for diagnostics
- Inline declaration: proofs live in the phase that verifies them

**MCP Surface for Agents**
- Boot/Seed/Probe/Verify — four operation categories for any test surface
- Agent-mode prompt: system prompt giving the agent browser/API instructions
- Entity reference map: seeded data available to the agent after boot
- Workspace operations: read files, execute commands, inspect trees

## Verification Surfaces

The methodology is identical across surfaces. What changes is the client:

| Surface | Client ("eyes") | DISCOVERY |
|---------|-----------------|-----------|
| UI | agent-browser (headed browser) | Navigate, mine selectors, screenshots |
| Backend API | mcporter (MCP-over-HTTP) | Call endpoints, mine schemas |
| MCP tools | Direct MCP calls | Call tools, verify outputs |
| Background jobs | MCP + DB/queue probes | Trigger, observe side effects |

**Universal principle:** expose your system under test as an MCP surface
with Boot/Seed/Probe/Verify operations. See `references/verification-surfaces.md`.

## Gotchas

- **Never hand-edit tracking YAML.** Only specctl mutates these files.
- **Follow `next` actions.** Don't guess the workflow — `next` contains MCP tool hints.
- **Never skip `write_spec_section`.** Write behavioral prose BEFORE formalizing with Gherkin.
- **Requirement = observable behavior.** External surface, not implementation internals.
- **`@manual` for non-automated verification.** Inspection-verified requirements.
- **Unregistered Gherkin tags:** `INVALID_GHERKIN_TAG` → use Bash: `specctl config add-tag <tag>` (CLI-only, not available via MCP), then retry the MCP tool call.
- **Delta close blocked:** all requirements must be verified first.
- **Sync vs delta add:** for review-required drift, start with `review_diff`. Use `sync` only when that review concludes the drift is clarification-only or the checkpoint just needs re-anchoring; otherwise open the appropriate delta.
- **Uninit repo?** Call `specctl_init` first. It creates `.specs/` and auto-detects source prefixes. All other tools return `NOT_INITIALIZED` until init runs.
- **`warnings` in config/context:** `specctl_context` and `specctl config` emit advisory `warnings`. Config warnings cover missing `source_prefixes` entries on disk: `{"kind":"MISSING_SOURCE_PREFIX","prefix":"ui/src/","resolved_path":"/abs/path","severity":"warning"}`. Spec-context warnings can also surface governed residue such as `DEFERRED_SUPERSEDED_RESIDUE` with `delta_ids`, `requirement_ids`, and `details`. These are advisory — review them through governed specctl actions, never by hand-editing tracking YAML.
- **Spec the test infra too.** The framework is a product — govern it like one.

## Delta escape hatches: withdraw, rebind, repair validation

Three agent-facing escape hatches exist for governance states that otherwise
force misuse of `defer` and create permanent residue:

- **Opened a delta in error?** Use `specctl delta withdraw <charter:slug> <D-id> --reason "<text>"`.
  This transitions `open | in-progress | deferred → withdrawn` with an
  auditable reason. Withdrawn deltas are inert: they do not emit
  `DEFERRED_SUPERSEDED_RESIDUE`, do not count as live, and cannot be
  resumed. If you change your mind, open a fresh delta. Do **not** use
  `delta defer` to park an error — `defer` signals "not now, maybe
  later" and keeps residue warnings firing.

- **Your requirements were superseded under an open delta?** Two paths:
  - Auto: nothing to enable. `req replace` always auto-rebinds every
    independent `open | in-progress | deferred` delta whose
    `affects_requirements` references the replaced REQ. Each rebind
    emits an `AUTO_REBIND_APPLIED` entry under `result.auto_rebinds[]`
    on the `req replace` response. Absence of `result.auto_rebinds`
    means no open delta matched, not that rebinding was disabled.
  - Explicit: `specctl delta rebind-requirements <charter:slug> <D-id>
    --from REQ-X --to REQ-Y` (or `--remove REQ-X --reason "<text>"`).
    Works on `open | in-progress | deferred` deltas only; `closed`
    deltas keep their anchors immutable. Use this when auto-rebind
    picked the wrong target (e.g. scope narrowed on replace) or when
    you want a reason recorded.

- **`delta add --intent repair` got `VALIDATION_FAILED` with
  `reason: closed_delta_invariant`?** A closed delta already depends on
  the requirement being verified, so `req stale` (the only update path
  repair allows) is forbidden. The error payload lists the conflicting
  closed deltas under `conflicts[]` and names the intent to use instead
  under `suggestion.intent`. Re-run `delta add` with `--intent change`
  and follow `req replace` to preserve closed-delta verification while
  introducing an updated successor requirement.

### Observable reason fields

When you pass a reason to these verbs, inspect the response envelope to
confirm it was recorded. The audit data lives on both the write result and
the state projection:

- `specctl delta withdraw` → `result.delta.withdrawn_reason` **and**
  `state.deltas.items[].withdrawn_reason` **and** `focus.delta.withdrawn_reason`.
  All three carry the same string. The state field is what survives into
  future `specctl context` calls; the result field is what you observe
  immediately. The diff surface also records the transition under
  `deltas.withdrawn[]`.
- `specctl delta rebind-requirements --to ... --reason ...` →
  `result.rebind.reason`. `--reason` is optional on `--to` (useful for the
  audit trail) and required on `--remove`. Both paths write to the same
  `result.rebind.reason` field; the absence of the key in `--to` without a
  reason is intentional, not a bug.
- `specctl req replace ...` → `result.auto_rebinds[]`, one entry per
  rebound delta with `code: "AUTO_REBIND_APPLIED"`, `delta`, `from`, `to`.
  Rebinds are unconditional — there is no config gate. Absence of
  `result.auto_rebinds` means no independent open delta referenced the
  replaced requirement, not that rebinding was skipped.

### Repair-intent validation walkthrough

The `VALIDATION_FAILED` payload from `delta add --intent repair` is structured
so the agent can retry without interaction:

```
focus.delta_add.reason                = "closed_delta_invariant"
focus.delta_add.conflicts[]           = [ { closed_delta, requires_verified }, ... ]
focus.delta_add.suggestion.intent     = "change"
focus.delta_add.suggestion.rationale  = "<human-readable>"
```

Retry with the suggested intent: the same `area` and `notes`, `intent: change`
instead of `repair`, the same `affects_requirements`. Then follow the write
spec section guidance and `req replace` the conflict-named requirement(s)
with updated successors so closed-delta verification evidence stays valid.

## Recommended Tools

- **oh-my-claudecode** — multi-agent orchestration plugin for Claude Code. Provides ralph (persistent execution loops), and ralphplan (convergence planning) and much more. Install via `npx skills find oh-my-claudecode`.
- **oh-my-codex** — equivalent orchestration plugin for OpenAI Codex.
- **agent-browser** — headed browser automation for UI DISCOVERY. The agent navigates real pages, mines selectors, captures screenshots.
- **mcporter** — MCP-over-HTTP transport for backend API DISCOVERY. Wraps REST endpoints as MCP tools — Postman but agent-first.
- **Playwright** — test runner for UI journey tests. Phase-based execution with budgets, retries, and trace viewer.
