# specctl

Specification governance for agent workflows.

**Keep agents aligned with intended behavior, not just current code.**

If `git` helps teams track the history of source code, `specctl` helps agent
workflows track the history of **intent**.

`specctl` gives agents a durable source of truth for:

- what behavior matters
- what changed intentionally
- what has been verified
- what the next legal move is

That is the useful metaphor:

- `git` tracks how code changed
- `specctl` tracks how intended behavior changed
- `git` records commits
- `specctl` records deltas, requirements, verification, and revisions

Without that, agents drift: they infer too much from code, lose intent, skip verification, and declare partial work done. `specctl` exists to keep an agent inside a governed loop.

## Why it exists

Most agent loops need more than:

- a model
- a prompt
- a codebase

They also need a workflow that remembers:

- the behavioral contract
- the active change set
- the verification state
- the checkpoint/revision boundary

That is what `specctl` provides.

The core loop is:

```text
context -> delta -> requirement -> verify -> close -> bump/sync
```

At each step, `specctl` returns explicit `next` guidance so the agent does not have to invent process on the fly.

## Without vs with specctl

### Without specctl

- the agent edits code directly
- intent stays implicit
- verification is ad hoc
- partial work can look complete

### With specctl

- intent is tracked
- change is explicit
- verification is recorded
- the next legal step is constrained

## What ships

- **CLI** — command surface for governed spec operations
- **MCP server** — stdio MCP adapter exposing `specctl_*` tools
- **Packaged skill** — the primary agent-facing entrypoint
- **Self-governed example** — `specctl` ships its own governed spec as the built-in example
- **Dashboard** — optional embedded governance UI

## Install

### 1. Install the binary

```bash
go install github.com/aitoroses/specctl/cmd/specctl@latest
```

For stable consumers, prefer a tagged version once releases exist:

```bash
go install github.com/aitoroses/specctl/cmd/specctl@vX.Y.Z
```

### 2. Install the packaged skill

```bash
npx skills add https://github.com/aitoroses/specctl --skill specctl --global
```

## Use with Codex

1. Install the binary
2. Install the packaged skill
3. Run the packaged setup path:

```bash
bash skills/specctl/scripts/setup.sh
```

That setup installs/configures:

- the `specctl` binary
- local or global MCP configuration

After that, the normal entrypoint is the skill plus `specctl context ...`.
Treat `specctl context` as the triage surface: when it reports
review-required drift, follow its `review_diff` handoff to
`specctl diff` before deciding between sync and new tracked work.

## Use with Claude Code

1. Install the binary
2. Install the packaged skill
3. Run the packaged setup path:

```bash
bash skills/specctl/scripts/setup.sh --global
```

This configures the MCP server in the expected Claude-facing config path and keeps the skill as the main operating surface.

## Setup targets

The packaged setup script supports multiple targets:

```bash
# project-local .mcp.json
bash skills/specctl/scripts/setup.sh

# Claude Code global MCP config
bash skills/specctl/scripts/setup.sh --claude-global

# Codex global MCP config
bash skills/specctl/scripts/setup.sh --codex-global

# both global locations
bash skills/specctl/scripts/setup.sh --global
```

Repeated runs should converge the `specctl` MCP entry instead of duplicating it.

## First run

If you want to understand the product shape immediately:

```bash
specctl example
```

That returns the built-in governed example:

- `SPEC.md`
- `SPEC_FORMAT.md`
- `.specs/specctl.yaml`
- `.specs/specctl/CHARTER.yaml`
- `.specs/specctl/cli.yaml`

If you want to start operating on a real governed surface:

```bash
specctl context <charter:slug>
```

Examples:

```bash
specctl context specctl:cli
specctl context specctl:mcp
specctl context specctl:dashboard
specctl context specctl:skill
```

If drift is reported, the next step is normally:

```bash
specctl diff <charter:slug>
```

If you only remember one idea, remember this:

> Git tracks how code changed.  
> specctl tracks how intended behavior changes.

## Core workflow

Typical agent-driven sequence:

```bash
specctl context <charter:slug>
# if context reports review-required drift
specctl diff <charter:slug>
specctl delta add ...
# edit SPEC.md
specctl req add|replace|refresh ...
# implement code/tests
specctl req verify ...
specctl delta close ...
specctl rev bump ...
```

Key rule:

- write meaning in `SPEC.md`
- use `specctl` to mutate tracking state

Do **not** hand-edit `.specs/*.yaml`.

### Upgrading existing repos

Some config keys default differently depending on whether the repo was
initialized with the current `specctl init` or pre-dates the key.

- `auto_rebind_on_replace` — controls whether `specctl req replace` updates
  independent open/in-progress/deferred deltas whose `affects_requirements`
  lists the replaced requirement. Fresh `specctl init` writes this as
  `true`. Existing `.specs/specctl.yaml` files that omit the key get
  `false` at load time (backwards-compatible).

If you want auto-rebind in an existing repo, add the key explicitly:

```yaml
# .specs/specctl.yaml
auto_rebind_on_replace: true
```

When the flag is off (or absent), the absence of `result.auto_rebinds` in a
`req replace` response is the signal that rebinding was skipped. Use
`specctl delta rebind-requirements --from REQ-X --to REQ-Y` per delta if
you prefer the explicit path.

## For agents vs humans

### Agents

The primary agent-facing surface is:

- [`skills/specctl/SKILL.md`](./skills/specctl/SKILL.md)

Agents should use:

- the packaged skill
- CLI/MCP surfaces
- `next` guidance

### Humans

Humans mostly:

- install/configure the tool
- maintain specs and code
- review governance state
- decide policy
- approve merges/releases

In other words:

- humans define intent and boundaries
- agents operate the workflow

## Documentation

- [`skills/specctl/SKILL.md`](./skills/specctl/SKILL.md) — primary agent-facing entrypoint
- [`SPEC.md`](./SPEC.md) — core behavioral specification
- [`ARCHITECTURE.md`](./ARCHITECTURE.md) — package boundaries and system shape
- [`CONTRIBUTING.md`](./CONTRIBUTING.md) — maintainer/development workflow
- [`SECURITY.md`](./SECURITY.md) — vulnerability reporting policy
- [`RELEASING.md`](./RELEASING.md) — release/version/install policy
- [`examples/`](./examples) — quickstart examples
- [`docs/oss-launch-ops.md`](./docs/oss-launch-ops.md) — repo settings / launch evidence

## Development

Build:

```bash
make build
```

Install locally:

```bash
make install
```

Run tests:

```bash
make test-go
```

Dashboard typecheck/build:

```bash
make dashboard-typecheck
make dashboard-build
```

## License

MIT
