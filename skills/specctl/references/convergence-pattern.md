# Convergence Pattern

A convergence plan computes the delta between CURRENT state (what
exists in the codebase) and TARGET state (what JOURNEYS.md specifies).
Running it repeatedly shrinks the delta toward zero.

## The Convergence Loop

```
JOURNEYS.md + Codebase → Analyze → Plan → Review → Execute → Verify
                              ↑                                  |
                              └──────── re-run on drift ─────────┘
```

## Analysis Phase

Discover the CURRENT state by scanning:

1. **Test files** — existing specs/tests, their phase coverage, tags
2. **Domain helpers** — selectors, interaction functions, proof factories
3. **Seed patterns** — database schema, seed client methods, entity relationships
4. **Instrumentation** — data-testid attributes, API endpoints, MCP tools
5. **MCP tools** — available E2E infrastructure (boot, seed, probe, verify)

Classify each journey as:
- **NEW** — no test file exists
- **PARTIAL** — some phases implemented
- **COMPLETE** — all phases match JOURNEYS.md
- **DRIFT** — journey updated since test was written
- **LEGACY** — test predates JOURNEYS.md (migration candidate)

Actions per classification:
- **COMPLETE** → no changes needed. Include in verification pass only.
- **LEGACY** → treat as DRIFT. Compare test against JOURNEYS.md, produce migration plan. Absorb patterns, then delete the legacy test.

## Plan Structure

The convergence plan has per-journey sections:

### For NEW journeys
```
GATE:       prerequisite product changes (missing instrumentation, endpoints)
SEED:       environment design (entities, relationships, API keys)
DISCOVERY:  what to explore during agent-driven discovery
PHASES:     per-phase implementation notes (selectors, helpers, proofs)
```

### For PARTIAL journeys
```
DELTA:      only the missing/modified phases
HELPERS:    new domain helpers needed
PROOFS:     new proof factories to implement
```

### For DRIFT journeys
```
DIFF:       what changed in JOURNEYS.md since the test was written
MIGRATION:  specific phase updates needed
```

## Plan Rules

- **Reference JOURNEYS.md by phase name** — never reproduce DO/SEE/CHECK content
- **No code blocks** — the plan is a strategy document, not implementation
- **No absolute timing** — timing comes from DISCOVERY
- **No pre-baked selectors** — selectors are mined during DISCOVERY
- **JOURNEYS.md is source of truth** — the plan adds implementation context

## Execution

Plans are executed following the Agent-First methodology:

1. **GATE items first** — implement prerequisites in the product
2. **SEED design** — implement seed functions for each journey
3. **Per-journey, phase by phase:**
   - DISCOVERY: boot environment, probe system, mine data
   - WRITE: implement test phases incrementally, always green
4. **Verify** — run the test, then `specctl req verify` with the test file

## Companion PRD

For automated execution, the plan can produce a machine-readable PRD:

```json
{
  "conventions": { "test_dir": "e2e/tests/", "runner": "playwright" },
  "stories": [
    {
      "id": "INFRA-001",
      "title": "Add SEL.newSelector to domain helpers",
      "type": "infrastructure",
      "mode": "code-only",
      "acceptance": ["grep confirms export exists"]
    },
    {
      "id": "W1-GATE",
      "title": "Instrument missing data-testid attributes",
      "type": "gate",
      "mode": "code-only",
      "depends_on": []
    },
    {
      "id": "W1-01",
      "title": "W1 phase 01_landing",
      "type": "journey-phase",
      "mode": "mcp-interactive",
      "depends_on": ["W1-GATE", "INFRA-001"],
      "acceptance": ["e2e_run(W1) passes phase 01"]
    }
  ]
}
```

Stories execute in dependency order. Infrastructure and gate items
are code-only. Journey phases are mcp-interactive (requiring
DISCOVERY with a live environment).

## Automation (optional)

If oh-my-claudecode (Claude Code) or oh-my-codex (Codex) is installed,
the convergence workflow can be automated via multi-agent consensus:
Analyst → Planner → Architect → Critic loop, then execution via ralph.

Without orchestration plugins, follow the analysis → plan → execute
steps manually using this reference. The methodology is the same —
the plugins automate the agent coordination, not the pattern itself.
