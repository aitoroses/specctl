# specctl Architecture

This document is the shortest path for a new maintainer to understand how the codebase is organized.

Important framing: `specctl` is **agent-facing first**. The CLI exists, but the core product value is that an agent can use specctl through a skill + CLI/MCP surfaces and follow governed transitions without inventing workflow logic.

## Core idea

`specctl` is a specification governance engine, not a planner or test runner.

It has three major surfaces:

1. **Core governance semantics** — requirements, deltas, drift, revisions
2. **CLI adapter** — command/execution surface that agents can drive directly
3. **MCP adapter** — stdio MCP server exposing `specctl_*` tools
4. **Skill surface** — the human-authored instructions that teach agents how to use specctl well

The **meaning** of the system lives in the behavioral specs:

- `SPEC.md` — core semantics
- `internal/mcp/SPEC.md` — MCP transport semantics

The **default agent entrypoint** lives in:

- `skills/specctl/SKILL.md`

The **mutable tracking state** lives in:

- `.specs/specctl/*.yaml`

## Package responsibilities

### `internal/domain`

Pure domain types and invariants:

- charter
- delta
- requirement
- tracking file rules

If a rule should hold regardless of CLI or MCP, it probably belongs here.

### `internal/application`

Use-case orchestration and projections:

- read surfaces (`context`, `diff`)
- write surfaces (`delta add`, `req replace`, `sync`, `rev bump`)
- next-action generation
- contract/read models

This is the behavioral engine most adapters call into.

### `internal/infrastructure`

Persistence and environment adapters:

- git
- filesystem
- design doc IO
- registry / tracking file IO
- validation / repo helpers

This package is intentionally broad, but it should remain adapter-oriented rather than semantic.

### `internal/cli`

Cobra command layer:

- argument parsing
- stdin handling
- command help
- response envelopes

This package should stay thin relative to `internal/application`, because the goal is not a human-heavy CLI product; it is an agent-drivable execution surface.

### `internal/mcp`

MCP transport adapter:

- MCP server setup
- MCP tool registration
- translation from specctl next-actions into MCP hints
- uninitialized fallback behavior

This package should not redefine core governance semantics.

## Skill as first-class product surface

`skills/specctl/SKILL.md` is not auxiliary marketing copy. It is part of the product surface:

- it tells agents when to invoke specctl
- it teaches the ownership split
- it explains how to interpret `next`
- it keeps agent behavior aligned with the governed model

If the tool is extracted, the skill should be treated as a first-class maintained artifact.

### `internal/presenter`

Formatting/presentation helpers shared across transport surfaces.

### `dashboard/`

Optional governance UI:

- React/Vite SPA
- embedded into Go via build pipeline
- not required for core CLI/MCP governance

## Governance boundaries

### `specctl:cli`

Owns core behavioral semantics and broad core implementation.

### `specctl:mcp`

Owns MCP transport-specific behavior only.

### `specctl:dashboard`

Owns dashboard UX and delivery surface.

## Design rules for future changes

1. **Do not hand-edit tracking YAML**
2. **Do not duplicate semantics across CLI and MCP specs**
3. **Prefer core semantics in `domain` / `application`, adapters in `cli` / `mcp`**
4. **Keep dashboard optional**
5. **If a change only affects transport behavior, keep it out of the core spec**

## Extraction concerns

The code is already a standalone Go module, but extraction still needs:

- root CI workflow
- explicit license choice
- final boundary cleanup for a few overlapping monorepo-owned files
- release/versioning process
- confirmation that the skill ships with the extracted repo as a first-class entrypoint
