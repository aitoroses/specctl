---
spec: mcp
charter: specctl
format: behavioral-spec
---

# specctl MCP Adapter

The MCP adapter exposes specctl's governed workflow to MCP clients. It is
transport-specific: it defines how specctl tools are advertised, how
uninitialized repositories still surface an initialization path, and how
agent-facing `next` guidance is translated into executable MCP tool hints.
It does not redefine core governance semantics such as drift rules,
requirement lifecycle, or revision logic — those remain governed by
`specctl:cli`.

## MCP Tool Surface

The MCP server exposes a named tool surface that MCP clients can list and call.
A connected client observes `specctl_*` tools for the core governance actions,
with each call returning a JSON envelope encoded as MCP text content. The
adapter's job is to make the existing specctl workflows callable from an MCP
client without changing their meaning.

### Data Model

The MCP tool surface reads and exposes these fields:

| Field | Type | Description |
|-------|------|-------------|
| `implementation.name` | `string` | Always `"specctl"` |
| `implementation.version` | `string` | Current MCP server version string |
| `tools[].name` | `string` | Registered MCP tool name such as `specctl_context` |
| `tools[].description` | `string` | Short human-readable description of the tool |
| `result.content[]` | `array` | MCP content blocks, with specctl envelopes emitted as text |

### Contracts

Success (list tools includes the governed tool surface):
```json
{
  "tools": [
    { "name": "specctl_context", "description": "Read registry, charter, spec, or file ownership context." },
    { "name": "specctl_diff", "description": "Return a semantic diff against the stored checkpoint." },
    { "name": "specctl_requirement_verify", "description": "Mark one requirement as verified." }
  ]
}
```

Success (tool call returns the specctl envelope as MCP text content):
```json
{
  "content": [
    {
      "type": "text",
      "text": "{\"state\":{\"status\":\"verified\"},\"focus\":{},\"next\":{\"mode\":\"none\"}}"
    }
  ],
  "isError": false
}
```

### Invariants

- The MCP surface exposes specctl tools with the `specctl_` prefix
- MCP transport wraps specctl envelopes as text content rather than inventing a second response schema
- Tool registration belongs to the adapter; governance semantics remain owned by `specctl:cli`

## Requirement: MCP server exposes specctl tools to MCP clients

```gherkin requirement
@specctl @mcp
Feature: MCP server exposes specctl tools to MCP clients
```

### Scenarios

```gherkin scenario
Scenario: Listing tools returns the governed specctl MCP surface
  Given a connected MCP client session
  When the client lists tools
  Then the response includes the registered `specctl_*` tools
```

## Uninitialized MCP Surface

The MCP server must remain useful even before `.specs/` exists. In an
uninitialized repository, it still exposes `specctl_init` and returns a
structured `NOT_INITIALIZED` envelope for all other tools. This preserves the
first step of the workflow for MCP clients instead of failing with an opaque
transport error.

### Data Model

| Field | Type | Description |
|-------|------|-------------|
| `state.initialized` | `bool` | `false` when no `.specs/` directory exists |
| `focus.reason` | `string` | Why the tool cannot proceed before initialization |
| `next.steps[].action` | `string` | Initialization recovery step |
| `next.steps[].mcp.tool` | `string` | MCP tool name the client can call next |
| `error.code` | `string` | `NOT_INITIALIZED` |

### Contracts

Error (uninitialized repo):
```json
{
  "state": { "initialized": false },
  "focus": { "reason": "specctl is not initialized in this repository" },
  "next": {
    "mode": "sequence",
    "steps": [
      {
        "action": "initialize",
        "mcp": {
          "available": true,
          "tool": "specctl_init",
          "input": {}
        }
      }
    ]
  },
  "error": {
    "code": "NOT_INITIALIZED",
    "message": "No .specs/ directory found. Call specctl_init to initialize specctl governance in this repository."
  }
}
```

### Invariants

- `specctl_init` remains callable before initialization
- Every other tool returns `NOT_INITIALIZED` with an MCP hint to call `specctl_init`
- Uninitialized behavior is an adapter contract, not a replacement for repository governance

## Requirement: Uninitialized MCP server preserves the initialization path

```gherkin requirement
@specctl @mcp
Feature: Uninitialized MCP server preserves the initialization path
```

### Scenarios

```gherkin scenario
Scenario: Uninitialized repositories expose init and structured guidance
  Given no `.specs/` directory exists
  When an MCP client calls a non-init specctl tool
  Then the response returns `NOT_INITIALIZED` and an MCP hint to call `specctl_init`
```

## MCP Hint Translation Surface

Specctl's read and write surfaces emit `next` guidance that agents follow. The
MCP adapter translates executable actions into MCP hints so an MCP client can
call the next tool directly. For agent-owned steps that are not available in
MCP v1, the adapter marks the hint unavailable and explains that the step must
be handled outside the MCP server.

### Data Model

| Field | Type | Description |
|-------|------|-------------|
| `next.mode` | `string` | Guidance mode carried from the underlying specctl surface |
| `next.steps[].action` | `string` | Action the agent should take next |
| `next.steps[].mcp.available` | `bool` | Whether the action can be executed directly through MCP |
| `next.steps[].mcp.tool` | `string?` | Tool name for MCP-callable steps |
| `next.steps[].mcp.input` | `object?` | Structured MCP input derived from the CLI template |
| `next.steps[].mcp.reason` | `string?` | Why a step is unavailable in MCP v1 |

### Contracts

Success (next action translated into an MCP hint):
```json
{
  "next": {
    "mode": "sequence",
    "steps": [
      {
        "action": "create_charter",
        "mcp": {
          "available": true,
          "tool": "specctl_charter_create",
          "input": { "charter": "runtime" }
        }
      }
    ]
  }
}
```

Success (unsupported prerequisite remains agent-owned):
```json
{
  "next": {
    "mode": "sequence",
    "steps": [
      {
        "action": "write_spec_section",
        "mcp": {
          "available": false,
          "reason": "unsupported_in_v1"
        }
      }
    ]
  }
}
```

### Invariants

- MCP-callable next actions preserve the original tool intent and structured inputs
- Unsupported steps stay explicit via `mcp.available: false`; they are not silently dropped
- Hint translation may enrich the transport, but it must not alter the underlying workflow semantics

## Requirement: MCP tool hints preserve executable next-action guidance

```gherkin requirement
@specctl @mcp
Feature: MCP tool hints preserve executable next-action guidance
```

### Scenarios

```gherkin scenario
Scenario: Next actions expose callable MCP tool hints
  Given a specctl surface returns executable next guidance
  When the MCP client receives the envelope
  Then MCP-callable steps include `mcp.tool` and structured `mcp.input`
```

```gherkin scenario
Scenario: Unsupported agent-owned prerequisites stay explicit
  Given the next action requires a non-MCP file edit step
  When the MCP client receives the envelope
  Then the step remains present with `mcp.available` set to false
```


## Delta Withdraw and Rebind Tool Surface

The MCP bridge uses an explicit allowlist in `registerTools` and a parallel
stub list in `registerUninitializedTools`. New CLI verbs do not appear in the
MCP surface until they are registered there by hand. Keeping the surfaces
aligned is a governance obligation on `specctl:mcp`.

### Data Model

Two new tools:

- `specctl_delta_withdraw` with input `{ spec, delta_id, reason }` wraps
  `application.Service.WithdrawDelta`. `reason` is required.
- `specctl_delta_rebind_requirements` with input
  `{ spec, delta_id, from, to?, remove?, reason? }` wraps
  `application.Service.RebindDeltaRequirements`. Either `to` or `remove`
  must be set. `reason` is required when `remove` is true and optional
  (but carried through) when `to` is set.

### Invariants

- Every CLI subcommand that agents are expected to reach through MCP has a
  corresponding entry in both `registerTools` and
  `registerUninitializedTools`.
- Tool input JSON Schemas mirror the CLI flags one-for-one, including
  optionality and the mutual exclusion between `to` and `remove`.
- `TestListTools` in `server_test.go` is updated whenever a tool is added or
  removed; the expected list is sorted and exhaustive.

## Requirement: MCP exposes delta withdraw and rebind-requirements as first-class tools

```gherkin requirement
@specctl @mcp
Feature: MCP exposes delta withdraw and rebind-requirements as first-class tools
```

### Scenarios

```gherkin scenario
Scenario: specctl_delta_withdraw appears in the MCP tool list
  Given the MCP server is initialized
  When the client calls ListTools
  Then the returned names include specctl_delta_withdraw
```

```gherkin scenario
Scenario: specctl_delta_rebind_requirements appears in the MCP tool list
  Given the MCP server is initialized
  When the client calls ListTools
  Then the returned names include specctl_delta_rebind_requirements
```
