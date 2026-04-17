# Agent-First E2E Test Methodology

Spec First. Agent First. Validation Lock.

A methodology where an AI agent with MCP access leads the full cycle:
discover what to test, implement what's missing, write the test, and
lock it green.

## The Workflow

```
SPEC → SCENARIOS → GATE → SEED → DISCOVERY → WRITE
```

### 1. Spec

Read the spec or design document. This is the source of truth for what
the product should do. In specctl-governed projects, this is the SPEC.md
with behavioral surfaces and Gherkin requirements.

### 2. Scenarios

Extract user journeys and acceptance criteria from the spec. Each
scenario becomes a candidate journey test. See `journey-format.md`
for the JOURNEYS.md structure.

### 3. Gate

Does the app support this scenario? Check:
- Are required UI elements instrumented (data-testid)?
- Do the API endpoints exist?
- Are the MCP tools available?

If not, the agent implements the prerequisite first — questioning
design decisions against the spec, not just making it work.

### 4. Seed

Design the environment for the scenario:
- What users, entities, API keys need to exist?
- What state must be present before the journey starts?
- Use the test MCP's seed operations to create preconditions.

### 5. Discovery

The agent boots the environment via MCP, then executes the spec:

- Probes the system with the spec in hand
- Verifies the real flow matches the spec
- Directs implementation if something doesn't match
- Mines data: selectors, timings, response shapes, states
- Captures evidence at each step (screenshots for UI, responses for API)
- Refines the prompt until it can replay the journey end-to-end

**Exit condition:** the agent can replay the full spec without friction
AND all assertions verify green. Typically 2-5 iterations.

The **prompt** is the artifact of this loop — it captures everything
learned during discovery.

### 6. Write

The test is written **phase by phase, always green**:

1. Start with seed + prompt + empty journey body — green
2. Add phase 1 assertions — run — green
3. Add phase 2 — run — green
4. Continue until the test reflects the prompt

**Never break green.** Each phase is validated before adding the next.
When a phase fails, roll back, debug, fix, re-add.

## Phase Structure

Every test is organized into numbered phases:

```
01_landing     — navigate and verify entry point
02_first_send  — primary user action
03_bootstrap   — system initialization
04_first_turn  — verify system response
```

Each phase has:
- **Label** — human-readable description
- **Budget** — expected duration + max timeout (from discovery)
- **Proofs** — domain-defined assertions with verify functions

## Prompt Vocabulary

Three verb prefixes in prompts and journey descriptions:

- **DO** — an action (navigate, click, type, call). Always immediate.
- **SEE** — a visual/observable assertion (visible, gone, contains). Polls until satisfied.
- **CHECK** — a data/state assertion not directly visible (file content, DB row, API response). Polls until satisfied.

**Rule of thumb:** if a human could verify by looking, use SEE. If it
requires inspecting data behind the surface, use CHECK.

## Proof System

Proofs are domain-defined assertions with both metadata (for reporting)
and a `verify` function (for execution):

```
Proof {
  kind: string              // 'workspace-file', 'url-match', etc.
  verify: (ctx) => boolean  // domain owns the logic
  [descriptors]             // path, expected, pattern — for diagnostics
}
```

**Boundary:** domain teaches the framework HOW to verify. Framework
decides WHEN and HOW LONG to try (polling within phase budget).

## Architecture: Portable vs Project-Specific

| Layer | Contents | Portable |
|-------|----------|----------|
| **Framework** | phases, proofs, budget, screenshots, seed primitives | Yes |
| **Domain** | selectors, page interactions, proof factories | No — project-specific |

The framework knows nothing about the product. A different project
brings its own domain helpers and proof factories but reuses the
same framework pattern.

## Principles

1. **Prompt is first-class** — the distilled artifact of discovery, not documentation
2. **Always green** — built incrementally, never break
3. **Agent is self-sufficient** — spec + MCP + implementation tools
4. **Phase-contained** — every action inside a budgeted phase
5. **Semantic test body** — reads like the spec, no framework mechanics
6. **Discovery feeds everything** — timings, selectors, flow, evidence
7. **Validation lock** — incremental green is irrefutable proof
8. **Portable pattern** — framework generic, project provides domain + MCP
