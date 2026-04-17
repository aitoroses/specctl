# Behavioral Specification Format

This template defines the structure for each behavioral surface section
in a specctl-governed specification. The design doc is the source of
truth for the system's behavior. specctl tracks requirements via Gherkin
blocks for navigation and governance, but the spec itself must be
complete enough that an agent could reimplement the system from it alone.

## Document Frontmatter

```yaml
---
spec: <slug>
charter: <charter-name>
format: behavioral-spec
---
```

## Per-Surface Structure

Each behavioral surface follows five layers:

### 1. Prose

Explain what this behavioral surface does, why it exists, and when
it's used. Write from the external surface — what a caller or user
observes, not how the implementation works internally. Include the
surface's role in the overall system and what problem it solves.
This prose is the primary specification — everything below supports it.

### 2. Data Model

Define the schema fields this surface reads or writes. Include
field names, types, optionality, and relationships. Example:

| Field | Type | Description |
|-------|------|-------------|
| `slug` | `string` | Spec identifier |
| `status` | `draft\|ready\|active\|verified` | Derived lifecycle |
| `requirements[]` | `array` | Each with id, title, lifecycle |

### 3. Contracts

Show exact JSON/response shapes for each response path. Include at
minimum: one success response and one response per error code. Use
concrete field names and realistic values. Mark dynamic fields with
placeholders like `<HEAD_SHA>`.

```json
{
  "state": { "status": "verified", "rev": 2 },
  "focus": {},
  "next": { "mode": "none" }
}
```

### 4. Invariants

List rules that must always hold. Write each as a declarative statement:

- Delta close requires all introduced requirements to be verified
- Requirement title is derived from Gherkin, never free-text
- `next` is never null and always present

### 5. Gherkin Tracking

Each requirement block in the document:

- Starts at a `## Requirement:` heading (h2 level)
- Contains exactly one `gherkin requirement` fenced block
- May contain `gherkin scenario` fenced blocks
- Ends at the next `## Requirement:` heading or end-of-document

```markdown
## Requirement: Context reports spec status and drift state

\`\`\`gherkin requirement
@specctl @read
Feature: Context reports spec status and drift state
\`\`\`

### Scenarios

\`\`\`gherkin scenario
Scenario: Clean spec returns clean drift status
  Given a tracked spec with no changes since checkpoint
  When the agent runs specctl context
  Then status is clean and next is none
\`\`\`
```

### 6. Journey (optional)

Include when the surface participates in a multi-step workflow. Show
the surface's role in the sequence: what comes before, what it does,
and what comes after. Use command names and reference connecting actions.

```text
context → diff → delta add --intent add → write_spec_section →
req add → implement_and_test → req verify → delta close → rev bump
```

Skip this section for standalone surfaces like config or hook.

## Writing Guidelines

- **External surface only** — describe what a caller observes, not internals
- **Concrete contracts** — real JSON shapes, not prose descriptions
- **Testable invariants** — each rule verifiable by an automated check
- **Gherkin as anchor** — Feature line = requirement title, tags = classification
- **Scenarios summarize contracts** — Given/When/Then in testable form
- **One requirement per distinct behavior** — not per endpoint or entity

## Running `specctl example`

The specctl binary embeds its own spec as a working reference. Run
`specctl example` to see all five layers applied to a real system —
the tool's own behavioral specification.
