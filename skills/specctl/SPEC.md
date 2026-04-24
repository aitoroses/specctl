---
spec: skill
charter: specctl
format: behavioral-spec
---

# specctl Packaged Skill

The packaged skill is the primary human-facing entrypoint for an agent to learn how to use `specctl` correctly. It explains when the tool should be invoked, how the ownership split works, how `next` guidance should be followed, and how setup should install/configure the binary and MCP server without the agent inventing its own workflow.

## Skill Guidance Surface

The packaged skill gives agents a high-level operating model for using `specctl` as a governance engine rather than as an ad-hoc CLI. The skill should consistently frame `specctl` as agent-facing first and direct the agent toward the intended workflow: context first, then deltas/requirements/verification, with MCP support when available.

### Data Model

- Skill file: `skills/specctl/SKILL.md`
- Packaged README: `skills/specctl/README.md`
- Setup script: `skills/specctl/scripts/setup.sh`
- Reference docs: `skills/specctl/references/*`

### Contracts

Public skill install path:
```text
npx skills add https://github.com/aitoroses/specctl --skill specctl --global
```

### Invariants

- The skill is treated as a first-class product surface
- The skill frames `specctl` as agent-facing first
- The skill describes the intended governed workflow instead of encouraging ad-hoc mutation of tracking files
- The skill explains that `specctl context` warnings are advisory status, not permission to hand-edit tracking YAML
- The skill distinguishes stronger workflow-driving `next` actions from fallback advisory cleanup guidance like `review_warnings`

## Requirement: Packaged skill teaches the agent-first governance workflow

```gherkin requirement
@specctl @manual
Feature: Packaged skill teaches the agent-first governance workflow
```

### Scenarios

```gherkin scenario
Scenario: Skill frames specctl as an agent-facing tool
  Given an agent opens the packaged skill
  When it reads the overview and interaction guidance
  Then it is told to treat specctl as a governance engine for agents
  And it is directed toward context and next-action driven workflow
```

## Skill Setup Surface

The packaged setup path should install the binary and configure the MCP server from the packaged skill surface. A stranger following the public skill entrypoint should not need to rediscover hidden install steps. The setup path must also handle the fact that Claude-oriented clients and Codex do not use the same global config format.

### Data Model

- Setup script path: `skills/specctl/scripts/setup.sh`
- Project JSON target: local `.mcp.json`
- Claude global target: `~/.claude/.mcp.json`
- Codex global target: `~/.codex/config.toml`

### Invariants

- The setup path is reachable from the packaged skill surface
- Setup installs the `specctl` binary and configures MCP for both Claude-style JSON config and Codex TOML config
- Re-running setup converges the `specctl` entry instead of duplicating or silently preserving stale values
- The packaged-skill install instructions stay placeholder-free in public docs

## Requirement: Packaged skill setup path installs and configures specctl for Claude Code and Codex

```gherkin requirement
@specctl @manual
Feature: Packaged skill setup path installs and configures specctl for Claude Code and Codex
```

### Scenarios

```gherkin scenario
Scenario: Skill setup path is explicit and usable
  Given a maintainer or agent follows the packaged skill documentation
  When they run the documented setup path
  Then the specctl binary is installed
  And MCP configuration instructions are explicit
  And the target client config is updated coherently

```gherkin scenario
Scenario: Re-running setup repairs stale specctl config
  Given an existing stale specctl MCP entry is present
  When the user re-runs the documented setup path
  Then the specctl entry is updated to the current expected shape
  And unrelated config is preserved
```
```

## Retract, Rebind, and Repair-Intent Guidance

The packaged skill must teach the agent how to react to three
lifecycle states that today lack clean escape hatches: a delta opened
in error, a delta whose affected requirements were superseded under
it, and a `delta add --intent repair` that collides with closed-delta
invariants. Without skill-level guidance, the agent falls back to
`defer` plus a second `delta add`, which burns a D-id and creates
permanent residue in the tracking YAML.

### Invariants

- The skill documents `delta withdraw` as the verb to retract a
  non-closed delta opened in error, and explicitly contrasts it with
  `delta defer` ("not now, maybe later").
- The skill documents `delta rebind-requirements` for explicit
  re-anchoring and describes the `auto_rebind_on_replace` config that
  gates automatic rebinding on `req replace`.
- The skill documents the `VALIDATION_FAILED` response shape emitted
  by `delta add --intent repair` when closed-delta invariants would
  block the resulting `req stale`, including the suggested
  `--intent change` redirect.

## Requirement: Skill guides retraction rebind and repair-intent validation

```gherkin requirement
@specctl @manual
Feature: Skill guides retraction rebind and repair-intent validation
```


## Observable Reason Fields and Config Defaults

The skill's high-level escape-hatch guidance (REQ-004) tells agents when to
use withdraw, rebind, and repair-intent validation. This requirement
layers on top of it: it pins the exact observable shape of the reason
fields for each verb, documents the `auto_rebind_on_replace` default
divergence between `specctl init` and pre-existing repos, and gives a
concrete retry walkthrough for the closed-delta-invariant rejection.

### Invariants

- The skill names the exact JSON paths where a withdrawal reason is
  observable on both the write result and the state projection.
- The skill calls out that `result.rebind.reason` is emitted on both
  `--to` and `--remove` paths when a reason is supplied, and that its
  absence on `--to` without a reason is intentional.
- The skill tells the agent that the absence of `result.auto_rebinds`
  after `req replace` is the signal that `auto_rebind_on_replace` is
  off.
- The skill documents the `auto_rebind_on_replace` default divergence
  (`specctl init` → `true`, pre-existing repos → `false`) with an
  inspection-and-flip recipe.
- The skill includes a concrete retry walkthrough for the repair-intent
  `VALIDATION_FAILED` payload, naming the exact `focus.delta_add` keys.

## Requirement: Skill documents observable reason fields and config defaults for governance verbs

```gherkin requirement
@specctl @manual
Feature: Skill documents observable reason fields and config defaults for governance verbs
```
