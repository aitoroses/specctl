# Specification Format Template

This template defines the structure for each behavioral surface section
in a specctl-governed specification. The design doc is the source of
truth for the system's behavior. specctl tracks requirements via Gherkin
blocks for navigation and governance, but the spec itself must be
complete enough that an agent could reimplement the system from it alone.

Each behavioral surface follows this structure:

---

## <Surface Name>

<!-- Explain what this behavioral surface does, why it exists, and when
it's used in the agent workflow. Write from the external surface — what
a caller or user observes, not how the implementation works internally.
Include the surface's role in the overall system and what problem it
solves. This prose is the primary specification — everything below
supports it. -->

### Data Model

<!-- Define the schema fields this surface reads or writes. Include
field names, types, optionality, and relationships to other model
objects. For example, if this surface mutates the tracking file, show
which YAML fields are affected and their valid values. If it reads
from config, describe which config fields matter and why. -->

### Contracts

<!-- Show exact JSON response shapes for each response path. Include
at minimum: one success response and one response per error code the
surface can return. Use concrete field names and realistic values.
Mark dynamic fields with placeholders like <HEAD_SHA> or <today>.

Example structure:

Success:
```json
{
  "state": { "status": "verified", "rev": 2, ... },
  "focus": { ... },
  "result": { "kind": "...", ... },
  "next": { "mode": "sequence", "steps": [...] }
}
```

Error (ERROR_CODE):
```json
{
  "state": { ... },
  "focus": { "reason": "..." },
  "next": { "mode": "none" },
  "error": { "code": "ERROR_CODE", "message": "..." }
}
```
-->

### Invariants

<!-- List rules that must always hold for this surface. These are the
constraints that make the behavior unambiguous. Write each as a
declarative statement. Examples:

- Delta close requires all introduced requirements to be verified
- Repair deltas must declare affects_requirements with active IDs
- Rev bump is gated on closed deltas not yet in the changelog

If a rule has exceptions, state them explicitly. -->

## Requirement: <Observable Behavior Title>

<!-- The requirement heading and Gherkin block below are for specctl
tracking and quick navigation. They anchor the behavioral contract
to the governance system. The Gherkin Feature line must match this
heading title. Tags must be configured in specctl.yaml. -->

```gherkin requirement
@<tag> @<tag>
Feature: <Observable Behavior Title>
```

### Scenarios

<!-- Each scenario is one observable contract. Write from the external
surface using Given/When/Then. Reference specific error codes, field
values, and state transitions — not vague descriptions. The scenarios
summarize the contracts section above in testable form.

A surface may have multiple Requirement blocks if it covers distinct
behavioral groups. -->

```gherkin scenario
Scenario: <Descriptive name>
  Given <precondition referencing specific state>
  When <action the agent takes>
  Then <observable outcome with specific values or codes>
```

### Journey

<!-- OPTIONAL: Include this section when the surface participates in
a multi-step workflow. Show the surface's role in the sequence:
what comes before it, what it does, and what comes after. Use
command names and reference the next actions that connect the steps.

Example:
  delta add (this surface) -> write_spec_section -> req add -> ...

Skip this section for standalone surfaces like config or hook. -->
