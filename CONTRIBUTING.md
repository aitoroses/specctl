# Contributing to specctl

This project is self-governed. Code changes and spec changes should move together.

Also: this is an **agent-facing tool**. Humans maintain it, but the normal operator is an agent using the skill and then the CLI/MCP surfaces.

## Prerequisites

- Go `1.26.x`
- `pnpm` for dashboard work

## Local development

### Build

```bash
make build
```

### Install

```bash
make install
```

### Run tests

```bash
make test-go
```

### Dashboard

```bash
make dashboard-dev
make dashboard-build
```

## Governance workflow

## Primary usage model

The intended order of surfaces is:

1. human installs/configures specctl
2. agent reads/uses [`skills/specctl/SKILL.md`](./skills/specctl/SKILL.md)
3. agent calls CLI or MCP tools
4. human intervenes for policy, review, or debugging

When changing the repo, prefer improvements that make agent behavior clearer and safer over improvements that only make ad-hoc manual CLI use nicer.

### Rules

- **Do not hand-edit** `.specs/specctl/*.yaml`
- Write meaning in `SPEC.md` / `internal/mcp/SPEC.md`
- Use `specctl` commands to register, replace, verify, close, and bump
- Keep [`skills/specctl/SKILL.md`](./skills/specctl/SKILL.md) aligned with the real product behavior

### Typical change flow

```bash
specctl context specctl:cli
specctl delta add ...
# edit SPEC.md
specctl req add|replace|refresh ...
# implement code/tests
specctl req verify ...
specctl delta close ...
specctl rev bump ...
```

### MCP-specific changes

Use `specctl:mcp` when the behavior is transport-specific:

- tool registration
- uninitialized fallback
- MCP hint translation
- MCP envelope behavior

Do **not** move core semantics like drift rules or requirement lifecycle into `specctl:mcp`.

### Skill-specific changes

If the behavioral workflow changes but the skill is not updated, the product is effectively inconsistent for its main user (the agent).

When reviewing a change, ask:

- Does the skill still describe the correct workflow?
- Does `next` still match the skill’s guidance?
- Would an agent use the tool correctly after this change?

## Review checklist

Before committing:

- docs updated if external behavior changed
- specs and tracking state aligned
- generated artifacts not committed
- binaries not committed
- runtime state under `.omc/` not committed

## Commit quality

This repo uses structured, rationale-first commit messages. Prefer:

- why the change was made
- what constraint shaped it
- what you rejected
- what you tested
