---
spec: cli
charter: specctl
format: behavioral-spec
---
# specctl v2 — Behavioral Specification

This document defines the observable behaviors of specctl v2, an
agent-facing specification engine. specctl governs the lifecycle of
specifications: tracking what behaviors a system should exhibit,
whether those behaviors are implemented and tested, and guiding the
agent through the workflow of defining, implementing, and verifying
each behavioral contract.

The core principle: a specification documents **observable behavior**
from the external surface. Not implementation details, not internal
architecture — what a user or caller sees. Every requirement in this
system represents a testable behavioral contract.

**specctl is:** a spec registry, a requirement registry, a
collaboration engine, a context engine, a change-management system,
a lifecycle state machine. Its read API returns context plus explicit
next-step semantics. Its write API applies validated mutations
atomically. It keeps agents inside legal specctl transitions.

**specctl is not:** a planning system, an execution system, a test
runner, a documentation generator, or a source-of-truth editor for
requirement prose or scenarios.

**Core ownership split:**

- `SPEC.md` owns meaning (prose, scenarios, contract richness)
- specctl owns collaboration state (lifecycle, verification, delta
  status, match integrity, next-step guidance)
- git owns code history

**Mutation boundary:**

- CLI-managed only: tracking YAML, charter YAML
- Mixed ownership: `specctl.yaml` (CLI manages tags/prefixes,
  humans author formats)
- Directly edited: design documents (`SPEC.md`)

Agents never hand-edit tracking YAML or charter YAML. They leave
specctl to edit `SPEC.md` and implement code, then return to
register, validate, verify, and align that work.

specctl validates and tracks requirement content in SPEC.md but never
writes semantic requirement text into that document. Agents leave specctl
to edit SPEC.md, implement code, or write tests; they return to specctl
to register, validate, verify, and align that work.

**Agent-first principles:**

- Derive, don't type: title from `Feature:` line, tags from `@`
  lines, status from state, changelog from closed deltas
- Help as specification: every `--help` is a mini-spec an agent
  can act on without reading this document

### Help Text Contract

Every command's `--help` output is a specification an agent can act on
without reading this document. Help text must include:

- Command meaning in the collaboration model
- Stdin format with required vs optional fields
- Example invocation
- Output envelope shape
- Error codes the command can return
- Decision criteria when this command appears in `next`

For requirement commands (`req add`, `req replace`, `req refresh`), help
must explicitly teach:

- A requirement is an observable behavior that should be tested
- Requirement-level Gherkin is the contract match surface
- Scenarios belong in SPEC.md
- Write from the external surface, not implementation internals

### Design Decisions

Key behavioral decisions governing this specification:

- Every tracking file points at exactly one design document
- Intent (add/change/remove/repair) drives the requirement workflow
- Requirement title and tags are derived from Gherkin, never free-text
- Delta provenance is recorded via `origin_checkpoint` on every delta
- Help text is a specification contract, not just documentation
- `@e2e` is a verification mode; `requirement` is the domain-neutral noun

## 1. Context Read Surface

specctl context is the agent's primary entry point. It reads the
tracking file and produces a complete picture of the spec's health:
current status (draft/ready/active/verified), drift state relative to
the last checkpoint, requirement match integrity, and — critically —
the **next actions** the agent should take. The next actions are what
make specctl agent-first: the tool doesn't just report state, it tells
the agent what to do about it. When committed drift is present,
`specctl context` stays in the triage role: it classifies whether review
is required and hands the agent to `specctl diff` before the agent
chooses semantic tracking vs checkpoint cleanup.

Registry- and charter-level context may also attach advisory `focus`
summaries even when there is no urgent `next` action. Those summaries
help the operator see where deferred work clusters or drifted surfaces
exist without turning optional review into false urgency.

### Data Model

The context projection reads and assembles these tracking file fields:

| Field | Type | Description |
|-------|------|-------------|
| `slug` | `string` | Spec identifier (`^[a-z0-9][a-z0-9-]*$`) |
| `charter` | `string` | Charter namespace |
| `title` | `string` | Human-readable spec title |
| `status` | `draft\|ready\|active\|verified` | Derived from delta/requirement/match state |
| `rev` | `int >= 1` | Current revision number |
| `checkpoint` | `string` | Git SHA of last verified baseline |
| `created` | `YYYY-MM-DD` | Spec creation date |
| `updated` | `YYYY-MM-DD` | Last write timestamp |
| `last_verified_at` | `YYYY-MM-DD` | Last verification date |
| `scope_drift` | `object` | `{status, checkpoint, drift_source, tracked_by[], files_changed_since_checkpoint[]}` |
| `uncommitted_changes` | `string[]` | Dirty paths under `scope[]` from `git status --porcelain` |
| `warnings[]` | `array` | Advisory spec-context warnings with `{kind, code, severity, message, delta_ids[], requirement_ids[], details}` |
| `requirements[]` | `array` | Each with `id, title, tags, lifecycle, verification, match.status, spec_context.scenarios[]` |
| `actionable_unverified_requirements[]` | `array` | Actionable non-verified requirements from the current truth surface only (`lifecycle: active`) |
| `inactive_unverified_requirements[]` | `array` | Non-verified inactive requirements (`lifecycle: superseded|withdrawn`) retained for audit/debugging and cleanup context |
| `deltas` | `object` | Summary counts (`open, in_progress, closed, deferred`) plus `items[]` |
| `validation` | `object` | `{valid: bool, findings[]}` |

### Contracts

Success (clean):
```json
{
  "state": {
    "slug": "session-lifecycle",
    "charter": "runtime",
    "status": "verified",
    "rev": 9,
    "checkpoint": "a1b2c3f",
    "scope_drift": { "status": "clean" },
    "uncommitted_changes": [],
    "validation": { "valid": true, "findings": [] }
  },
  "focus": {},
  "next": { "mode": "none" }
}
```

Success (clean warning-only residue):
```json
{
  "state": {
    "slug": "session-lifecycle",
    "charter": "runtime",
    "status": "verified",
    "scope_drift": { "status": "clean" },
    "uncommitted_changes": [],
    "warnings": [
      {
        "kind": "historical_residue",
        "code": "DEFERRED_SUPERSEDED_RESIDUE",
        "severity": "warning",
        "message": "Deferred deltas only reference superseded requirements already replaced by later closed work. Review whether governed cleanup is still needed.",
        "delta_ids": ["D-002"],
        "requirement_ids": ["REQ-001"],
        "details": {
          "dedupe_key": "deferred_superseded_residue:REQ-001",
          "replacement_requirement_ids": ["REQ-002"],
          "replacement_delta_ids": ["D-003"]
        }
      }
    ],
    "validation": { "valid": true, "findings": [] }
  },
  "focus": {},
  "next": {
    "mode": "sequence",
    "steps": [
      {
        "action": "review_warnings",
        "kind": "guidance"
      }
    ]
  }
}
```

Success (drifted):
```json
{
  "state": {
    "slug": "session-lifecycle",
    "charter": "runtime",
    "status": "active",
    "scope_drift": {
      "status": "drifted",
      "checkpoint": "a1b2c3f",
      "drift_source": "design_doc",
      "files_changed_since_checkpoint": ["runtime/src/domain/session_execution/SPEC.md"]
    }
  },
  "focus": {
    "scope_drift": {
      "drift_source": "design_doc",
      "review_required": true,
      "correctness_blocker": false,
      "housekeeping_candidate": true
    }
  },
  "next": {
    "mode": "choose_one",
    "options": [
      { "action": "review_diff", "template": { "argv": ["specctl", "diff", "runtime:session-lifecycle"] } },
      { "action": "sync", "choose_when": "review confirms the design-doc edit is clarification-only and behavior remains unchanged" }
    ]
  }
}
```

Error (SPEC_NOT_FOUND):
```json
{
  "state": { "target": "runtime:unknown-spec" },
  "focus": { "lookup": { "reason": "spec_not_found" } },
  "next": {
    "mode": "choose_one",
    "options": [{ "action": "create_spec" }]
  }
}
```

### Invariants

- If `uncommitted_changes[]` is non-empty, `next` prepends `stage_changes` and `commit_changes` before any specctl mutation step
- If `scope_drift.status == "drifted"` and review is required, `next` exposes `review_diff` as the canonical handoff to `specctl diff` before any sync cleanup choice
- If `scope_drift.status == "tracked"`, `next` continues the active delta/requirement workflow
- If `scope_drift.status == "unavailable"`, `next` begins with `specctl sync <target> --checkpoint HEAD`
- `focus.scope_drift` classifies whether the current drift is an immediate correctness blocker or a review-first housekeeping candidate
- Residue such as `superseded_orphans` may remain visible in `focus`, but it does not replace drift triage when committed scope drift exists
- `state` is a projected view, never a raw YAML fragment
- `state.requirements[]` is the canonical requirement record; summary arrays are convenience views
- `state.actionable_unverified_requirements[]` contains only active requirements that still need action
- `state.inactive_unverified_requirements[]` contains inactive non-verified requirements for audit/debugging and cleanup context; it does not directly block `next`, delta close, or rev bump on its own
- Deferred counts alone do not create warnings or backlog-driving `next` steps
- `DEFERRED_SUPERSEDED_RESIDUE` appears only when deferred deltas point exclusively at superseded requirements already replaced by later closed work
- `state.warnings[]` remains visible even when stronger `next` actions suppress fallback warning review
- Clean context with only `review_warnings` fallback serializes as `next.mode = sequence`; otherwise clean + empty next remains `none`
- Registry- and charter-level advisory focus may summarize deferred or drifted surfaces while keeping `next.mode = none`
- `next` is never `null` and always present

## Requirement: Context classifies committed drift before checkpoint cleanup

```gherkin requirement
@specctl @read
Feature: Context classifies committed drift before checkpoint cleanup
```

#### Scenarios

```gherkin scenario
Scenario: Clean spec returns clean drift status
  Given a tracked spec with no changes since checkpoint
  When the agent runs specctl context
  Then status is clean and next is none
```

```gherkin scenario
Scenario: Uncommitted governed changes trigger staging guidance
  Given uncommitted changes exist under a governed scope
  When the agent runs specctl context
  Then next contains stage_changes and commit_changes steps
```

```gherkin scenario
Scenario: Committed drift is classified before the agent chooses sync
  Given committed changes exist since the checkpoint
  When the agent runs specctl context
  Then next offers choose_one between review_diff and sync
  And focus.scope_drift marks whether the cleanup is a non-blocking housekeeping candidate
```

## 2. Diff Surface

specctl diff compares the current working tree against the last verified
checkpoint and classifies what changed: was it the design document (spec
prose), the scope code (governed source files), or both? The diff
produces actionable branches — concrete `next` options the agent can
choose from: open a new delta to track the change, refresh a requirement
whose identity is unchanged, sync the checkpoint when review shows the
tracked behavior still holds, or repair if evidence is stale. The diff is
how the agent decides between "the spec changed" and "the checkpoint
must move but the governed behavior is still correct."

### Data Model

The diff projection compares current working tree against the checkpoint:

| Field | Type | Description |
|-------|------|-------------|
| `target` | `string` | Spec identifier (`charter:slug`) |
| `baseline` | `string` | Always `"checkpoint"` |
| `comparison` | `string` | Range `"<checkpoint>..HEAD"` |
| `drift_source` | `design_doc\|scope_code\|both` | What changed since checkpoint |
| `from` | `object` | `{rev, checkpoint, status}` at baseline |
| `to` | `object` | `{rev, checkpoint, status}` current |
| `design_doc` | `object` | `{path, changed: bool, sections_changed[]}` |
| `scope_code` | `object` | `{changed_files[]}` |
| `requirements.match_issues[]` | `array` | `{id, status}` for each mismatched requirement |
| `validation` | `object` | `{valid: bool, findings[]}` |

### Contracts

Success (with changes):
```json
{
  "state": {
    "target": "runtime:session-lifecycle",
    "baseline": "checkpoint",
    "comparison": "a1b2c3f..HEAD",
    "drift_source": "design_doc",
    "from": { "rev": 9, "checkpoint": "a1b2c3f", "status": "verified" },
    "to": { "rev": 9, "checkpoint": "a1b2c3f", "status": "active" },
    "design_doc": {
      "path": "runtime/src/domain/session_execution/SPEC.md",
      "changed": true,
      "sections_changed": [
        { "heading": "Requirement: Compensation stage 4 failure cleanup", "type": "modified", "lines": [120, 170] }
      ]
    },
    "scope_code": { "changed_files": [] },
    "requirements": { "match_issues": [] }
  },
  "focus": {
    "review_surface": {
      "sections_changed": ["..."],
      "classification": {
        "review_required": true,
        "correctness_blocker": false,
        "housekeeping_candidate": true
      }
    }
  },
  "next": {
    "mode": "choose_then_sequence",
    "options": [
      { "action": "record_additive_change", "choose_when": "net-new observable behavior", "template": { "argv": ["specctl", "delta", "add", "<target>", "--intent", "add", "--area", "<area>"] } },
      { "action": "record_behavior_change", "choose_when": "existing requirement is no longer current truth", "template": { "argv": ["specctl", "delta", "add", "<target>", "--intent", "change", "--area", "<area>"] } },
      { "action": "sync", "choose_when": "review confirms the change is clarification-only and governed behavior remains unchanged" }
    ]
  }
}
```

Success (no changes):
```json
{
  "state": {
    "target": "runtime:session-lifecycle",
    "baseline": "checkpoint",
    "drift_source": null,
    "design_doc": { "changed": false },
    "scope_code": { "changed_files": [] }
  },
  "focus": {},
  "next": { "mode": "none" }
}
```

Error (CHECKPOINT_UNAVAILABLE):
```json
{
  "state": { "target": "runtime:session-lifecycle" },
  "focus": { "scope_drift": { "status": "unavailable" } },
  "next": {
    "mode": "sequence",
    "steps": [{ "action": "sync", "template": { "argv": ["specctl", "sync", "<target>", "--checkpoint", "HEAD"] } }]
  },
  "error": { "code": "CHECKPOINT_UNAVAILABLE", "message": "Stored checkpoint SHA cannot be resolved" }
}
```

### Invariants

- Diff ignores bookkeeping fields: `status`, `rev`, `updated`, `last_verified_at`, `checkpoint`, `changelog`
- `drift_source` is classified as `design_doc`, `scope_code`, or `both` based on which files changed
- Every `next` option includes `why`, `choose_when`, and an executable `template.argv`
- Diff guidance separates review-first housekeeping candidates from immediate correctness blockers, but it never infers semantics on the agent's behalf
- Diff is a review surface, not an inference engine -- it never makes semantic decisions for the agent

## Requirement: Diff guides semantic changes and housekeeping re-anchors

```gherkin requirement
@specctl @read
Feature: Diff guides semantic changes and housekeeping re-anchors
```

### Scenarios

```gherkin scenario
Scenario: Scope code drift produces semantic review with change options
  Given committed code changes exist since the checkpoint
  When the agent runs specctl diff
  Then the response contains changed files and actionable delta branches
```

```gherkin scenario
Scenario: Design-doc clarification offers checkpoint re-anchor after review
  Given committed design-doc changes exist since the checkpoint
  When the agent runs specctl diff
  Then the response marks the drift as review-first and non-blocking
  And next includes sync for clarification-only cleanup after review
```

```gherkin scenario
Scenario: No changes since checkpoint returns empty diff
  Given no files changed since the checkpoint
  When the agent runs specctl diff
  Then the response indicates no semantic changes
```

```gherkin scenario
Scenario: Tracked drift continuation shows delta-aware diff
  Given an open delta tracks the drifted scope
  When the agent runs specctl diff
  Then the response shows drift is already tracked by the delta
```

## 14. Agent Workflow Guidance

specctl is agent-first: every command response includes `next` actions
that tell the agent what to do. The quality of those actions determines
whether the agent produces a specification or an inventory. When a delta
is opened with intent add or change, the `next` sequence includes a
`write_spec_section` step BEFORE the `req add` step. This step guides
the agent to ask the user what observable behavior is being introduced,
write prose explaining the surface and its rationale, then formalize
with Gherkin blocks. The proactive guidance ensures specifications
document intent, not just structure. If a format template is configured,
the guidance includes the template path so the agent can follow the
project's documentation standard.

### Data Model

`write_spec_section` step fields within `next.steps[]`:

| Field | Type | Description |
|-------|------|-------------|
| `action` | `"write_spec_section"` | Step action identifier |
| `kind` | `"edit_file"` | Agent performs external file editing |
| `priority` | `1` | Always first in the sequence |
| `path` | `string` | `documents.primary` path for the spec |
| `why` | `string` | Instruction to write prose before formalizing |
| `context.intent` | `add\|change` | Delta intent that triggered this step |
| `context.delta_id` | `string` | Delta ID (e.g. `D-002`) |
| `context.area` | `string` | Delta area description |
| `context.current` | `string` | Current state description |
| `context.target` | `string` | Target state description |
| `context.design_doc` | `string` | Path to the design document |
| `context.format_template` | `string\|null` | Template path if a format is configured; `null` otherwise |
| `context.guidance` | `string` | Detailed five-layer writing instructions |

The subsequent step is always the formalization command:

| Intent | Next step `action` | Template command |
|--------|-------------------|-----------------|
| `add` | `add_requirement` | `specctl req add <target> --delta <id>` |
| `change` | `replace_requirement` | `specctl req replace <target> <req-id> --delta <id>` |

### Contracts

Delta add (intent: add) next with write_spec_section:
```json
{
  "next": {
    "mode": "sequence",
    "steps": [
      {
        "action": "write_spec_section",
        "kind": "edit_file",
        "priority": 1,
        "path": "runtime/src/domain/session_execution/SPEC.md",
        "why": "Write the specification section in the design doc before formalizing the requirement. Ask the user about the behavior first.",
        "context": {
          "intent": "add",
          "delta_id": "D-002",
          "area": "Heartbeat timeout",
          "current": "Current gap",
          "target": "Target gap",
          "design_doc": "runtime/src/domain/session_execution/SPEC.md",
          "format_template": null,
          "guidance": "Ask the user what observable behavior this delta introduces... Write a specification section with five layers: (1) Prose, (2) Data Model, (3) Contracts, (4) Invariants, (5) Gherkin tracking..."
        }
      },
      {
        "action": "add_requirement",
        "kind": "run_command",
        "priority": 2,
        "template": { "argv": ["specctl", "req", "add", "runtime:session-lifecycle", "--delta", "D-002"], "stdin_format": "gherkin" },
        "why": "Record a net-new requirement for this delta."
      }
    ]
  }
}
```

Delta add (intent: change) next with write_spec_section:
```json
{
  "next": {
    "mode": "sequence",
    "steps": [
      {
        "action": "write_spec_section",
        "kind": "edit_file",
        "priority": 1,
        "path": "runtime/src/domain/session_execution/SPEC.md",
        "context": {
          "intent": "change",
          "delta_id": "D-002",
          "area": "Compensation cleanup rewrite",
          "current": "Current cleanup wording is outdated",
          "target": "Replace the tracked cleanup contract",
          "design_doc": "runtime/src/domain/session_execution/SPEC.md",
          "format_template": null,
          "guidance": "..."
        }
      },
      {
        "action": "replace_requirement",
        "kind": "run_command",
        "priority": 2,
        "template": { "argv": ["specctl", "req", "replace", "runtime:session-lifecycle", "REQ-001", "--delta", "D-002"], "stdin_format": "gherkin" },
        "why": "Replace the active requirement that this change supersedes."
      }
    ]
  }
}
```

### Invariants

- `write_spec_section` appears before `add_requirement` for `add` intent and before `replace_requirement` for `change` intent
- `write_spec_section` is never emitted for `remove` or `repair` intents (those do not introduce new spec prose)
- `priority: 1` for `write_spec_section`, `priority: 2` for the subsequent formalization command
- When `format_template` is `null`, `guidance` includes instructions to consider creating a format template
- When `format_template` is set, `guidance` references the template path so the agent follows the project standard

### Command Legality Matrix

| Command | Legal when | Illegal when | Error code |
|---------|------------|--------------|------------|
| `delta add --intent add` | net-new observable behavior, structurally valid | malformed input or invalid requirement references | `INVALID_INPUT` |
| `delta add --intent change` | active requirement no longer current truth, names affected active requirements | `affects_requirements` missing, unknown, or non-active | `INVALID_INPUT` |
| `delta add --intent remove` | behavior intentionally removed, names affected active requirements | `affects_requirements` missing, unknown, or non-active | `INVALID_INPUT` |
| `delta add --intent repair` | behavior true but evidence stale, names affected active requirements | `affects_requirements` missing, unknown, or non-active | `INVALID_INPUT` |
| `delta start` | delta is `open` | delta is not `open` | `DELTA_INVALID_STATE` |
| `delta defer` | delta is `open` or `in-progress` | delta is `closed` or `deferred` | `DELTA_INVALID_STATE` |
| `delta resume` | delta is `deferred` | delta is not `deferred` | `DELTA_INVALID_STATE` |
| `delta close` | all updates resolved, all touched reqs verified, no match issues | unresolved updates, unverified reqs, or match blocking | `UNVERIFIED_REQUIREMENTS`, `DELTA_UPDATES_UNRESOLVED`, `REQUIREMENT_MATCH_BLOCKING` |
| `req add` | delta requires `add_requirement` | delta intent does not support add | `INVALID_INPUT` |
| `req replace` | old req `active`, delta requires replacement | old req not active or delta mismatch | `INVALID_INPUT` |
| `req withdraw` | old req `active`, delta requires withdrawal | old req not active or delta mismatch | `INVALID_INPUT` |
| `req stale` | old req `active`, delta requires repair | old req not active or delta mismatch | `INVALID_INPUT` |
| `req refresh` | req `active`, identity unchanged, only match text needs updating | not active, no drift, or recorded workflow conflict | `INVALID_INPUT` |
| `req verify` | req `active`, matches SPEC.md, evidence valid | `superseded`/`withdrawn`, or match blocking | `REQUIREMENT_INVALID_LIFECYCLE`, `REQUIREMENT_MATCH_BLOCKING` |
| `rev bump` | status `verified`, semantic changes exist | not verified, no changes, or match blocking | `INVALID_INPUT`, `REQUIREMENT_MATCH_BLOCKING` |
| `sync` | reviewed housekeeping drift or checkpoint repair, no live deltas or match blocking | live deltas or match blocking | `INVALID_INPUT`, `REQUIREMENT_MATCH_BLOCKING` |
| `spec create` | charter exists, paths valid, frontmatter consistent | spec already exists, invalid paths, frontmatter mismatch | `SPEC_EXISTS`, `INVALID_PATH`, `PRIMARY_DOC_FRONTMATTER_MISMATCH` |
| `charter create` | charter does not yet exist, groups provided | charter already exists or missing group metadata | `CHARTER_EXISTS`, `CHARTER_GROUP_MISSING` |
| `charter add-spec` | spec tracking file exists, slug not already in charter | slug already registered or group not declared | `SPEC_EXISTS`, `CHARTER_GROUP_MISSING` |
| `charter remove-spec` | slug is registered in the charter | slug not found in charter | `CHARTER_SPEC_MISSING` |
| `config add-tag` | tag matches `^[a-z0-9][a-z0-9-]*$` | tag invalid or already present | `INVALID_INPUT` |
| `config remove-tag` | tag exists in `gherkin_tags` | tag not found | `INVALID_INPUT` |
| `config add-prefix` | prefix ends with `/` | prefix invalid or already present | `INVALID_INPUT` |
| `config remove-prefix` | prefix exists in `source_prefixes` | prefix not found | `INVALID_INPUT` |

## Requirement: Delta add guides spec section writing before formalization

```gherkin requirement
@specctl @lifecycle
Feature: Delta add guides spec section writing before formalization
```

### Scenarios

```gherkin scenario
Scenario: Delta add with intent add includes write_spec_section step
  Given a tracked spec with no open deltas
  When the agent adds a delta with intent add
  Then next contains write_spec_section before add_requirement
```

```gherkin scenario
Scenario: Delta add with intent change includes write_spec_section step
  Given a tracked spec with an active requirement
  When the agent adds a delta with intent change
  Then next contains write_spec_section before replace_requirement
```

```gherkin scenario
Scenario: Write spec section guidance asks the user about behavior
  Given the write_spec_section step is present in next
  When the agent reads the guidance
  Then it instructs asking the user what observable behavior this introduces
```

## 3. Delta Lifecycle

A delta is a unit of intentional change to a specification. Every
modification to tracked requirements flows through a delta: adding new
behaviors (intent: add), changing existing ones (intent: change),
removing behaviors (intent: remove), or repairing stale evidence
(intent: repair). The delta FSM enforces discipline: open → active →
closed, with defer/resume for interruptions. A delta cannot close until
all its requirements are verified — this is the core integrity
guarantee. The delta's `next` actions guide the agent through writing
the spec section, formalizing requirements, implementing, and verifying.

### Data Model

Delta fields stored in the tracking file:

| Field | Type | Description |
|-------|------|-------------|
| `id` | `D-NNN` | Sequential, no gaps |
| `area` | `string` | Human description of the change area |
| `intent` | `add\|change\|remove\|repair` | Semantic intent driving the workflow |
| `status` | `open\|in-progress\|closed\|deferred` | FSM state |
| `origin_checkpoint` | `string` | Git SHA when delta was opened |
| `current` | `string` | Description of current state |
| `target` | `string` | Description of target state |
| `notes` | `string` | Context for the change |
| `affects_requirements` | `string[]` | Required for change/remove/repair; REQ IDs that must be active |
| `updates` | `string[]` | Derived from intent: `add_requirement\|replace_requirement\|withdraw_requirement\|stale_requirement` |

FSM transitions: `open -> in-progress -> closed`, `open|in-progress -> deferred`, `deferred -> open`. Closed deltas never reopen.

### Contracts

Success (delta add):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "ready" },
  "focus": { "delta": { "id": "D-009", "intent": "add", "status": "open" } },
  "result": {
    "delta": { "id": "D-009", "intent": "add", "status": "open", "updates": ["add_requirement"] },
    "allocation": { "id": "D-009" }
  },
  "next": {
    "mode": "sequence",
    "steps": [
      { "action": "write_spec_section", "why": "Write the behavioral surface prose before formalizing with Gherkin" },
      { "action": "add_requirement", "template": { "argv": ["specctl", "req", "add", "<target>", "--delta", "D-009"] } }
    ]
  }
}
```

Success (delta close):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "verified" },
  "focus": { "delta": { "id": "D-009", "status": "closed" } },
  "result": { "delta": { "id": "D-009", "status": "closed" } },
  "next": {
    "mode": "sequence",
    "steps": [{ "action": "rev_bump", "template": { "argv": ["specctl", "rev", "bump", "<target>", "--checkpoint", "HEAD"] } }]
  }
}
```

Error (UNVERIFIED_REQUIREMENTS):
```json
{
  "state": { "slug": "session-lifecycle", "status": "active" },
  "focus": { "delta_close": { "reason": "requirements_unverified", "blocking_requirements": ["REQ-007"] } },
  "next": { "mode": "none" },
  "error": { "code": "UNVERIFIED_REQUIREMENTS", "message": "Delta D-009 has unverified active requirements: REQ-007" }
}
```

### Invariants

- Intent determines allowed updates: `add` -> `add_requirement`, `change` -> `replace_requirement`, `remove` -> `withdraw_requirement`, `repair` -> `stale_requirement`
- `affects_requirements` is required for `change`, `remove`, and `repair` intents; every referenced requirement must be `active`
- `affects_requirements` is omitted or empty for `add` intent
- Delta close requires: all tracked updates resolved, all active touched requirements verified, no blocking match issues
- Closed deltas never reopen; semantic corrections create new deltas
- Delta metadata is immutable once created except for `status`

## Requirement: Delta lifecycle governs units of change

```gherkin requirement
@specctl @lifecycle
Feature: Delta lifecycle governs units of change
```

### Scenarios

```gherkin scenario
Scenario: Delta add with intent creates an open delta
  Given a tracked spec with no open deltas
  When the agent adds a delta with intent add
  Then the delta is open and next guides requirement addition
```

```gherkin scenario
Scenario: Delta transitions follow the FSM
  Given an open delta exists
  When the agent starts, defers, and resumes the delta
  Then each transition succeeds and the status updates accordingly
```

```gherkin scenario
Scenario: Delta close succeeds when all requirements are verified
  Given all requirements in the delta are verified
  When the agent closes the delta
  Then the delta status becomes closed and next guides revision bump
```

```gherkin scenario
Scenario: Delta close fails with unverified requirements
  Given a delta has unverified requirements
  When the agent attempts to close the delta
  Then the response returns UNVERIFIED_REQUIREMENTS
```

### Journey: Additive Change (spec-first)

```text
context → diff → delta add --intent add → write_spec_section →
req add → implement_and_test → req verify → delta close → rev bump
```

Start: `scope_drift.status == "drifted"`, `drift_source == "design_doc"`,
human already wrote the requirement block in `SPEC.md`.
Terminal: new requirement is `active + verified`, spec is `verified + clean`.

### Journey: Additive Change (code-first)

```text
context (drift_source: scope_code) → delta add --intent add →
req add (FAILS: REQUIREMENT_NOT_IN_SPEC) → write_requirement_block →
req add (retry) → req verify → delta close → rev bump
```

Start: `drift_source == "scope_code"`, agent already implemented new
behavior but hasn't updated `SPEC.md`. The `req add` failure guides
the agent to write the spec section before retrying.
Terminal: new requirement is `active + verified`, spec is `verified + clean`.

## 4. Requirement Add and Replace

Requirements are the atomic units of behavioral specification. Each
requirement represents one observable behavior: "the system does X when
Y happens." The requirement's identity is its Gherkin block — tags plus
a Feature line — which must exist in both the tracking file and the
design document (SPEC.md). This dual-presence is the match integrity
contract: the spec document is the source of truth for humans, the
tracking file is the source of truth for the tool, and they must agree.

When the Gherkin block doesn't exist in SPEC.md yet, specctl guides
the agent to write the spec section first — including prose explaining
the behavior's purpose — before formalizing. This ensures specifications
are documents, not inventories.

### Data Model

Requirement fields stored in the tracking file:

| Field | Type | Description |
|-------|------|-------------|
| `id` | `REQ-NNN` | Sequential, no gaps |
| `title` | `string` | Derived from the Gherkin `Feature:` line |
| `tags` | `string[]` | Derived from Gherkin `@tag` lines; must be configured or built-in semantic tags |
| `lifecycle` | `active\|superseded\|withdrawn` | Current state |
| `verification` | `unverified\|verified\|stale` | Evidence state |
| `introduced_by` | `string` | Delta ID that introduced this requirement |
| `supersedes` | `string?` | REQ ID this replaces (on `req replace`) |
| `superseded_by` | `string?` | REQ ID that replaced this |
| `test_files` | `string[]` | Repo-relative paths to evidence files |
| `gherkin` | `string` | Exact normalized requirement-level block (tags + Feature line only) |
| `match.status` | `matched\|no_exact_match\|missing_in_spec\|duplicate_in_spec` | Match integrity against SPEC.md |

The canonical definition: a requirement is an observable behavior that
should be tested. A requirement may correspond to a UI journey, API
contract, protocol interaction, background workflow, or infrastructure
behavior. The `@e2e` tag is a verification mode, not the noun itself —
`@manual` is equally valid for requirements verified without automated tests.

### Requirement Block Structure

Each requirement block in the design document:

- Starts at a `## Requirement:` heading (h2 level) dedicated to one requirement
- Contains exactly one `gherkin requirement` fenced block immediately after the heading
- May contain zero or more `gherkin scenario` fenced blocks
- Ends at the next `## Requirement:` heading or end-of-document

Normalization rules for requirement matching are conservative:

- Normalize line endings to `\n`
- Trim trailing spaces
- Trim leading and trailing blank lines inside the fenced block
- Preserve words, case, punctuation, and tag order

Requirement IDs are allocated sequentially starting at REQ-001. Gaps in the sequence are invalid -- the next ID is always `max(existing) + 1`.

specctl validates only parseable requirement blocks (`## Requirement:` headings with `gherkin requirement` fences). It does not validate the rest of the markdown document — prose, data model sections, and other content remain human-owned and unvalidated.

### Contracts

Success (req add):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "requirement": { "id": "REQ-008", "lifecycle": "active", "verification": "unverified" } },
  "result": {
    "requirement": { "id": "REQ-008", "title": "Session resume from snapshot", "lifecycle": "active", "verification": "unverified" },
    "allocation": { "id": "REQ-008" }
  },
  "next": {
    "mode": "sequence",
    "steps": [
      {
        "action": "implement_and_test",
        "template": { "action": "edit_file" },
        "context": { "requirement": "REQ-008", "title": "Session resume from snapshot", "tags": ["runtime", "e2e"], "scope": ["runtime/src/domain/session_execution/"], "scenarios": ["Resume restores terminal state"], "verification_level": "e2e" }
      },
      { "action": "verify_requirement", "template": { "argv": ["specctl", "req", "verify", "runtime:session-lifecycle", "REQ-008", "--test-file", "<test_path>"] } }
    ]
  }
}
```

Success (req replace):
```json
{
  "state": { "slug": "session-lifecycle", "status": "active" },
  "focus": {
    "replaced_requirement": { "id": "REQ-006", "lifecycle": "superseded" },
    "new_requirement": { "id": "REQ-007", "lifecycle": "active", "verification": "unverified" }
  },
  "result": {
    "requirement": { "id": "REQ-007", "title": "Compensation stage 4 failure cleanup", "lifecycle": "active" },
    "allocation": { "id": "REQ-007" }
  },
  "next": {
    "mode": "sequence",
    "steps": [
      { "action": "implement_and_test", "template": { "action": "edit_file" }, "context": { "requirement": "REQ-007" } },
      { "action": "verify_requirement", "template": { "argv": ["specctl", "req", "verify", "<target>", "REQ-007", "--test-file", "<test_path>"] } }
    ]
  }
}
```

Error (REQUIREMENT_NOT_IN_SPEC):
```json
{
  "state": { "slug": "session-lifecycle", "status": "active" },
  "focus": { "req_add": { "reason": "requirement_not_in_spec" } },
  "next": {
    "mode": "choose_one",
    "options": [{ "action": "write_requirement_block", "template": { "action": "edit_file", "description": "Write the requirement block in SPEC.md in the parseable format, then retry req add" } }]
  },
  "error": { "code": "REQUIREMENT_NOT_IN_SPEC", "message": "Gherkin block not found in SPEC.md" }
}
```

Error (INVALID_GHERKIN):
```json
{
  "error": { "code": "INVALID_GHERKIN", "message": "malformed Gherkin block: no Feature line found", "focus": { "reason": "invalid_structure" } }
}
```

Error (INVALID_GHERKIN_TAG):
```json
{
  "error": { "code": "INVALID_GHERKIN_TAG", "message": "tag @unknown is not in configured gherkin_tags list", "focus": { "tag": "@unknown" } }
}
```

Error (REQUIREMENT_DUPLICATE_IN_SPEC):
```json
{
  "error": { "code": "REQUIREMENT_DUPLICATE_IN_SPEC", "message": "requirement block appears multiple times in SPEC.md (ambiguous match)", "focus": { "slug": "session-lifecycle", "occurrences": 2 } }
}
```

### Invariants

- `title` is always derived from the Gherkin `Feature:` line, never agent-supplied
- `tags` are always derived from the Gherkin `@tag` lines; non-semantic tags must be configured in `specctl.yaml`
- The exact normalized requirement-level Gherkin must exist in `SPEC.md` for `req add` and `req replace`
- `req replace` marks the old requirement as `superseded` and records `supersedes`/`superseded_by` relationships
- New requirements start as `lifecycle: active, verification: unverified`
- ID allocation is `max(existing) + 1` after strict sequence validation

## Requirement: Requirement add and replace track observable behaviors

```gherkin requirement
@specctl @write
Feature: Requirement add and replace track observable behaviors
```

### Scenarios

```gherkin scenario
Scenario: Req add registers a new requirement matched to SPEC.md
  Given a Gherkin block exists in SPEC.md and a delta is open
  When the agent pipes the Gherkin to req add
  Then the requirement is active and match status is matched
```

```gherkin scenario
Scenario: Req add fails when requirement block is not in SPEC.md
  Given the Gherkin block does not exist in SPEC.md
  When the agent pipes the Gherkin to req add
  Then the response returns REQUIREMENT_NOT_IN_SPEC with write guidance
```

```gherkin scenario
Scenario: Req replace supersedes the old requirement
  Given an active requirement exists under a change delta
  When the agent pipes replacement Gherkin to req replace
  Then the old requirement is superseded and the new one is active
```

### Journey: Behavior Change

```text
diff → delta add --intent change (affects_requirements: [REQ-NNN]) →
write_spec_section → req replace REQ-NNN → implement_and_test →
req verify → delta close → rev bump
```

Start: `SPEC.md` changed, an active requirement no longer states
current truth, human already wrote the updated block.
Terminal: old requirement is `superseded`, new requirement is
`active + verified`.

## 5. Requirement Mutation

Requirements have a lifecycle beyond add/verify. When a behavior is
intentionally removed, the requirement is **withdrawn**. When the
underlying code changes and the evidence is no longer trusted, the
requirement is marked **stale** — it needs re-verification. When the
Gherkin text in SPEC.md changes (typo fix, scenario rewrite), the
tracked copy is **refreshed** to match. These mutations keep the
tracking file honest: it reflects what the system actually does, not
what it did three months ago.

### Data Model

Lifecycle and verification are the two state dimensions that mutation commands affect:

| Lifecycle | Verification | Allowed | Meaning |
|-----------|--------------|---------|---------|
| `active` | `unverified` | yes | Current truth, evidence not yet recorded |
| `active` | `verified` | yes | Current truth, evidenced |
| `active` | `stale` | yes | Current truth, evidence no longer trusted |
| `superseded` | `unverified` | yes | Replaced before verification |
| `superseded` | `verified` | yes | Was evidenced, then replaced |
| `superseded` | `stale` | **no** | Stale only applies to active truth |
| `withdrawn` | `unverified` | yes | Removed before verification |
| `withdrawn` | `verified` | yes | Was evidenced, then removed |
| `withdrawn` | `stale` | **no** | Stale only applies to active truth |

### Contracts

Success (req withdraw):
```json
{
  "state": { "slug": "session-lifecycle", "status": "active" },
  "focus": { "requirement": { "id": "REQ-009", "lifecycle": "withdrawn" } },
  "result": { "requirement": { "id": "REQ-009", "lifecycle": "withdrawn" } },
  "next": {
    "mode": "sequence",
    "steps": [{ "action": "close_delta", "template": { "argv": ["specctl", "delta", "close", "<target>", "<delta-id>"] } }]
  }
}
```

Success (req stale):
```json
{
  "state": { "slug": "session-lifecycle", "status": "active" },
  "focus": { "requirement": { "id": "REQ-011", "verification": "stale" } },
  "result": { "requirement": { "id": "REQ-011", "verification": "stale" } },
  "next": {
    "mode": "sequence",
    "steps": [
      { "action": "implement_and_test", "template": { "action": "edit_file" }, "context": { "guidance": "Fix the code or tests so the requirement holds again." } },
      { "action": "verify_requirement", "template": { "argv": ["specctl", "req", "verify", "<target>", "REQ-011", "--test-file", "<test_path>"] } }
    ]
  }
}
```

Success (req refresh):
```json
{
  "state": { "slug": "session-lifecycle", "status": "active" },
  "focus": { "requirement": { "id": "REQ-006", "lifecycle": "active", "verification": "verified" } },
  "result": { "requirement": { "id": "REQ-006", "match": { "status": "matched" } } },
  "next": { "mode": "none" }
}
```

Error (REQUIREMENT_NOT_FOUND):
```json
{
  "state": { "slug": "session-lifecycle" },
  "focus": {},
  "next": { "mode": "none" },
  "error": { "code": "REQUIREMENT_NOT_FOUND", "message": "Requirement REQ-999 does not exist" }
}
```

### Invariants

- Only `active` requirements can be withdrawn (`req withdraw`) or staled (`req stale`)
- `req withdraw` sets `lifecycle: withdrawn`; no implementation step follows (behavior is removed)
- `req stale` sets `verification: stale`; implementation + re-verification steps follow
- `req refresh` updates only the stored Gherkin match text; it must not change lifecycle or create a new requirement
- `req refresh` is illegal when a live delta already records replacement, withdrawal, or repair for the requirement
- `stale` verification is only valid on `active` lifecycle; `superseded` and `withdrawn` cannot be stale

## Requirement: Requirement withdraw stale and refresh mutate state

```gherkin requirement
@specctl @write
Feature: Requirement withdraw stale and refresh mutate state
```

### Scenarios

```gherkin scenario
Scenario: Req withdraw marks a requirement as withdrawn
  Given an active requirement under a remove delta
  When the agent runs req withdraw
  Then the requirement lifecycle becomes withdrawn
```

```gherkin scenario
Scenario: Req stale marks a requirement for re-verification
  Given an active verified requirement under a repair delta
  When the agent runs req stale
  Then the requirement verification becomes stale
```

```gherkin scenario
Scenario: Req refresh updates the stored Gherkin block
  Given an active requirement with a matching SPEC.md block
  When the agent pipes updated Gherkin to req refresh
  Then the stored Gherkin is replaced and match status is rechecked
```

### Journey: Behavior Removal

```text
diff → delta add --intent remove (affects_requirements: [REQ-NNN]) →
req withdraw REQ-NNN → delta close → rev bump
```

Start: the spec intentionally removes an observable behavior.
Terminal: requirement is `withdrawn`, delta is `closed`. No
implement step follows because the behavior is removed.

### Journey: Behavior Repair

```text
context (blocking_requirement.verification: stale) →
delta add --intent repair (affects_requirements: [REQ-NNN]) →
req stale REQ-NNN → implement_and_test → req verify → delta close → sync
```

Start: requirement meaning is correct but evidence is stale or code
regressed. Terminal: requirement is re-verified, `next` suggests
`sync` (not `rev bump`) because no new semantic content was introduced.

## 6. Verification Surface

Verification is the proof step. `req verify` confirms that the
observable behavior described by a requirement is actually implemented
and tested. The agent provides `--test-file` paths pointing to the
tests that exercise the behavior. Verification enforces two gates:
the requirement must be in the `active` lifecycle (can't verify
something that's been withdrawn or superseded), and the Gherkin must
match SPEC.md exactly (can't verify against a stale definition). These
gates exist because verification is a claim: "this behavior works."
The claim must be precise.

### Data Model

Verification reads and writes these fields:

| Field | Type | Description |
|-------|------|-------------|
| `verification` | `unverified\|verified\|stale` | Updated to `verified` on success |
| `test_files` | `string[]` | Replaced with the provided `--test-file` set on verify |
| `match.status` | `matched\|no_exact_match\|missing_in_spec\|duplicate_in_spec` | Must be `matched` for verify to proceed |
| `lifecycle` | `active` | Must be `active`; `superseded` and `withdrawn` are rejected |

Semantic tag `@manual` allows `test_files: []` on verification. All other requirements need at least one `--test-file`.

### Contracts

Success (req verify):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "requirement": { "id": "REQ-008", "verification": "verified" } },
  "result": { "requirement": { "id": "REQ-008", "verification": "verified", "test_files": ["runtime/tests/e2e/test_resume.py"] } },
  "next": {
    "mode": "sequence",
    "steps": [{ "action": "close_delta", "template": { "argv": ["specctl", "delta", "close", "<target>", "<delta-id>"] } }]
  }
}
```

Error (REQUIREMENT_MATCH_BLOCKING):
```json
{
  "state": { "slug": "session-lifecycle", "status": "active" },
  "focus": {
    "req_verify": { "reason": "match_blocking" },
    "requirement_match_issues": [{ "id": "REQ-008", "status": "no_exact_match" }]
  },
  "next": {
    "mode": "choose_one",
    "options": [
      { "action": "req_refresh", "choose_when": "same requirement identity; only match text changes" },
      { "action": "delta_add_change", "choose_when": "existing requirement is no longer current truth" }
    ]
  },
  "error": { "code": "REQUIREMENT_MATCH_BLOCKING", "message": "Requirement REQ-008 has a blocking match issue" }
}
```

Error (TEST_FILES_REQUIRED):
```json
{
  "state": { "slug": "session-lifecycle" },
  "focus": { "req_verify": { "reason": "test_files_required" } },
  "next": { "mode": "none" },
  "error": { "code": "TEST_FILES_REQUIRED", "message": "At least one --test-file is required for non-manual requirements" }
}
```

### Invariants

- `req verify` is legal only for `active` requirements; `superseded` and `withdrawn` return `REQUIREMENT_INVALID_LIFECYCLE`
- `req verify` is blocked by any requirement-match issue on that requirement
- Every `--test-file` path must exist on disk; missing files return `TEST_FILE_NOT_FOUND`
- `test_files` is fully replaced (not appended) with each verify invocation
- `@manual` tagged requirements may be verified with `test_files: []`
- Verification rechecks match integrity before persisting

## Requirement: Verification enforces lifecycle and match integrity

```gherkin requirement
@specctl @write
Feature: Verification enforces lifecycle and match integrity
```

### Scenarios

```gherkin scenario
Scenario: Req verify marks a requirement as verified with test files
  Given an active unverified requirement with matching SPEC.md block
  When the agent runs req verify with test file paths
  Then the requirement verification becomes verified
```

```gherkin scenario
Scenario: Req verify rejects inactive lifecycle
  Given a superseded or withdrawn requirement
  When the agent attempts req verify
  Then the response returns REQUIREMENT_INVALID_LIFECYCLE
```

```gherkin scenario
Scenario: Req verify blocks on match issues
  Given a requirement with a mismatched SPEC.md block
  When the agent attempts req verify
  Then the response returns REQUIREMENT_MATCH_BLOCKING
```

```gherkin scenario
Scenario: Req verify requires test files for non-manual requirements
  Given a requirement without the manual tag
  When the agent runs req verify without test files
  Then the response returns TEST_FILES_REQUIRED
```

## 7. Revision Management

Revisions mark milestones. After all requirements in a delta are
verified and the delta is closed, `rev bump` increments the spec
version and records a changelog entry documenting what changed. The
bump is gated on closed deltas that haven't been recorded yet — not on
file diffs from the checkpoint. This makes rev bump resilient to
ordering: whether the agent syncs first or bumps first, the result is
correct. `sync` is the lighter operation: it re-anchors the checkpoint
without bumping the version, used when reviewed drift is housekeeping
only and the governed behavior still holds. The command does not infer
that conclusion from file type; the agent must review the diff first.

### Data Model

Revision management reads and writes these tracking file fields:

| Field | Type | Description |
|-------|------|-------------|
| `rev` | `int >= 1` | Incremented by `rev bump` |
| `checkpoint` | `string` | Git SHA; updated by both `rev bump` and `sync` |
| `last_verified_at` | `YYYY-MM-DD` | Advanced by `rev bump` and `sync` (not by other writes) |
| `updated` | `YYYY-MM-DD` | Advanced on every successful write |
| `changelog[]` | `array` | Each entry: `{rev, date, deltas_opened[], deltas_closed[], reqs_added[], reqs_verified[], summary}` |

Changelog entries are appended by `rev bump` and record structured fields derived from closed deltas not yet in the changelog.

### Contracts

Success (rev bump):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "verified", "rev": 10, "checkpoint": "<HEAD_SHA>", "scope_drift": { "status": "clean" } },
  "focus": {},
  "result": {
    "previous_rev": 9,
    "rev": 10,
    "previous_checkpoint": "a1b2c3f",
    "checkpoint": "<HEAD_SHA>",
    "changelog_entry": { "rev": 10, "date": "<today>", "deltas_closed": ["D-009"], "reqs_verified": ["REQ-008"], "summary": "Added session resume behavior" }
  },
  "next": { "mode": "none" }
}
```

Success (sync):
```json
{
  "state": { "slug": "session-lifecycle", "status": "verified", "checkpoint": "<HEAD_SHA>", "scope_drift": { "status": "clean" } },
  "focus": {},
  "result": { "previous_checkpoint": "a1b2c3f", "checkpoint": "<HEAD_SHA>" },
  "next": { "mode": "none" }
}
```

Error (status_not_verified):
```json
{
  "state": { "slug": "session-lifecycle", "status": "active" },
  "focus": { "rev_bump": { "reason": "status_not_verified" } },
  "next": { "mode": "none" },
  "error": { "code": "INVALID_INPUT", "message": "Rev bump requires verified status; current status is active" }
}
```

### Invariants

- `rev bump` requires: `status == verified`, at least one closed delta not yet in the changelog, `--checkpoint` flag, and a summary via stdin
- `rev bump` is illegal when blocking requirement-match issues remain on active requirements
- `sync` requires: no live semantic deltas (open or in-progress) and no blocking match issues
- `sync` requires `--checkpoint <sha>` and a summary via stdin
- `sync` is a reviewed re-anchor operation: use it only when the agent has concluded the drift is housekeeping-only and the governed behavior remains correct
- Both `rev bump` and `sync` advance `last_verified_at` and are the only paths from `tracked`/`drifted` -> `clean`
- Checkpoint must be a resolvable git SHA; unresolvable returns `CHECKPOINT_UNAVAILABLE`

## Requirement: Revision management re-anchors reviewed housekeeping drift

```gherkin requirement
@specctl @lifecycle
Feature: Revision management re-anchors reviewed housekeeping drift
```

### Scenarios

```gherkin scenario
Scenario: Rev bump increments version after verified delta close
  Given all deltas are closed and all requirements verified
  When the agent runs rev bump with a summary
  Then the revision increments and a changelog entry is recorded
```

```gherkin scenario
Scenario: Sync re-anchors checkpoint without bumping revision
  Given reviewed drift with no semantic spec changes
  When the agent runs sync with a checkpoint
  Then the checkpoint moves and drift status becomes clean
```

```gherkin scenario
Scenario: Sync accepts reviewed design-doc clarification drift
  Given design-doc drift was reviewed and determined to be clarification-only
  When the agent runs sync with a checkpoint
  Then the checkpoint moves without bumping the revision
```

### Journey: Reviewed Checkpoint Re-anchor

```text
context (drift_source: design_doc|scope_code|both) → diff →
[agent concludes behavior is unchanged and re-anchor is housekeeping] →
sync --checkpoint HEAD <<< summary
```

Start: `scope_drift.status == "drifted"`, review complete, no live
semantic ambiguity. Terminal: `scope_drift.status: "clean"`.

### Journey: Checkpoint Repair

```text
context (scope_drift.status: unavailable) →
sync --checkpoint HEAD <<< summary
```

Start: `scope_drift.status == "unavailable"` (stored checkpoint SHA
cannot be resolved). Terminal: checkpoint re-anchored, drift is `clean`.

## 8. Configuration Surface

Project configuration lives in `.specs/specctl.yaml`. It defines which
Gherkin tags are valid (`gherkin_tags`), which directory prefixes are
governed (`source_prefixes`), and which format templates are available
(`formats`). Tags and prefixes are CLI-managed (add-tag, remove-tag,
add-prefix, remove-prefix). Formats are human-authored — the CLI
preserves them during mutations. The config is the project's vocabulary:
it determines what tags an agent can use in requirement blocks and what
directories trigger hook notifications.

### Data Model

Config fields stored in `.specs/specctl.yaml`:

| Field | Type | Description |
|-------|------|-------------|
| `gherkin_tags` | `string[]` | CLI-managed; each matches `^[a-z0-9][a-z0-9-]*$` |
| `semantic_tags` | `string[]` | Built-in (`e2e`, `manual`); read-only, never in mutable config |
| `source_prefixes` | `string[]` | CLI-managed; each ends with `/` |
| `formats` | `map<string, format>` | Human-authored; read-only to CLI |
| `formats.<key>.template` | `string` | Repo-relative path to the format template file |
| `formats.<key>.recommended_for` | `string` | Glob pattern for auto-selection during `spec create` |
| `formats.<key>.description` | `string` | Human-readable description |

### Contracts

Success (config read):
```json
{
  "state": {
    "gherkin_tags": ["runtime", "domain"],
    "semantic_tags": ["e2e", "manual"],
    "source_prefixes": ["runtime/src/"],
    "formats": {},
    "validation": { "valid": true, "findings": [] }
  },
  "focus": {},
  "next": { "mode": "none" }
}
```

Success (config add-tag):
```json
{
  "state": {
    "gherkin_tags": ["adapter", "domain", "runtime"],
    "semantic_tags": ["e2e", "manual"],
    "source_prefixes": ["runtime/src/"],
    "formats": {},
    "validation": { "valid": true, "findings": [] }
  },
  "focus": { "config_mutation": { "kind": "add_tag", "value": "adapter" } },
  "result": { "kind": "config", "mutation": "add-tag", "tag": "adapter" },
  "next": { "mode": "none" }
}
```

Success (config remove-tag):
```json
{
  "state": {
    "gherkin_tags": ["runtime"],
    "semantic_tags": ["e2e", "manual"],
    "source_prefixes": ["runtime/src/"],
    "formats": {},
    "validation": { "valid": true, "findings": [] }
  },
  "focus": { "config_mutation": { "kind": "remove_tag", "value": "domain" } },
  "result": { "kind": "config", "mutation": "remove-tag", "tag": "domain" },
  "next": { "mode": "none" }
}
```

Error (INVALID_INPUT):
```json
{
  "state": { "command": "config", "args": ["unexpected"] },
  "focus": { "input": { "reason": "too_many_args", "max_args": 0, "received": 1 } },
  "next": { "mode": "none" },
  "error": { "code": "INVALID_INPUT", "message": "accepts 0 arg(s), received 1" }
}
```

### Invariants

- Tags must match `^[a-z0-9][a-z0-9-]*$` (lowercase kebab-case)
- Prefixes must end with `/`
- `gherkin_tags` and `source_prefixes` are CLI-managed; write commands must preserve `formats` semantically unchanged
- Semantic tags (`e2e`, `manual`) are built-in and never appear in mutable config
- Tag list is maintained as an ordered unique set (sorted alphabetically)
- Tracking-file `tags` and project-level `gherkin_tags` are validation-only labels — they constrain which tags are legal in Gherkin blocks but do not change CLI behavior. Semantic tags (`@e2e`, `@manual`) are built-in and behavior-changing; project tags are not.

## Requirement: Configuration manages project-level settings

```gherkin requirement
@specctl @write
Feature: Configuration manages project-level settings
```

### Scenarios

```gherkin scenario
Scenario: Config read returns current project configuration
  Given a specctl project with tags and prefixes
  When the agent runs specctl config
  Then the response contains gherkin_tags, semantic_tags, and source_prefixes
```

```gherkin scenario
Scenario: Config add and remove tag mutate the tag list
  Given a configured project
  When the agent adds then removes a tag
  Then the tag list reflects each mutation
```

```gherkin scenario
Scenario: Config add and remove prefix mutate the prefix list
  Given a configured project
  When the agent adds then removes a prefix
  Then the prefix list reflects each mutation
```

## Requirement: INVALID_GHERKIN_TAG guides the agent to register missing tags then retry

```gherkin requirement
@specctl @write
Feature: INVALID_GHERKIN_TAG guides the agent to register missing tags then retry
```

### Scenarios

```gherkin scenario
Scenario: INVALID_GHERKIN_TAG returns next steps to register and retry
  Given a requirement uses @webhook but "webhook" is not in gherkin_tags
  When the agent runs req add with that Gherkin
  Then the error returns INVALID_GHERKIN_TAG with next steps: config add-tag for each missing tag, then retry req add
```

### Journey: Config Mutation

```text
config add-tag <tag> → [next: none]
config remove-tag <tag> → [next: none]
config add-prefix <prefix>/ → [next: none]
config remove-prefix <prefix>/ → [next: none]
```

Config mutations are atomic and terminal. Each returns `next: none`
because configuration changes have no follow-up workflow steps.

## 9. Charter Management

Charters are organizational containers for related specs. A charter
groups specs into named groups with ordering, and provides the registry
that `specctl context` uses to resolve spec targets. Charter commands
are structural — they create the directory layout, manage CHARTER.yaml,
and add/remove spec membership. The charter is the entry point for a
new project: `charter create` is always the first command.

### Data Model

Charter fields stored in `.specs/{charter}/CHARTER.yaml`:

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Charter identifier; must equal directory name under `.specs/` |
| `title` | `string` | Human-readable charter title |
| `description` | `string` | Required, non-empty |
| `groups[]` | `array` | Named groupings with `key`, `title`, `order` |
| `groups[].key` | `string` | Matches `^[a-z0-9][a-z0-9-]*$`; unique within charter |
| `groups[].title` | `string` | Human-readable group title |
| `groups[].order` | `int >= 0` | Sort order within the charter |
| `specs[]` | `array` | Ordered spec membership entries |
| `specs[].slug` | `string` | Must correspond to a tracking file in the same charter directory |
| `specs[].group` | `string` | Must reference a declared group key |
| `specs[].order` | `int >= 0` | Sort order within the group |
| `specs[].depends_on` | `string[]` | DAG references to other slugs in the same charter |
| `specs[].notes` | `string` | Required, non-empty planning note |

### Contracts

Success (charter create):
```json
{
  "state": {
    "name": "runtime",
    "title": "Runtime System",
    "description": "Specs for runtime",
    "groups": [{ "key": "execution", "order": 10, "title": "Execution Engine" }],
    "ordered_specs": [],
    "tracking_file": ".specs/runtime/CHARTER.yaml",
    "validation": { "valid": true, "findings": [] }
  },
  "focus": {},
  "result": {
    "kind": "charter",
    "tracking_file": ".specs/runtime/CHARTER.yaml",
    "created_groups": [{ "key": "execution", "order": 10, "title": "Execution Engine" }]
  },
  "next": {
    "mode": "sequence",
    "steps": [{ "action": "create_spec", "template": { "argv": ["specctl", "spec", "create", "runtime:<slug>", "--title", "<title>", "--doc", "<design_doc>", "--scope", "<scope_dir_1>/", "--group", "execution", "--order", "<order>", "--charter-notes", "<charter_notes>"] } }]
  }
}
```

Success (charter add-spec):
```json
{
  "state": {
    "name": "runtime",
    "title": "Runtime System",
    "ordered_specs": [{ "slug": "session-lifecycle", "group": { "key": "recovery", "order": 20, "title": "Recovery and Cleanup" }, "order": 30, "depends_on": [], "notes": "Session FSM and cleanup behavior", "status": "ready" }],
    "validation": { "valid": true, "findings": [] }
  },
  "focus": {},
  "result": { "kind": "charter_entry", "entry": { "slug": "session-lifecycle", "group": "recovery", "order": 30, "depends_on": [], "notes": "Session FSM and cleanup behavior" } },
  "next": { "mode": "none" }
}
```

Success (charter remove-spec):
```json
{
  "state": {
    "name": "runtime",
    "ordered_specs": [],
    "validation": { "valid": true, "findings": [] }
  },
  "focus": {},
  "result": { "kind": "charter_entry", "removed_slug": "session-lifecycle" },
  "next": { "mode": "none" }
}
```

Error (CHARTER_NOT_FOUND):
```json
{
  "error": { "code": "CHARTER_NOT_FOUND", "message": "charter \"runtime\" does not exist" }
}
```

Error (CHARTER_GROUP_MISSING):
```json
{
  "error": { "code": "CHARTER_GROUP_MISSING", "message": "charter create requires group metadata but it's missing" }
}
```

Error (CHARTER_SPEC_MISSING):
```json
{
  "error": { "code": "CHARTER_SPEC_MISSING", "message": "charter \"runtime\" does not list spec \"session-lifecycle\"" }
}
```

Error (GROUP_REQUIRED):
```json
{
  "error": { "code": "GROUP_REQUIRED", "message": "new group needs title and order metadata", "focus": { "group": "<group_key>" } }
}
```

Error (SPEC_EXISTS):
```json
{
  "error": { "code": "SPEC_EXISTS", "message": "spec \"session-lifecycle\" is already registered in charter \"runtime\"", "focus": { "slug": "session-lifecycle", "charter": "runtime" } }
}
```

### Invariants

- Charter `name` must equal the directory name under `.specs/`
- `groups[].key` values must be unique within the charter
- `specs[].slug` values must be unique and correspond to tracking files in the same charter directory
- `depends_on` may reference only slugs in the same charter; self-dependency and cycles are invalid
- `specs[].group` must reference a declared group key
- Charter create always returns a `create_spec` next step

## Requirement: Charter commands manage the spec registry

```gherkin requirement
@specctl @write
Feature: Charter commands manage the spec registry
```

### Scenarios

```gherkin scenario
Scenario: Charter create initializes a charter with groups
  Given no charter exists for the given name
  When the agent creates a charter with groups via stdin
  Then the charter directory and CHARTER.yaml are created
```

```gherkin scenario
Scenario: Charter add-spec and remove-spec modify membership
  Given a charter with an existing spec
  When the agent adds or removes a spec from the charter
  Then the charter ordered_specs list reflects the change
```

## 10. Hook Integration

The git hook provides commit-time awareness. When staged files fall
under a governed scope, the hook reports which specs are affected. This
is informational only — the hook doesn't block commits. It lets the
agent know that a commit touched governed code, which might trigger
drift detection on the next `specctl context` call.

### Data Model

Hook input and output fields:

| Field | Type | Description |
|-------|------|-------------|
| `input_files` | `string[]` | Staged file paths piped via stdin |
| `affected_specs[]` | `array` | Each with `charter, slug, status, design_doc, tracking_file, matched_files[], design_doc_staged, tracking_file_staged` |
| `considered_files` | `string[]` | Input files that fell under a governed scope or matched a spec artifact |
| `ignored_files` | `string[]` | Input files outside all configured `source_prefixes` and not matching any spec artifact |
| `unmatched_files` | `string[]` | Files under a `source_prefix` but not owned by any spec scope |
| `validation` | `object` | `{valid: bool, findings[]}` — includes `UNOWNED_SOURCE_FILE` findings |

### Contracts

Success (hook with matches):
```json
{
  "state": {
    "input_files": [".specs/runtime/session-lifecycle.yaml", "runtime/src/domain/session_execution/services.py", "docs/notes.md"],
    "affected_specs": [{
      "charter": "runtime",
      "slug": "session-lifecycle",
      "status": "draft",
      "design_doc": "runtime/src/domain/session_execution/SPEC.md",
      "tracking_file": ".specs/runtime/session-lifecycle.yaml",
      "matched_files": ["runtime/src/domain/session_execution/services.py"],
      "design_doc_staged": false,
      "tracking_file_staged": true
    }],
    "considered_files": [".specs/runtime/session-lifecycle.yaml", "runtime/src/domain/session_execution/services.py"],
    "ignored_files": ["docs/notes.md"],
    "unmatched_files": [],
    "validation": { "valid": true, "findings": [] }
  },
  "focus": {},
  "next": { "mode": "none" }
}
```

Success (hook no matches):
```json
{
  "state": {
    "input_files": ["docs/notes.md"],
    "affected_specs": [],
    "considered_files": [],
    "ignored_files": ["docs/notes.md"],
    "unmatched_files": [],
    "validation": { "valid": true, "findings": [] }
  },
  "focus": {},
  "next": { "mode": "none" }
}
```

### Invariants

- Hook only considers files under configured `source_prefixes` or matching spec artifacts (`.specs/` paths, design docs)
- Hook is informational only — it never blocks commits and never mutates state
- Files under a `source_prefix` but not owned by any spec scope appear in `unmatched_files` with `UNOWNED_SOURCE_FILE` validation findings
- Hook does not compute full drift coverage or `tracked` status

## Requirement: Hook reports affected specs from staged files

```gherkin requirement
@specctl @read
Feature: Hook reports affected specs from staged files
```

### Scenarios

```gherkin scenario
Scenario: Hook identifies governed specs from staged file paths
  Given staged files fall under a governed scope
  When the git hook pipes staged paths to specctl hook
  Then the response lists affected specs with drift indicators
```

## 11. Ownership Resolution

`context --file` answers: "which spec governs this file?" It resolves
file paths against configured source prefixes to find the owning spec.
When exactly one spec matches, the agent knows where to direct changes.
When multiple specs overlap, the response reports ambiguity — the agent
can't act until the scope design is fixed. When no spec matches, the
response suggests creating a new spec or extending an existing scope.

### Data Model

`context --file` projection fields:

| Field | Type | Description |
|-------|------|-------------|
| `file` | `string` | Repo-relative input path |
| `resolution` | `matched\|ambiguous\|unmatched` | Outcome of ownership resolution |
| `match_source` | `design_doc\|scope\|null` | How the match was found |
| `matches[]` | `array` | Each with `charter, slug, match_source, scope_prefix?` |
| `governing_spec` | `object\|null` | `{charter, slug, tracking_file, documents.primary}` when exactly one match |
| `validation` | `object` | `{valid: bool, findings[]}` |

### Contracts

Success (file matched single):
```json
{
  "state": {
    "file": "runtime/src/domain/session_execution/SPEC.md",
    "resolution": "matched",
    "match_source": "design_doc",
    "matches": [{ "charter": "runtime", "slug": "session-lifecycle", "match_source": "design_doc" }],
    "governing_spec": { "charter": "runtime", "slug": "session-lifecycle", "tracking_file": ".specs/runtime/session-lifecycle.yaml", "documents": { "primary": "runtime/src/domain/session_execution/SPEC.md" } },
    "validation": { "valid": true, "findings": [] }
  },
  "focus": {},
  "next": { "mode": "none" }
}
```

Success (file ambiguous):
```json
{
  "state": {
    "file": "runtime/src/domain/shared/transport.py",
    "resolution": "ambiguous",
    "match_source": "scope",
    "matches": [
      { "charter": "runtime", "slug": "first-owner", "match_source": "scope", "scope_prefix": "runtime/src/domain/shared/" },
      { "charter": "runtime", "slug": "second-owner", "match_source": "scope", "scope_prefix": "runtime/src/domain/shared/" }
    ],
    "governing_spec": null,
    "validation": { "valid": false, "findings": [{ "code": "AMBIGUOUS_FILE_OWNERSHIP", "severity": "error" }] }
  },
  "focus": { "ownership": { "matches": ["..."] } },
  "next": { "mode": "none" }
}
```

Success (file no match):
```json
{
  "state": {
    "file": "adapters/src/http/client.py",
    "resolution": "unmatched",
    "match_source": null,
    "matches": [],
    "governing_spec": null,
    "validation": { "valid": true, "findings": [] }
  },
  "focus": { "ownership": { "reason": "no_governing_spec" } },
  "next": {
    "mode": "choose_one",
    "options": [{ "action": "create_spec", "template": { "argv": ["specctl", "spec", "create", "<charter>:<slug>", "--scope", "adapters/src/http/"] } }]
  }
}
```

### Invariants

- Scope matching is longest-prefix-wins when ordering multiple matches
- Exact `documents.primary` match takes priority over `scope[]` matching (returns `match_source: "design_doc"`)
- Nearby `SPEC.md` files do not govern adjacent code by proximity alone — only declared `scope[]` is authoritative
- When no match is found, `next` offers `create_spec`

## Requirement: File ownership resolution maps files to specs

```gherkin requirement
@specctl @read
Feature: File ownership resolution maps files to specs
```

### Scenarios

```gherkin scenario
Scenario: Context file matches a single governing spec
  Given a file path falls under exactly one spec scope
  When the agent runs context --file with that path
  Then the response identifies the owning spec
```

```gherkin scenario
Scenario: Context file reports ambiguity for overlapping scopes
  Given a file path falls under multiple spec scopes
  When the agent runs context --file with that path
  Then the response reports ambiguous ownership
```

```gherkin scenario
Scenario: Context file reports no match for ungoverned paths
  Given a file path is not under any spec scope
  When the agent runs context --file with that path
  Then the response reports no governing spec found
```

### Journey: File Ownership Resolution

```text
context --file <path> (resolution: unmatched) → spec create
context --file <path> (resolution: ambiguous) → [informational, no action]
```

Start: agent needs to know which spec governs a file. No match
suggests `spec create`. Ambiguous ownership is informational until
the scope design is fixed; specctl does not guess.

## 12. Match Integrity

Match integrity is the cross-cutting enforcement mechanism. Every
tracked requirement has a Gherkin block that must exactly match its
counterpart in the design document. When they diverge — because the
spec was edited without refreshing the tracked copy, or vice versa —
critical operations are blocked: `req verify` can't prove a behavior
against a stale definition, and `delta close` can't seal a delta with
unresolved mismatches. The agent is guided to either refresh the
tracked copy (if the spec is right) or update the spec (if the code
changed the contract).

### Data Model

Requirement match fields on each tracked requirement:

| Field | Type | Description |
|-------|------|-------------|
| `match.status` | `matched\|no_exact_match\|missing_in_spec\|duplicate_in_spec` | Match state against SPEC.md |
| `match.heading` | `string` | The `## Requirement: <title>` heading found in SPEC.md (present when heading located) |

Match issue shape surfaced in `focus.requirement_match_issues[]`:

| Field | Type | Description |
|-------|------|-------------|
| `id` | `string` | Requirement ID (e.g. `REQ-001`) |
| `status` | `string` | One of the non-`matched` match states |
| `heading` | `string` | The heading where the mismatch was detected |

### Contracts

Error (REQUIREMENT_MATCH_BLOCKING on verify):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": {
    "req_verify": { "reason": "match_blocking" },
    "requirement": { "id": "REQ-001", "match": { "status": "no_exact_match", "heading": "Requirement: Compensation stage 4 failure cleanup" } },
    "requirement_match_issues": [{ "id": "REQ-001", "status": "no_exact_match", "heading": "Requirement: Compensation stage 4 failure cleanup" }]
  },
  "next": {
    "mode": "choose_one",
    "options": [
      { "action": "refresh_requirement", "choose_when": "Same requirement identity; only match text changes." },
      { "action": "delta_add_change", "choose_when": "Existing requirement is no longer the correct statement of truth." },
      { "action": "delta_add_remove", "choose_when": "Observable behavior is intentionally removed." }
    ]
  },
  "error": { "code": "REQUIREMENT_MATCH_BLOCKING", "message": "req verify is blocked by requirement match issues" }
}
```

### Invariants

- `matched` = stored Gherkin exactly matches the normalized SPEC.md block under the requirement heading
- `no_exact_match` (drifted) = heading found in SPEC.md but the Gherkin content differs from the stored copy
- `missing_in_spec` = no requirement heading found in SPEC.md for this requirement
- `duplicate_in_spec` = more than one matching block found in SPEC.md
- Match issues on `active` requirements are blocking for `req verify`, `delta close`, and `rev bump`
- Match issues on `superseded` and `withdrawn` requirements are informational only (historical integrity warnings)

## Requirement: Match integrity blocks operations on mismatch

```gherkin requirement
@specctl @lifecycle
Feature: Match integrity blocks operations on mismatch
```

### Scenarios

```gherkin scenario
Scenario: Verify is blocked when requirement Gherkin mismatches SPEC.md
  Given a requirement whose stored Gherkin differs from SPEC.md
  When the agent attempts req verify
  Then the response returns REQUIREMENT_MATCH_BLOCKING with refresh guidance
```

```gherkin scenario
Scenario: Delta close is blocked by unresolved match issues
  Given a delta with requirements that have match issues
  When the agent attempts delta close
  Then the response returns REQUIREMENT_MATCH_BLOCKING
```

### Journey: Match Refresh

```text
context (requirement_match_issues: no_exact_match) →
req refresh REQ-NNN <<< updated gherkin → [continue normal workflow]
```

Start: requirement identity unchanged, match text drifted. No
lifecycle change needed. Terminal: match status returns to `matched`.

### Journey: Match Blocks Verify

```text
req verify REQ-NNN (FAILS: REQUIREMENT_MATCH_BLOCKING) →
choose_one: req refresh | delta add --intent change | delta add --intent remove
```

Start: active requirement exists but exact match to `SPEC.md` is
broken. Terminal: agent resolves via refresh or opens a delta.

### Journey: Match Blocks Close

```text
delta close (FAILS: REQUIREMENT_MATCH_BLOCKING) →
choose_one: req refresh | delta add --intent change | delta add --intent remove
```

Start: delta is otherwise ready to close but a touched active
requirement has a blocking match issue. Terminal: agent resolves
the match issue, then retries `delta close`.

## 13. Spec Creation

`spec create` is the birth of a governed specification. It creates the
tracking file, registers the spec in its charter, detects or creates
the design document, and sets the initial checkpoint to HEAD. The
command validates everything upfront: scope directories must exist,
frontmatter must be consistent, format references must resolve. When
a format is configured, auto-selection matches the doc path against
`recommended_for` globs — ambiguous matches are rejected. A spec
that passes creation is immediately ready for deltas and requirements.

When no format template is configured, spec create returns a
`create_format_template` next step (kind: `ask_user_then_edit`).
This step instructs the agent to interview the user about the system
being specified before creating the template. The interview covers:
system type, observable surfaces, caller interaction patterns, state
transitions, and quality bar. The step also provides the exact
`specctl.yaml` config schema (formats.<key> with template,
recommended_for, description fields) so the agent knows precisely
what to edit. When a format is already configured, spec create
returns no next steps.

### Data Model

`spec create` inputs and result fields:

| Field | Type | Description |
|-------|------|-------------|
| `target` | `string` | `charter:slug` identifier |
| `--title` | `string` | Human-readable spec title |
| `--doc` | `string` | Repo-relative path to the design document |
| `--scope` | `string[]` | Repeatable; governed directories (must exist, must end with `/`) |
| `--group` | `string` | Charter group key |
| `--group-title` | `string` | Required if group is new |
| `--group-order` | `int` | Required if group is new |
| `--order` | `int` | Spec order within the group |
| `--charter-notes` | `string` | Required planning note for charter membership |
| `--tag` | `string[]` | Repeatable; optional spec-level tags |
| `result.design_doc_action` | `bootstrapped\|validated_existing\|prepended_frontmatter\|rewritten_frontmatter` | How the design document was handled |
| `result.selected_format` | `string\|null` | Format key auto-selected or `null` |
| `result.tracking_file` | `string` | Path to the created tracking file |
| `result.design_doc` | `string` | Path to the design document |

### Contracts

Success (spec create, no format configured):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "draft", "rev": 1, "checkpoint": "<HEAD_SHA>", "created": "<today>", "last_verified_at": "<today>" },
  "focus": { "spec": { "target": "runtime:session-lifecycle" } },
  "result": {
    "kind": "spec",
    "tracking_file": ".specs/runtime/session-lifecycle.yaml",
    "design_doc": "runtime/src/domain/session_execution/SPEC.md",
    "design_doc_action": "bootstrapped",
    "selected_format": null
  },
  "next": {
    "mode": "sequence",
    "steps": [{
      "action": "create_format_template",
      "kind": "ask_user_then_edit",
      "context": {
        "interview": ["What kind of system is this spec governing?", "What are the distinct behavioral surfaces?", "..."],
        "config_schema": { "location": "formats.<key>", "fields": { "template": "...", "recommended_for": "...", "description": "..." } }
      }
    }]
  }
}
```

Error (INVALID_PATH):
```json
{
  "state": { "target": "runtime:session-lifecycle", "charter_exists": true },
  "focus": {},
  "next": { "mode": "none" },
  "error": { "code": "INVALID_PATH", "message": "scope directory does not exist: runtime/src/missing/" }
}
```

Error (FORMAT_AMBIGUOUS):
```json
{
  "state": { "target": "runtime:session-e2e-context", "charter_exists": true },
  "focus": { "design_doc": { "path": "runtime/src/domain/tests/e2e/session_execution/CONTEXT.md" } },
  "next": { "mode": "none" },
  "error": { "code": "FORMAT_AMBIGUOUS", "message": "multiple configured formats match the design doc path" }
}
```

Error (PRIMARY_DOC_FRONTMATTER_INVALID):
```json
{
  "error": { "code": "PRIMARY_DOC_FRONTMATTER_INVALID", "message": "design doc frontmatter can't be parsed", "focus": { "doc": "runtime/src/domain/session_execution/SPEC.md" } }
}
```

Error (PRIMARY_DOC_FRONTMATTER_MISMATCH):
```json
{
  "error": { "code": "PRIMARY_DOC_FRONTMATTER_MISMATCH", "message": "frontmatter spec: field \"wrong-slug\" doesn't match target slug \"session-lifecycle\"", "focus": { "expected": "session-lifecycle", "found": "wrong-slug" } }
}
```

Error (SPEC_EXISTS):
```json
{
  "error": { "code": "SPEC_EXISTS", "message": "spec \"session-lifecycle\" is already registered in charter \"runtime\"", "focus": { "slug": "session-lifecycle", "charter": "runtime" } }
}
```

### Invariants

- Every `--scope` directory must exist on disk; missing directories return `INVALID_PATH`
- Frontmatter `spec` field must match the slug and `charter` field must match the charter name
- If the design doc already exists, frontmatter is validated or deterministically prepended/rewritten (`design_doc_action`)
- Format auto-selection matches `--doc` path against `formats.<key>.recommended_for` globs; ambiguous matches return `FORMAT_AMBIGUOUS`
- Format references in frontmatter must exist in config; unknown formats return `FORMAT_NOT_CONFIGURED`
- `checkpoint` is set to HEAD at creation; `last_verified_at` is set to today
- `spec create` resolves HEAD and persists it; unresolvable HEAD returns `CHECKPOINT_UNAVAILABLE`
- Scope entries must be directories (trailing `/`), never individual files
- `documents.primary` must be under one of the declared `scope[]` entries

## Requirement: Spec creation registers and validates new specs

```gherkin requirement
@specctl @write
Feature: Spec creation registers and validates new specs
```

### Scenarios

```gherkin scenario
Scenario: Spec create initializes tracking with checkpoint
  Given a charter exists and a design document is present
  When the agent runs spec create with scope and group flags
  Then the tracking file is created and checkpoint is set to HEAD
```

```gherkin scenario
Scenario: Spec create rejects invalid paths and frontmatter
  Given a scope directory does not exist or frontmatter is malformed
  When the agent attempts spec create
  Then the response returns the appropriate validation error
```

```gherkin scenario
Scenario: Spec create rejects ambiguous format matches
  Given multiple configured formats match the design doc path
  When the agent attempts spec create
  Then the response returns FORMAT_AMBIGUOUS
```

```gherkin scenario
Scenario: Spec create rejects unconfigured format references
  Given the design doc frontmatter references a format not in config
  When the agent attempts spec create
  Then the response returns FORMAT_NOT_CONFIGURED
```

```gherkin scenario
Scenario: Spec create returns format template interview when no format configured
  Given a spec is created with no format templates in config
  When the agent runs spec create
  Then next contains a create_format_template step with kind ask_user_then_edit
  And the step includes interview questions and config_schema with the formats YAML structure
```

### Journey: Create a New Spec

```text
charter create → spec create → [create_format_template] →
delta add --intent add → write_spec_section → req add →
implement_and_test → req verify → delta close → rev bump
```

Start: no tracking file exists, charter already exists (or created first).
Terminal: tracking exists, checkpoint set to HEAD, spec is `draft`.
The `create_format_template` step appears only when no format is configured.

## 15. Context Advanced Flows

Beyond basic status reporting, context produces **chooser flows** for
complex situations. When a requirement's Gherkin drifts from SPEC.md,
context offers a refresh chooser — the agent picks between refreshing
the tracked copy or opening a change delta. When a requirement is stale,
context offers a repair chooser — the agent opens a repair delta to
re-verify. When the spec itself is not found but the charter exists,
context guides spec creation. When committed drift is not yet tracked,
context marks whether the situation is a correctness blocker or a
review-first housekeeping candidate before surfacing review/sync
guidance. These chooser flows are how specctl
handles ambiguity: instead of making a decision, it presents the
options with `choose_when` conditions the agent evaluates.

### Data Model

Chooser options and drift continuation fields surfaced in `next`:

| Field | Type | Description |
|-------|------|-------------|
| `next.mode` | `choose_one` | Chooser flows always use `choose_one` |
| `next.options[].action` | `string` | One of `review_diff`, `refresh_requirement`, `delta_add_change`, `delta_add_repair`, `create_spec`, `sync` |
| `next.options[].choose_when` | `string` | Decision criterion the agent evaluates |
| `next.options[].template` | `object` | Executable `argv` or `action` |
| `focus.scope_drift.status` | `drifted\|tracked\|unavailable\|clean` | Drift state driving the chooser |
| `focus.scope_drift.drift_source` | `design_doc\|scope_code\|both` | What changed since checkpoint |
| `focus.scope_drift.review_required` | `bool` | Whether review must happen before the agent can classify the drift |
| `focus.scope_drift.correctness_blocker` | `bool` | Whether the tool already knows the situation blocks correctness-sensitive mutations |
| `focus.scope_drift.housekeeping_candidate` | `bool` | Whether re-anchoring may be sufficient if review finds no semantic change |
| `focus.scope_drift.tracked_by` | `string[]` | Delta IDs already tracking this drift |
| `focus.requirement_match_issues[]` | `array` | `{id, status, heading}` per mismatched requirement |
| `focus.lookup.reason` | `string` | `spec_not_found` when spec does not exist |

### Contracts

Refresh chooser (requirement Gherkin drifted):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": {
    "requirement_match_issues": [{ "id": "REQ-006", "status": "no_exact_match", "heading": "Requirement: Compensation stage 4 failure cleanup" }]
  },
  "next": {
    "mode": "choose_one",
    "options": [
      { "action": "refresh_requirement", "choose_when": "Same requirement identity; only match text changed", "template": { "argv": ["specctl", "req", "refresh", "runtime:session-lifecycle", "REQ-006"] } },
      { "action": "delta_add_change", "choose_when": "Existing requirement is no longer the correct statement of truth", "template": { "argv": ["specctl", "delta", "add", "runtime:session-lifecycle", "--intent", "change", "--area", "<area>"] } }
    ]
  }
}
```

Repair chooser (stale evidence):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": {
    "requirement": { "id": "REQ-008", "verification": "stale" }
  },
  "next": {
    "mode": "choose_one",
    "options": [
      { "action": "delta_add_repair", "choose_when": "Evidence is no longer trusted and needs re-verification", "template": { "argv": ["specctl", "delta", "add", "runtime:session-lifecycle", "--intent", "repair", "--area", "<area>"] } }
    ]
  }
}
```

Spec not found with charter exists:
```json
{
  "state": { "target": "runtime:unknown-spec" },
  "focus": { "lookup": { "reason": "spec_not_found", "charter_exists": true } },
  "next": {
    "mode": "choose_one",
    "options": [{ "action": "create_spec", "template": { "argv": ["specctl", "spec", "create", "runtime:<slug>", "--title", "<title>", "--doc", "<doc>", "--scope", "<scope>/"] } }]
  }
}
```

Design-doc drift with optional housekeeping re-anchor:
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "verified" },
  "focus": {
    "scope_drift": {
      "status": "drifted",
      "drift_source": "design_doc",
      "review_required": true,
      "correctness_blocker": false,
      "housekeeping_candidate": true
    }
  },
  "next": {
    "mode": "choose_one",
    "options": [
      { "action": "review_diff", "choose_when": "Review the drift before deciding whether it needs semantic tracking.", "template": { "argv": ["specctl", "diff", "runtime:session-lifecycle"] } },
      { "action": "sync", "choose_when": "Review confirms the edit is clarification-only and the checkpoint just needs re-anchoring.", "template": { "argv": ["specctl", "sync", "runtime:session-lifecycle", "--checkpoint", "HEAD"] } }
    ]
  }
}
```

Checkpoint unavailable:
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "scope_drift": { "status": "unavailable" } },
  "next": {
    "mode": "sequence",
    "steps": [{ "action": "sync", "template": { "argv": ["specctl", "sync", "runtime:session-lifecycle", "--checkpoint", "HEAD"] } }]
  }
}
```

### Invariants

- Chooser flows appear only when drift or match issues are actionable on `active` requirements
- Match issues on `superseded`/`withdrawn` requirements are informational only and never produce chooser flows
- When `scope_drift.status == "tracked"`, no drift chooser appears — context continues the active delta workflow
- When `scope_drift.status == "unavailable"`, the only option is `sync --checkpoint HEAD` (no chooser)
- Review-first drift guidance can mark committed drift as non-blocking housekeeping without deciding whether the change is semantic
- When `focus.scope_drift.review_required == true`, `review_diff` is the canonical first-class handoff to `specctl diff`; sync guidance remains conditional on what that review concludes
- Spec-not-found always offers `create_spec` when the charter exists; when the charter also does not exist, guidance includes `charter create` first

## Requirement: Context chooser flows surface review-first drift guidance

```gherkin requirement
@specctl @read
Feature: Context chooser flows surface review-first drift guidance
```

### Scenarios

```gherkin scenario
Scenario: Refresh chooser when requirement Gherkin drifts
  Given a requirement whose tracked Gherkin differs from SPEC.md
  When the agent runs specctl context
  Then next offers choose_one between refresh and change delta
```

```gherkin scenario
Scenario: Repair chooser when requirement evidence is stale
  Given a verified requirement under a scope that drifted
  When the agent runs specctl context
  Then next offers a repair delta option
```

```gherkin scenario
Scenario: Design-doc drift is marked as review-first housekeeping
  Given committed design-doc drift exists with no match issues
  When the agent runs specctl context
  Then focus.scope_drift marks the situation as review-required and non-blocking
  And next includes review_diff and sync guidance
```

```gherkin scenario
Scenario: Registry context summarizes deferred and drifted surfaces without false urgency
  Given the governed repo contains deferred deltas or drifted specs
  When the agent runs specctl context without a target
  Then focus may include advisory summaries and a recommended review target
  And next remains none when no mandatory action exists
```

```gherkin scenario
Scenario: Charter context summarizes deferred and drifted surfaces without false urgency
  Given one charter contains deferred deltas or drifted specs
  When the agent runs specctl context for that charter
  Then focus may include advisory summaries and a recommended review target
  And next remains none when no mandatory action exists
```

```gherkin scenario
Scenario: Residue cleanup does not replace review-first drift triage
  Given committed drift exists and superseded orphan cleanup residue is still visible
  When the agent runs specctl context
  Then focus keeps scope_drift as the primary triage surface
  And next still begins with review_diff instead of cleanup guidance
```

```gherkin scenario
Scenario: Scope-code drift still hands off through review_diff first
  Given committed scope-code drift exists with no tracked delta
  When the agent runs specctl context
  Then focus.scope_drift marks the situation as review-required
  And next includes review_diff before any sync cleanup guidance
```

```gherkin scenario
Scenario: Spec not found with charter guides creation
  Given a charter exists but the spec slug is not registered
  When the agent runs specctl context
  Then the response guides spec creation
```

```gherkin scenario
Scenario: Charter and registry resolution for context
  Given a valid charter with registered specs
  When the agent runs specctl context at charter or registry level
  Then the response shows charter membership and spec health
```

```gherkin scenario
Scenario: Checkpoint unavailable degrades gracefully
  Given the stored checkpoint commit no longer exists
  When the agent runs specctl context
  Then drift detection is unavailable but the spec remains accessible
```

## 16. Delta Add Validation

`delta add` validates its inputs strictly before creating the delta.
The intent must be one of add/change/remove/repair. For change, remove,
and repair intents, `affects_requirements` must list active requirement
IDs — the delta must declare which existing behaviors it touches. If
the referenced requirements are not active (superseded, withdrawn), the
delta is rejected. These validations prevent orphaned deltas that
can't be closed because they reference invalid state.

### Data Model

Delta add input validation fields:

| Field | Type | Description |
|-------|------|-------------|
| `--intent` | `add\|change\|remove\|repair` | Required flag; determines the delta's workflow branch |
| `--area` | `string` | Required flag; human-readable description of the change area |
| `stdin.affects_requirements` | `string[]` | Required for `change`/`remove`/`repair`; each must be an active REQ ID |
| `stdin.current` | `string` | Description of current state |
| `stdin.target` | `string` | Description of target state |
| `stdin.notes` | `string` | Context for the change |
| `result.delta.updates` | `string[]` | Derived from intent: `add_requirement`, `replace_requirement`, `withdraw_requirement`, or `stale_requirement` |

Intent-to-update mapping:

| Intent | Required `affects_requirements` | Derived `updates` |
|--------|--------------------------------|-------------------|
| `add` | omitted or empty | `[add_requirement]` |
| `change` | required, all must be `active` | `[replace_requirement]` |
| `remove` | required, all must be `active` | `[withdraw_requirement]` |
| `repair` | required, all must be `active` | `[stale_requirement]` |

### Contracts

Error (missing intent):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "delta_add": { "reason": "invalid_delta_input" } },
  "next": { "mode": "none" },
  "error": { "code": "INVALID_INPUT", "message": "required flag --intent not provided" }
}
```

Error (change without affects_requirements):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "delta_add": { "reason": "affected_requirements_required" } },
  "next": { "mode": "none" },
  "error": { "code": "INVALID_INPUT", "message": "intent change requires affects_requirements listing active requirement IDs" }
}
```

Error (non-active affected requirement):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "delta_add": { "reason": "invalid_requirement_state", "requirement": { "id": "REQ-003", "lifecycle": "superseded" } } },
  "next": { "mode": "none" },
  "error": { "code": "INVALID_INPUT", "message": "affects_requirements references REQ-003 which is superseded, not active" }
}
```

Success (remove intent):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "ready" },
  "focus": { "delta": { "id": "D-010", "intent": "remove", "status": "open" } },
  "result": {
    "delta": { "id": "D-010", "intent": "remove", "status": "open", "updates": ["withdraw_requirement"], "affects_requirements": ["REQ-005"] },
    "allocation": { "id": "D-010" }
  },
  "next": {
    "mode": "sequence",
    "steps": [{ "action": "withdraw_requirement", "template": { "argv": ["specctl", "req", "withdraw", "runtime:session-lifecycle", "REQ-005", "--delta", "D-010"] } }]
  }
}
```

### Invariants

- `--intent` is mandatory; omission returns `INVALID_INPUT` with reason `invalid_delta_input`
- `change`, `remove`, and `repair` require `affects_requirements` with at least one entry
- Every requirement in `affects_requirements` must exist and have `lifecycle: active`; `superseded` or `withdrawn` references return `invalid_requirement_state`
- `add` intent must not include `affects_requirements` (omitted or empty)
- `delta add` does not check SPEC.md for requirement blocks — that validation defers to `req add`/`req replace`

## Requirement: Delta add validates intent and affected requirements

```gherkin requirement
@specctl @write
Feature: Delta add validates intent and affected requirements
```

### Scenarios

```gherkin scenario
Scenario: Delta add rejects missing intent
  Given no intent flag is provided
  When the agent attempts delta add
  Then the response returns INVALID_INPUT with missing intent reason
```

```gherkin scenario
Scenario: Delta add rejects change without affects_requirements
  Given intent is change but no affected requirements are listed
  When the agent attempts delta add
  Then the response returns INVALID_INPUT with affected requirements guidance
```

```gherkin scenario
Scenario: Delta add rejects non-active affected requirements
  Given the affected requirement is superseded or withdrawn
  When the agent attempts delta add with intent change
  Then the response returns INVALID_INPUT with invalid requirement state
```

```gherkin scenario
Scenario: Delta add succeeds for remove and repair intents
  Given valid affected requirements and appropriate intent
  When the agent adds a delta with intent remove or repair
  Then the delta is created with the correct updates list
```

## 17. Delta Close Enforcement

Delta close is the integrity gate. It verifies that all the delta's
obligations are met before sealing it. Unverified requirements block
closure — every requirement introduced by the delta must be verified.
Unresolved updates block closure — the delta's declared `updates` must
be completed. Match issues block closure — no requirement can have a
Gherkin mismatch with SPEC.md. The close response tells the agent
exactly what's blocking and what to do about it. After successful
closure, the next action is `rev_bump`.

### Data Model

Delta closure check fields in the error focus:

| Field | Type | Description |
|-------|------|-------------|
| `focus.delta_close.reason` | `string` | `requirements_unverified`, `updates_unresolved`, or `match_blocking` |
| `focus.delta_close.blocking_requirements` | `string[]` | REQ IDs that are unverified or stale |
| `focus.delta_close.unresolved_updates` | `string[]` | Update types not yet performed (e.g. `add_requirement`) |
| `focus.delta_close.match_issues` | `array` | `{id, status}` per requirement with a blocking match problem |
| `result.delta.status` | `"closed"` | On success, the delta is sealed |

Repair delta terminal behavior: when a `repair` delta closes, the spec may transition to `verified` if all active requirements are verified and no match issues remain. The `next` then suggests `sync` (not `rev bump`) because repair deltas do not introduce new semantic content.

### Contracts

Error (unverified requirements):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "delta_close": { "reason": "requirements_unverified", "blocking_requirements": ["REQ-007"] } },
  "next": {
    "mode": "sequence",
    "steps": [
      { "action": "implement_and_test", "template": { "action": "edit_file" }, "context": { "requirement": "REQ-007" } },
      { "action": "verify_requirement", "template": { "argv": ["specctl", "req", "verify", "runtime:session-lifecycle", "REQ-007", "--test-file", "<test_path>"] } }
    ]
  },
  "error": { "code": "UNVERIFIED_REQUIREMENTS", "message": "Delta D-009 has unverified active requirements: REQ-007" }
}
```

Error (unresolved updates):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "ready" },
  "focus": { "delta_close": { "reason": "updates_unresolved", "unresolved_updates": ["add_requirement"] } },
  "next": {
    "mode": "sequence",
    "steps": [{ "action": "add_requirement", "template": { "argv": ["specctl", "req", "add", "runtime:session-lifecycle", "--delta", "D-009"] } }]
  },
  "error": { "code": "DELTA_UPDATES_UNRESOLVED", "message": "Delta D-009 has unresolved updates: add_requirement" }
}
```

Success (repair delta terminal):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "verified" },
  "focus": { "delta": { "id": "D-010", "intent": "repair", "status": "closed" } },
  "result": { "delta": { "id": "D-010", "status": "closed" } },
  "next": {
    "mode": "sequence",
    "steps": [{ "action": "sync", "template": { "argv": ["specctl", "sync", "runtime:session-lifecycle", "--checkpoint", "HEAD"] } }]
  }
}
```

### Invariants

- Close requires an empty unverified set: every active requirement touched by the delta must be `verification: verified`
- Close requires all declared `updates[]` to have been performed (e.g., `add_requirement` satisfied by a `req add` on that delta)
- Close requires no blocking match issues on active requirements touched by the delta
- Repair delta terminal: after close, `next` suggests `sync` instead of `rev bump` because no new semantic content was introduced
- If the delta has no registered tracked update work at all, close is rejected with `DELTA_UPDATES_UNRESOLVED`

## Requirement: Delta close enforces all obligations before sealing

```gherkin requirement
@specctl @lifecycle
Feature: Delta close enforces all obligations before sealing
```

### Scenarios

```gherkin scenario
Scenario: Delta close rejects when updates are unresolved
  Given a delta with pending add_requirement updates
  When the agent attempts delta close
  Then the response returns with unresolved updates guidance
```

```gherkin scenario
Scenario: Delta close rejects on match blocking issues
  Given a delta's requirement has a Gherkin mismatch with SPEC.md
  When the agent attempts delta close
  Then the response returns REQUIREMENT_MATCH_BLOCKING
```

```gherkin scenario
Scenario: Delta close rejects invalid state transitions
  Given a delta that is not in open or active status
  When the agent attempts delta close
  Then the response returns DELTA_INVALID_STATE
```

```gherkin scenario
Scenario: Delta close with repair intent reaches terminal state
  Given a repair delta with all affected requirements re-verified
  When the agent closes the delta
  Then the delta reaches repair terminal and next suggests sync
```

## 18. Delta Transition Error Handling

Delta transitions (start, defer, resume) enforce the FSM strictly.
Each verb has preconditions: start requires open status, defer requires
active status, resume requires deferred status. When the delta or spec
doesn't exist, the response provides the exact error code and the
invalid state. When validation fails (the tracking file is corrupt),
transitions are blocked with repair guidance. These error paths ensure
the agent always knows why a transition failed and what to do.

### Data Model

Valid delta state transitions:

| Command | From | To | Description |
|---------|------|----|-------------|
| `delta start` | `open` | `in-progress` | Begin active work on the delta |
| `delta defer` | `open` | `deferred` | Pause before starting |
| `delta defer` | `in-progress` | `deferred` | Pause active work |
| `delta resume` | `deferred` | `open` | Resume paused delta |

Error focus fields for transition failures:

| Field | Type | Description |
|-------|------|-------------|
| `focus.delta_transition.reason` | `string` | `delta_not_found`, `invalid_state`, or `spec_not_found` |
| `focus.delta_transition.current_status` | `string` | The delta's actual status when the transition was attempted |
| `focus.delta_transition.requested` | `string` | The transition verb (`start`, `defer`, `resume`) |

### Contracts

Error (DELTA_NOT_FOUND):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "delta_transition": { "reason": "delta_not_found" } },
  "next": { "mode": "none" },
  "error": { "code": "DELTA_NOT_FOUND", "message": "Delta D-999 does not exist in runtime:session-lifecycle" }
}
```

Error (DELTA_INVALID_STATE):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "delta_transition": { "reason": "invalid_state", "current_status": "closed", "requested": "start" } },
  "next": { "mode": "none" },
  "error": { "code": "DELTA_INVALID_STATE", "message": "Cannot start delta D-007: current status is closed" }
}
```

Success (delta start):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "delta": { "id": "D-009", "status": "in-progress" } },
  "result": { "delta": { "id": "D-009", "status": "in-progress" } },
  "next": { "mode": "none" }
}
```

Error (SPEC_NOT_FOUND):
```json
{
  "state": { "target": "runtime:nonexistent" },
  "focus": { "lookup": { "reason": "spec_not_found" } },
  "next": { "mode": "none" },
  "error": { "code": "SPEC_NOT_FOUND", "message": "Spec runtime:nonexistent does not exist" }
}
```

### Invariants

- Closed deltas cannot transition; any verb on a closed delta returns `DELTA_INVALID_STATE`
- Deferred deltas can only `resume` (back to `open`); `start` on a deferred delta is invalid
- `start` is only valid from `open`; `defer` is valid from `open` or `in-progress`
- Delta metadata (intent, area, affects_requirements) is immutable across transitions; only `status` changes
- Validation errors on the tracking file block all transitions with `VALIDATION_FAILED`

## Requirement: Delta transitions reject invalid states and missing targets

```gherkin requirement
@specctl @lifecycle
Feature: Delta transitions reject invalid states and missing targets
```

### Scenarios

```gherkin scenario
Scenario: Delta transition fails when delta not found
  Given a delta ID that does not exist in the tracking file
  When the agent attempts start, defer, or resume
  Then the response returns DELTA_NOT_FOUND
```

```gherkin scenario
Scenario: Delta transition fails on invalid state
  Given a delta in a state incompatible with the requested transition
  When the agent attempts the transition
  Then the response returns DELTA_INVALID_STATE with current status
```

```gherkin scenario
Scenario: Delta transition fails when spec not found
  Given a spec target that does not exist
  When the agent attempts any delta transition
  Then the response returns SPEC_NOT_FOUND
```

```gherkin scenario
Scenario: Delta transition fails on validation errors
  Given a tracking file with validation findings
  When the agent attempts any delta transition
  Then the response returns VALIDATION_FAILED with repair guidance
```

## 19. Write Validation Common Surface

All write commands share a validation gate: before any mutation, specctl
validates the tracking file's current state. If the tracking file is
invalid (missing required fields, inconsistent status, corrupt YAML),
the write is rejected with `VALIDATION_FAILED` and the specific
findings. This common surface ensures that a broken tracking file
blocks ALL mutations equally — the agent must repair the file before
any specctl write command will proceed.

### Data Model

Validation findings returned on failure:

| Field | Type | Description |
|-------|------|-------------|
| `error.code` | `"VALIDATION_FAILED"` | Uniform error code for all write commands |
| `state.validation.valid` | `false` | Always `false` when findings exist |
| `state.validation.findings[]` | `array` | Each finding: `{code, severity, message, path?}` |
| `findings[].code` | `string` | Machine-readable finding code (e.g. `MISSING_REQUIRED_FIELD`, `INCONSISTENT_STATUS`, `INVALID_YAML`) |
| `findings[].severity` | `error\|warning` | `error` findings block writes; `warning` findings are informational |
| `findings[].message` | `string` | Human-readable explanation |
| `findings[].path` | `string?` | YAML path to the offending field (e.g. `requirements[2].lifecycle`) |

### Contracts

Error (VALIDATION_FAILED):
```json
{
  "state": {
    "slug": "session-lifecycle",
    "charter": "runtime",
    "validation": {
      "valid": false,
      "findings": [
        { "code": "MISSING_REQUIRED_FIELD", "severity": "error", "message": "requirements[2].gherkin is required", "path": "requirements[2].gherkin" },
        { "code": "INCONSISTENT_STATUS", "severity": "error", "message": "status is verified but REQ-003 is unverified" }
      ]
    }
  },
  "focus": {},
  "next": { "mode": "none" },
  "error": { "code": "VALIDATION_FAILED", "message": "Tracking file has 2 validation errors" }
}
```

### Invariants

- Validation runs post-mutation but pre-persist: if the resulting state is invalid, nothing is written to disk
- All write commands share the same validation gate; no write command bypasses it
- `error` severity findings block the write; `warning` severity findings are surfaced but do not block
- The validation surface covers YAML integrity, field constraints, ID sequencing, cross-reference consistency, and status derivation correctness
- Validation failure returns the full findings array so the agent can address all issues in one pass

### Status Invariant Table

| Status | Required invariants |
|--------|---------------------|
| `draft` | frontmatter valid, primary doc exists, `documents.primary` under `scope[]`, scope non-empty, IDs sequential |
| `ready` | `draft` + at least one live delta with unresolved tracked updates |
| `active` | `ready` + every required tracked update exists; verified test files exist unless `@manual` |
| `verified` | `active` + every live delta closed, every active req verified, no match issues |

## Requirement: Write commands reject invalid tracking state uniformly

```gherkin requirement
@specctl @write
Feature: Write commands reject invalid tracking state uniformly
```

### Scenarios

```gherkin scenario
Scenario: Write validation fails for req refresh on invalid state
  Given a tracking file with validation errors
  When the agent attempts req refresh
  Then the response returns VALIDATION_FAILED with findings
```

```gherkin scenario
Scenario: Write validation fails for req verify on invalid state
  Given a tracking file with validation errors
  When the agent attempts req verify
  Then the response returns VALIDATION_FAILED with findings
```

```gherkin scenario
Scenario: Write validation fails for rev bump and sync on invalid state
  Given a tracking file with validation errors
  When the agent attempts rev bump or sync
  Then the response returns VALIDATION_FAILED with findings
```

## 20. Rev Bump and Sync Preconditions

Rev bump and sync have specific preconditions beyond basic validation.
Rev bump requires the spec to be in verified status and closed deltas
not yet in the changelog — without these, there's nothing to bump.
It also requires `--checkpoint` and a summary. Sync requires no live
semantic deltas (you can't sync past active work). It also requires the
agent to have already reviewed the drift and concluded that re-anchoring
is housekeeping rather than new semantic work. Both commands reject when
the checkpoint is unavailable or match issues exist.

### Data Model

Rev bump inputs and preconditions:

| Field | Type | Description |
|-------|------|-------------|
| `--checkpoint` | `string` | Required; git SHA to anchor the new revision |
| `stdin` (summary) | `string` | Required; changelog narrative read from stdin |
| `state.status` | `"verified"` | Precondition: must be `verified` |
| `result.previous_rev` | `int` | Rev before bump |
| `result.rev` | `int` | `previous_rev + 1` |
| `result.changelog_entry` | `object` | `{rev, date, deltas_closed[], reqs_verified[], summary}` |

Sync inputs and preconditions:

| Field | Type | Description |
|-------|------|-------------|
| `--checkpoint` | `string` | Required; git SHA to re-anchor |
| `stdin` (summary) | `string` | Required; reason for sync |
| `state.deltas` (live) | `0` | Precondition: no open or in-progress deltas |
| `result.previous_checkpoint` | `string` | Checkpoint before sync |
| `result.checkpoint` | `string` | New checkpoint |

Error focus reasons:

| Reason | Command | Meaning |
|--------|---------|---------|
| `status_not_verified` | `rev bump` | Spec status is not `verified` |
| `no_semantic_changes` | `rev bump` | All closed deltas already recorded in changelog |
| `live_deltas_present` | `sync` | Open or in-progress deltas exist |
| `match_blocking` | both | Active requirements have unresolved match issues |

### Contracts

Error (status_not_verified for rev bump):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": { "rev_bump": { "reason": "status_not_verified" } },
  "next": { "mode": "none" },
  "error": { "code": "INVALID_INPUT", "message": "Rev bump requires verified status; current status is active" }
}
```

Error (no_semantic_changes for rev bump):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "verified" },
  "focus": { "rev_bump": { "reason": "no_semantic_changes" } },
  "next": { "mode": "none" },
  "error": { "code": "INVALID_INPUT", "message": "No closed deltas since last changelog entry; nothing to bump" }
}
```

Error (live_deltas_present for sync):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active", "deltas": { "open": 1, "in_progress": 0 } },
  "focus": { "sync": { "reason": "live_deltas_present", "live_delta_ids": ["D-009"] } },
  "next": { "mode": "none" },
  "error": { "code": "INVALID_INPUT", "message": "Sync is illegal while live deltas exist: D-009" }
}
```

Error (match_blocking for rev bump):
```json
{
  "state": { "slug": "session-lifecycle", "charter": "runtime", "status": "active" },
  "focus": {
    "rev_bump": { "reason": "match_blocking" },
    "requirement_match_issues": [{ "id": "REQ-006", "status": "no_exact_match" }]
  },
  "next": {
    "mode": "choose_one",
    "options": [
      { "action": "refresh_requirement", "choose_when": "Same requirement identity; only match text changes" },
      { "action": "delta_add_change", "choose_when": "Existing requirement is no longer current truth" }
    ]
  },
  "error": { "code": "REQUIREMENT_MATCH_BLOCKING", "message": "Rev bump blocked by match issues on active requirements" }
}
```

### Invariants

- Rev bump requires `status == verified` and at least one closed delta not yet recorded in the changelog
- Rev bump requires both `--checkpoint` and a summary via stdin; omitting either returns `INVALID_INPUT`
- Rev bump is blocked by any active requirement match issue (`REQUIREMENT_MATCH_BLOCKING`)
- Sync requires zero live deltas (open or in-progress); deferred deltas do not block sync
- Sync does not infer semantics from drift source; it relies on prior review and is legal only when the agent concludes the change is housekeeping-only
- Sync requires `--checkpoint` and a summary via stdin
- Both commands reject unresolvable checkpoint SHAs with `CHECKPOINT_UNAVAILABLE`

## Requirement: Rev bump and sync enforce reviewed re-anchor preconditions

```gherkin requirement
@specctl @lifecycle
Feature: Rev bump and sync enforce reviewed re-anchor preconditions
```

### Scenarios

```gherkin scenario
Scenario: Rev bump rejects non-verified spec status
  Given a spec that is not in verified status
  When the agent attempts rev bump
  Then the response returns status_not_verified
```

```gherkin scenario
Scenario: Rev bump rejects when no closed deltas need recording
  Given all closed deltas are already in the changelog
  When the agent attempts rev bump
  Then the response returns no_semantic_changes
```

```gherkin scenario
Scenario: Rev bump rejects missing checkpoint or summary
  Given the checkpoint or summary flag is omitted
  When the agent attempts rev bump
  Then the response returns INVALID_INPUT with the missing field
```

```gherkin scenario
Scenario: Sync rejects when live deltas are present
  Given open or active deltas exist
  When the agent attempts sync
  Then the response returns live_deltas_present
```

```gherkin scenario
Scenario: Sync accepts reviewed housekeeping drift
  Given committed drift was reviewed and determined not to change governed behavior
  When the agent attempts sync
  Then the checkpoint is re-anchored without a revision bump
```

```gherkin scenario
Scenario: Sync rejects missing checkpoint or summary
  Given the checkpoint or summary is omitted
  When the agent attempts sync
  Then the response returns INVALID_INPUT with the missing field
```

```gherkin scenario
Scenario: Rev bump and sync reject on match blocking
  Given active requirements have Gherkin mismatches
  When the agent attempts rev bump or sync
  Then the response returns REQUIREMENT_MATCH_BLOCKING
```

## 21. Duplicate Requirement Guard

`req add` must reject Gherkin whose Feature title already belongs to a
tracked, non-withdrawn requirement in the same spec. Without this guard,
two REQ IDs can point at the same `## Requirement:` heading in SPEC.md,
corrupting the tracking state. This was discovered during dogfooding:
REQ-015 and REQ-021 both tracked "Context produces chooser flows for
complex situations" simultaneously.

### Data Model

The guard checks `tracking.Requirements[*].Gherkin` against the incoming
Gherkin block. The comparison key is the Feature title derived via
`domain.DeriveRequirementTitle`. Requirements with `lifecycle: withdrawn`
are excluded — withdrawn behaviors can be re-introduced via `req add`.

### Contracts

Error (REQUIREMENT_ALREADY_TRACKED):
```json
{
  "state": { "status": "ready", "..." : "..." },
  "focus": {},
  "next": { "mode": "none" },
  "error": {
    "code": "REQUIREMENT_ALREADY_TRACKED",
    "message": "Feature \"<title>\" is already tracked as <REQ-ID> (<lifecycle>)"
  }
}
```

The `focus` includes `existing_requirement` with `id`, `lifecycle`, and
`verification` fields so the agent can decide whether to use `req replace`
or `req refresh` instead.

### Invariants

- Feature titles are unique among non-withdrawn requirements within a spec
- `req replace` is exempt: the replacement is expected to share the title
  with the requirement being superseded
- Withdrawn requirements do not block re-introduction via `req add`

## Requirement: Duplicate requirement guard rejects already-tracked Feature titles

```gherkin requirement
@specctl @write
Feature: Duplicate requirement guard rejects already-tracked Feature titles
```

### Scenarios

```gherkin scenario
Scenario: req add rejects when an active requirement has the same Feature title
  Given REQ-001 tracks Feature "Compensation stage 4 failure cleanup" with lifecycle active
  When the agent runs req add with the same Feature title on a different delta
  Then the response returns REQUIREMENT_ALREADY_TRACKED with existing_requirement.id REQ-001
```

```gherkin scenario
Scenario: req add succeeds when the matching requirement is withdrawn
  Given REQ-001 tracks Feature "Compensation stage 4 failure cleanup" with lifecycle withdrawn
  When the agent runs req add with the same Feature title
  Then a new requirement is created successfully
```

## 22. Drift Detection Surface

Drift detection compares the current working tree against the stored
checkpoint to classify what changed. It is currently exercised through
Context (section 1) and Diff (section 2), but the detection logic has its
own state machine with four states and specific transition rules that
warrant a dedicated surface description.

Four drift states: **clean** (nothing changed), **drifted** (uncommitted
or committed changes not yet reviewed), **tracked** (changes covered by
an open delta), **unavailable** (checkpoint cannot be resolved).
Transitions are driven by commits, delta operations, and sync/rev-bump.

### Data Model

| Field | Type | Description |
|-------|------|-------------|
| `scope_drift.status` | `clean \| drifted \| tracked \| unavailable` | Current drift state |
| `scope_drift.drift_source` | `design_doc \| scope_code \| both` | What changed; only present when `drifted` |
| `scope_drift.tracked_by` | `string[]` | Delta IDs covering the drift; only present when `tracked` |
| `scope_drift.checkpoint` | `string` | The baseline SHA |
| `uncommitted_changes` | `string[]` | File paths with unstaged edits under `scope[]` |

### Contracts

Clean:
```json
{ "scope_drift": { "status": "clean", "checkpoint": "<SHA>" } }
```

Drifted:
```json
{ "scope_drift": { "status": "drifted", "drift_source": "scope_code", "files_changed_since_checkpoint": ["..."] } }
```

Tracked:
```json
{ "scope_drift": { "status": "tracked", "tracked_by": ["D-001"] } }
```

Unavailable:
```json
{ "scope_drift": { "status": "unavailable" } }
```

### State Machine

```text
clean -- committed changes touching governed files --> drifted
drifted -- delta add covers the drift source --> tracked
tracked -- delta close + rev bump --> clean
tracked -- new uncovered committed changes --> drifted
any state -- top-level checkpoint unresolvable --> unavailable
unavailable -- sync --checkpoint HEAD --> clean
```

### Invariants

- `clean` -> `drifted` on any commit touching governed files
- `drifted` -> `tracked` when `delta add` covers the drift source
- `tracked` -> `clean` after delta close + rev bump
- `unavailable` -> `clean` after `sync --checkpoint HEAD`
- Drift classification uses semantic diff (ignores bookkeeping fields: `status`, `rev`, `updated`, `last_verified_at`, `checkpoint`, `changelog`)
- Non-deferred deltas contribute coverage; deferred deltas do not
- Closed deltas still count as coverage until the next sync point
- A delta with an unresolvable `origin_checkpoint` contributes no coverage
- Top-level checkpoint failure makes the whole spec `unavailable`
- `uncommitted_changes[]` does not change `scope_drift.status` by itself; context prepends staging/commit actions before specctl mutations
- `last_verified_at` is informational only — it records when the spec was last verified but is NOT used in drift computation. Drift is checkpoint-based (`checkpoint..HEAD`), never date-based.

### State Transitions

| From | To | Trigger |
|------|----|---------|
| `clean` | `drifted` | commit touching governed files |
| `drifted` | `tracked` | `delta add` covers the drift source |
| `drifted` | `clean` | `sync` (reviewed housekeeping drift) or `rev bump` |
| `tracked` | `clean` | `rev bump` after `delta close` |
| `tracked` | `drifted` | `delta close` without `rev bump` |
| any | `unavailable` | checkpoint SHA no longer resolvable |
| `unavailable` | `clean` | `sync --checkpoint HEAD` |

## 24. Cross-Boundary Specs

Cross-boundary work is represented by normal specs with broader scope.
There is no special cross-boundary system type. A spec may declare scope
entries spanning multiple directories or subsystems — the same lifecycle,
validation, and drift rules apply.

## 23. Write Atomicity and Concurrency

v2 is a single-writer engine at the spec level. One successful mutating command at a time per target spec, charter, or config file. specctl performs atomic replace writes -- no partial state on disk. There are no lock-management, `--actor`, or `--force` flags. Concurrent external edits to the same managed file are out of contract. Read commands reflect the last fully persisted state on disk.

**Invariants:**

- Exactly one mutating command succeeds at a time per target file
- Writes are atomic replace: the file is either fully updated or untouched
- No lock files, advisory locks, or actor-tracking mechanisms exist
- External edits to specctl-managed YAML between commands produce undefined results
- Read commands are always safe and reflect the last complete write
- Multi-file writes follow deterministic order: config, charter, tracking, bootstrapped primary doc
- specctl provides no commands to delete deltas or requirements. Closed deltas and withdrawn/superseded requirements remain in the tracking file as historical records. The only removal command is `charter remove-spec`, which deregisters a spec from its charter without deleting the tracking file.

## 25. Example Subcommand

`specctl example` returns the tool's own governed specification embedded
at compile time via `go:embed`. The agent receives a complete, real-world
example of a fully governed spec without reading external files or having
any spec initialized in the current repo. This is the ouroboros made
concrete: the specification that defines specctl is packed into the binary
that implements it.

### Data Model

The example embeds five files at compile time:

| Field | Source file | Role |
|-------|------------|------|
| `design_document` | `SPEC.md` | The behavioral specification (five-layer format) |
| `format_template` | `SPEC_FORMAT.md` | Section structure guide with guidance comments |
| `config` | `.specs/specctl.yaml` | Project configuration (tags, prefixes, formats) |
| `charter` | `.specs/specctl/CHARTER.yaml` | Spec registry with groups and ordering |
| `tracking` | `.specs/specctl/cli.yaml` | Lifecycle state: deltas, requirements, changelog |

### Contracts

Success:
```json
{
  "state": { "kind": "example", "version": "specctl:cli rev <N>" },
  "focus": {
    "description": "Complete governed specification example",
    "files": [
      { "path": "SPEC.md", "role": "design_document", "lines": 3157 },
      { "path": "SPEC_FORMAT.md", "role": "format_template", "lines": 111 },
      { "path": ".specs/specctl.yaml", "role": "config", "lines": 22 },
      { "path": ".specs/specctl/CHARTER.yaml", "role": "charter", "lines": 13 },
      { "path": ".specs/specctl/cli.yaml", "role": "tracking", "lines": 534 }
    ]
  },
  "result": {
    "design_document": "<SPEC.md content>",
    "format_template": "<SPEC_FORMAT.md content>",
    "config": "<specctl.yaml content>",
    "charter": "<CHARTER.yaml content>",
    "tracking": "<cli.yaml content>"
  },
  "next": { "mode": "none" }
}
```

This command has no error path — the example is compiled into the binary.

### Invariants

- No flags, no stdin, no filesystem access required
- The embedded content is frozen at build time — it reflects the spec
  version that was compiled, not the current repo state
- The standard envelope shape (`state`, `focus`, `result`, `next`) is
  preserved for consistency with all other specctl commands
- `focus.files[].lines` is computed from the embedded content

## Requirement: Example subcommand returns the embedded governed spec

```gherkin requirement
@specctl @read
Feature: Example subcommand returns the embedded governed spec
```

### Scenarios

```gherkin scenario
Scenario: specctl example returns all five embedded files
  Given specctl is installed with embedded spec content
  When the agent runs specctl example
  Then the response contains design_document, format_template, config, charter, and tracking in result
  And focus.files lists metadata for each embedded file
```

```gherkin scenario
Scenario: specctl example requires no flags or filesystem state
  Given no .specs/ directory exists in the current repo
  When the agent runs specctl example
  Then the response succeeds with the embedded spec content
```

---

## 26. Delta Add Eager Validation of Repair Intent

`delta add --intent repair` is valid only when the requirements named in
`affects_requirements` can legally be transitioned to `stale`. The
closed-delta invariant forbids staling a requirement that a closed
delta depends on being `verified`, so a repair delta whose only update
path is `req stale` against such a requirement cannot ever be closed.
Before this surface, specctl detected the conflict only at `req stale`
time, after a D-id had been allocated; the caller had to `defer` the
allocated delta and re-open with `--intent change`, burning the D-id
and leaving permanent residue in `state.deltas.deferred`. This surface
moves the check to `delta add` and fails fast.

### Invariants

- A `delta add` with `--intent repair` must reject with
  `VALIDATION_FAILED` before allocating a D-id when any entry in
  `affects_requirements` is referenced by a closed delta requiring it
  `verified`.
- The rejection payload enumerates the conflicting closed deltas and
  suggests `--intent change` as the structural alternative.
- The check applies only to `--intent repair`; other intents retain
  their current validation surface.

## Requirement: Delta add pre-flights repair intent against closed-delta invariants

```gherkin requirement
@specctl @lifecycle
Feature: Delta add pre-flights repair intent against closed-delta invariants
```

---

## 27. Delta Withdraw Lifecycle Transition

`delta withdraw` is a first-class retraction verb for deltas opened in
error. Without it, the only workaround is to `defer` the delta
permanently, which (a) burns a D-id, (b) leaves a `deferred` entry in
the tracking YAML forever, and (c) earns `DEFERRED_SUPERSEDED_RESIDUE`
warnings once the delta's affected requirements are superseded
downstream, because the tool assumes a `deferred` delta still matters.
Withdraw expresses the opposite signal: this delta will never happen
and the tool should stop treating it as live work.

### Invariants

- `delta withdraw` transitions `open | in-progress | deferred` →
  `withdrawn`; it is rejected on `closed` deltas (that concept is a
  compensating / revert delta, not a withdraw).
- Withdrawn deltas persist in the tracking YAML with `withdrawn_reason`
  recorded and appear under `deltas_withdrawn` in the rev changelog
  entry for the rev in which the transition happens.
- Withdrawn deltas do not contribute to `DEFERRED_SUPERSEDED_RESIDUE`
  and cannot be resumed; a change of mind requires a fresh `delta add`.

## Requirement: Delta withdraw retracts a non-closed delta with auditable reason

```gherkin requirement
@specctl @lifecycle
Feature: Delta withdraw retracts a non-closed delta with auditable reason
```

---

## 28. Requirement Rebind Across Supersession

A delta's `affects_requirements` list freezes at `delta add` time.
When a referenced requirement is later superseded through `req
replace`, open and deferred deltas remain anchored to dead requirement
IDs. Today specctl correctly raises `DEFERRED_SUPERSEDED_RESIDUE`
for this case but offers no resolution path — governance forbids
hand-editing the tracking YAML, and the only escape is to withdraw
(or defer-forever) the entire delta, losing information the team may
still want to track. This surface introduces two composable paths to
keep anchors live.

### Invariants

- Automatic rebind: when `req replace REQ-X --delta D-new` runs with a
  scope-preserving replacement, every `open | in-progress | deferred`
  delta whose `affects_requirements` contains `REQ-X` has that entry
  updated to the replacement ID. Gated by a new config key
  `auto_rebind_on_replace` (default `false` for existing installs,
  `true` for `specctl init`). Each rebind emits `AUTO_REBIND_APPLIED`
  in context warnings and is recorded in the rev changelog.
- Explicit rebind: `specctl delta rebind-requirements <charter:slug>
  <D-id> --from REQ-X --to REQ-Y` (or `--remove REQ-X --reason
  <text>`) updates `affects_requirements` for `open | in-progress |
  deferred` deltas. It does not change the delta's `status` or
  `intent`, and it does not change any requirement's lifecycle or
  verification.
- Closed-delta invariant: `affects_requirements` of `closed` deltas is
  immutable. Neither the automatic nor the explicit path may modify
  it.

## Requirement: Requirement rebind keeps open-delta anchors live across supersession

```gherkin requirement
@specctl @write
Feature: Requirement rebind keeps open-delta anchors live across supersession
```

---

## Appendix A: YAML Schemas

Reference schemas for the three specctl-managed stores. Sourced from SPEC.md section 3.

### A.1 Tracking File Schema

```yaml
# --- Identity ---
slug: session-lifecycle              # string, matches ^[a-z0-9][a-z0-9-]*$
charter: runtime                     # string, matches ^[a-z0-9][a-z0-9-]*$
title: Session Lifecycle             # string, required, non-empty
status: active                       # draft | ready | active | verified (auto-managed)
rev: 9                               # int >= 1 (auto-managed)
created: 2026-03-05                  # YYYY-MM-DD
updated: 2026-03-31                  # YYYY-MM-DD (auto-managed)
last_verified_at: 2026-03-31         # YYYY-MM-DD (auto-managed)
checkpoint: a1b2c3f                  # git SHA (auto-managed)

# --- Classification ---
tags:                                # ordered unique set, each ^[a-z0-9][a-z0-9-]*$
  - runtime
  - domain

# --- Documents ---
documents:
  primary: runtime/src/domain/session_execution/SPEC.md
  # normalized repo-relative markdown path
  # must be under at least one declared scope[]

# --- Governed scope ---
scope:                               # each entry: repo-relative dir ending in /
  - runtime/src/domain/session_execution/
  - runtime/src/adapters/outbound/lifecycle/

# --- Deltas ---
deltas:
  - id: D-008                        # exactly D-001, D-002, ... no gaps
    area: Compensation stage 4 cleanup  # string
    intent: change                   # add | change | remove | repair
    status: open                     # open | in-progress | closed | deferred
    origin_checkpoint: a1b2c3f       # git SHA (auto-managed)
    current: Stage 4 compensation exists in code but failure ordering is unclear
    target: Document ordering and verify cleanup behavior
    notes: Triggered by requirement rewrite in SPEC.md
    affects_requirements:            # required for change/remove/repair; omitted for add
      - REQ-006
    updates:                         # derived from intent
      - replace_requirement

# --- Requirements ---
requirements:
  - id: REQ-006                      # exactly REQ-001, REQ-002, ... no gaps
    title: Compensation stage 4 failure cleanup  # derived from Feature: line
    tags:                            # derived from Gherkin @tag lines
      - runtime
      - e2e
    lifecycle: superseded            # active | superseded | withdrawn
    verification: verified           # unverified | verified | stale
    introduced_by: D-007             # delta ID
    superseded_by: REQ-007           # optional, present when superseded
    test_files:                      # repo-relative paths
      - runtime/tests/e2e/journeys/test_compensation_cleanup.py
    gherkin: |                       # normalized requirement-level block (tags + Feature line)
      @runtime @e2e
      Feature: Compensation stage 4 failure cleanup

  - id: REQ-007
    title: Compensation stage 4 failure cleanup
    tags:
      - runtime
      - e2e
    lifecycle: active
    verification: unverified
    introduced_by: D-008
    supersedes: REQ-006              # optional, present on replacement
    test_files: []
    gherkin: |
      @runtime @e2e
      Feature: Compensation stage 4 failure cleanup

# --- Changelog ---
changelog:
  - rev: 9                           # int
    date: 2026-03-31                 # YYYY-MM-DD
    deltas_opened:                   # auto-managed structured fields
      - D-008
    deltas_closed: []
    reqs_added:
      - REQ-007
    reqs_verified: []
    summary: Reworked compensation cleanup requirement tracking
```

Auto-managed fields (written by specctl, not by humans):
`status`, `rev`, `updated`, `last_verified_at`, `checkpoint`,
`deltas[].origin_checkpoint`, changelog structured fields.

Field constraints:
- `slug` and `charter` match `^[a-z0-9][a-z0-9-]*$`
- `title` is required and non-empty
- `status` is one of `draft|ready|active|verified`
- `rev >= 1`
- Date fields use `YYYY-MM-DD`
- `checkpoint` and `origin_checkpoint` are explicit git SHAs
- `tags` is an ordered unique set matching `^[a-z0-9][a-z0-9-]*$`
- `documents.primary` is a normalized repo-relative markdown path
- `documents.primary` must be under at least one declared `scope[]`
- Every `scope[]` entry is a repo-relative directory ending in `/`
- `deltas[].id` are exactly `D-001`, `D-002`, ... with no gaps or reordering
- `requirements[].id` are exactly `REQ-001`, `REQ-002`, ... with no gaps or reordering
- `requirements[].title` is derived from `Feature:`
- `requirements[].tags` are derived from the Gherkin tags
- `requirements[].gherkin` stores only the requirement-level block (tags + Feature line), not scenarios
- All stored paths are slash-normalized, repo-relative, and must not contain `..`

Normalization rules for requirement matching:
- Normalize line endings to `\n`
- Trim trailing spaces
- Trim leading and trailing blank lines inside the fenced block
- Preserve words, case, punctuation, and tag order

### A.2 Charter Schema

```yaml
# File path: .specs/{name}/CHARTER.yaml
name: runtime                        # string, must equal directory name
title: Runtime System                # string, required
description: Specs for runtime control-plane and data-plane behavior  # required, non-empty

groups:
  - key: execution                   # unique, matches ^[a-z0-9][a-z0-9-]*$
    title: Execution Engine          # string
    order: 10                        # int >= 0
  - key: recovery
    title: Recovery and Cleanup
    order: 20

specs:
  - slug: redis-state                # unique, must correspond to tracking file
    group: execution                 # must reference a declared group key
    order: 10                        # int >= 0
    depends_on: []                   # same-charter slugs only; no self-dep, no cycles
    notes: Storage and CAS guarantees  # required, non-empty

  - slug: session-lifecycle
    group: recovery
    order: 20
    depends_on:
      - redis-state
    notes: Session FSM and cleanup behavior
```

Constraints:
- File path is exactly `.specs/{name}/CHARTER.yaml`
- `name` must equal the directory name
- `groups[].key` values are unique and match `^[a-z0-9][a-z0-9-]*$`
- `specs[].slug` values are unique and must correspond to tracking files in the same charter directory
- `specs[].group` must reference a declared group
- `depends_on` may reference only spec slugs in the same charter
- Self-dependency is invalid
- Cycles are invalid
- `description` is required and non-empty
- `groups[].order` and `specs[].order` are integers `>= 0`
- `specs[].notes` is required and non-empty

### A.3 Config Schema

```yaml
# File path: .specs/specctl.yaml
gherkin_tags:                        # CLI-managed; each matches ^[a-z0-9][a-z0-9-]*$
  - runtime
  - domain
  - ui
  - integration
  - contract
  - workflow

source_prefixes:                     # CLI-managed; each ends with /
  - runtime/src/
  - ui/src/
  - ui/convex/
  - ui/server/

formats:                             # human-authored; read-only to CLI
  ui-spec:
    template: ui/src/routes/SPEC-FORMAT.md       # repo-relative path
    recommended_for: ui/src/routes/**            # glob for auto-selection
    description: 8-section literate UI spec      # human-readable
  e2e-context:
    template: runtime/tests/e2e/CONTEXT-FORMAT.md
    recommended_for: "**/tests/e2e/**"
    description: E2E journey context document
```

Built-in semantic tags (not stored in config): `e2e`, `manual`.

Ownership rules:
- `gherkin_tags` and `source_prefixes` are CLI-managed
- `formats` is human-authored
- Config write commands must preserve `formats` semantically unchanged
