# specctl

Agent-facing specification governance for behavioral contracts.

`specctl` is primarily a **tool for agents**, not a human-first CLI. It governs what a system should do, tracks whether those behaviors are implemented and verified, and returns explicit next-step guidance so an agent can stay inside legal workflow transitions.

For humans, the main entrypoint is usually the packaged skill:

- [`skills/specctl/SKILL.md`](./skills/specctl/SKILL.md)

That skill teaches an agent when to use specctl, how to interpret `next`, and how to move through the governed workflow without inventing its own process.

## What it includes

- **CLI** — an agent-consumable command surface for spec creation, deltas, requirements, verification, and revision management
- **MCP adapter** — stdio MCP server exposing `specctl_*` tools
- **Dashboard** — optional Vite/React governance UI embedded into the Go binary
- **Self-governance** — specctl governs itself via `.specs/specctl/*` and `SPEC.md`

## How to think about it

Humans usually do **not** sit and drive every `specctl` command manually.
The intended model is:

1. a human installs/configures the tool once
2. an agent uses the skill + MCP/CLI surfaces
3. the agent follows `next` guidance to stay inside legal transitions

So the product should be evaluated as:

- **a tool for agents**
- **a skill-backed workflow surface**
- **a governance engine with adapters**

not as a traditional human-operated CLI product.

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

### For humans

Humans mostly:

- install/build the tool
- maintain specs and code
- debug edge cases
- review governance state
- evolve the skill/documentation

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

## Repository follow-up notes

Remaining follow-up:

- choose and add a `LICENSE` file if desired
- decide whether dashboard stays first-class or optional
- finish ownership cleanup for `internal/cli/mcp.go`

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
