# specctl

Agent-facing specification governance for behavioral contracts.

`specctl` is primarily a **tool for agents**, not a human-first CLI.

Most agent loops fail for the same reason: not because the model is incapable,
but because the workflow has no durable source of truth for:

- what behavior matters
- what changed intentionally
- what has actually been verified
- what the next legal move is

Without that, agents drift. They overfit to code, miss intent, hand-wave
verification, and declare partial work “done.”

`specctl` exists to close that gap.

It gives an agent a governed workflow:

1. understand the current spec surface
2. record intentional change as a delta
3. register or update requirements
4. verify evidence
5. converge the checkpoint/revision

At every step, it returns explicit `next` guidance so the agent does not have
to invent process on the fly.

For humans, the main entrypoint is usually the packaged skill:

- [`skills/specctl/SKILL.md`](./skills/specctl/SKILL.md)

That skill teaches an agent when to use specctl, how to interpret `next`, and
how to move through the governed workflow without inventing its own process.

## What it includes

- **CLI** — an agent-consumable command surface for spec creation, deltas, requirements, verification, and revision management
- **MCP adapter** — stdio MCP server exposing `specctl_*` tools
- **Dashboard** — optional Vite/React governance UI embedded into the Go binary
- **Self-governance** — specctl governs itself via `.specs/specctl/*` and `SPEC.md`

## Install

### Binary

```bash
go install github.com/aitoroses/specctl/cmd/specctl@latest
```

For stable consumers, prefer a tagged version once releases are cut:

```bash
go install github.com/aitoroses/specctl/cmd/specctl@vX.Y.Z
```

### Packaged skill

```bash
npx skills add https://github.com/aitoroses/specctl --skill specctl --global
```

## The story in one paragraph

Humans do not usually sit and drive `specctl` command-by-command. A human sets
direction, maintains the repo, and decides what policies should hold. The
agent does the operational work: it reads context, follows `next`, edits spec
documents, implements code, verifies evidence, and keeps the lifecycle state
aligned. `specctl` is the layer that keeps that loop honest.

## How to think about it

The intended model is:

1. a human installs/configures the tool once
2. an agent uses the packaged skill
3. the agent drives `specctl` through CLI or MCP
4. the agent follows `next` guidance to stay inside legal transitions
5. a human reviews and approves important outcomes

So the product should be evaluated as:

- **a tool for agents**
- **a skill-backed workflow surface**
- **a governance engine with adapters**

not as a traditional human-operated CLI product.

## What it feels like to use

The shortest path is:

```bash
specctl context <charter:slug>
```

That gives the agent:

- current lifecycle state
- drift status
- requirement match integrity
- verification state
- explicit `next` actions

From there the loop becomes:

```text
context -> delta -> requirement -> verify -> close -> bump/sync
```

That is the core product experience.

## Human setup / maintainer quick start

### Prerequisites

- Go `1.26.x` toolchain
- `pnpm` for dashboard development/builds

### Build the CLI

```bash
make build
```

### Install locally

```bash
make install
```

### Run Go tests

```bash
make test-go
```

### Build the dashboard bundle

```bash
make dashboard-build
```

### Run all primary checks

```bash
make test
```

## Repository layout

```text
cmd/specctl/             Cobra entrypoint
internal/application/    Workflow orchestration and read/write projections
internal/domain/         Pure domain types and invariants
internal/infrastructure/ Filesystem, git, config, registry, persistence adapters
internal/cli/            CLI command surface and envelope formatting
internal/mcp/            MCP transport adapter and MCP-specific tests/spec
internal/presenter/      Response presentation helpers
dashboard/               Optional React/Vite dashboard
.specs/specctl/          Self-governance tracking files
SPEC.md                  Core behavioral specification
internal/mcp/SPEC.md     MCP adapter behavioral specification
skills/specctl/          Agent skill packaging and references
test/                    E2E shell coverage
testdata/                Fixtures and golden inputs
```

## Behavioral ownership

- `SPEC.md` owns **core product semantics**
- `internal/mcp/SPEC.md` owns **MCP transport-specific behavior**
- `.specs/specctl/*.yaml` owns **tracking state**

Important rule: **do not hand-edit tracking YAML**. Use `specctl` commands.

## Main entrypoints

### For agents

- [`skills/specctl/SKILL.md`](./skills/specctl/SKILL.md)
- `specctl mcp`
- `specctl context ...`
- packaged skill install:

```bash
npx skills add https://github.com/aitoroses/specctl --skill specctl --global
```

### For humans

Humans mostly:

- install/build the tool
- maintain specs and code
- debug edge cases
- review governance state
- evolve the skill/documentation

The important distinction is:

- humans define intent and policy
- agents operate the workflow

## Debug / maintainer workflows

### Review spec health

```bash
specctl context specctl:cli
specctl context specctl:mcp
```

### Run the MCP server

```bash
specctl mcp
```

### Work on the dashboard

```bash
make dashboard-dev
```

## Documentation map

- [`skills/specctl/SKILL.md`](./skills/specctl/SKILL.md) — **primary agent-facing surface**
- [`SPEC.md`](./SPEC.md) — core behavioral specification
- [`ARCHITECTURE.md`](./ARCHITECTURE.md) — package responsibilities and boundaries
- [`CONTRIBUTING.md`](./CONTRIBUTING.md) — local development and governance workflow
- [`SECURITY.md`](./SECURITY.md) — vulnerability reporting policy
- [`RELEASING.md`](./RELEASING.md) — release/version/install policy
- [`examples/`](./examples) — starter usage examples
- [`docs/oss-launch-ops.md`](./docs/oss-launch-ops.md) — required repo settings and human signoff evidence

## Status

Strengths:
- strong behavioral governance
- clear Go module boundary
- good contract-test coverage
- narrow MCP adapter spec now exists
- clear skill-backed story once extracted

Remaining follow-up:
- refine a few overlapping ownership areas inherited from the monorepo
- formalize release/CI workflow in the extracted repo
