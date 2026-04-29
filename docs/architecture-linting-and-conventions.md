# Architecture Linting Rules and Conventions

## Goal

Define a **small, enforceable** architecture lint profile that keeps architecture state accurate enough to act as a system map, while avoiding high maintenance overhead.

This document is intentionally opinionated for agent usability:

- low number of rules
- deterministic rule outputs
- explicit `error | warn | info` severities
- gradual adoption path

## Design constraints for linting

A lint rule is acceptable only if it is:

1. **Machine-checkable** from repo state (or declared unknown)
2. **Actionable** with a clear remediation path
3. **Stable** across normal refactors
4. **Low-noise** in brownfield repositories

If a proposed rule fails one of the above, keep it out of the default profile.

## Canonical lint profile (v1)

### A) Structural integrity rules (always-on)

These prevent `ARCH.yaml` from becoming stale or contradictory.

1. `ARCH_ID_UNIQUE` (`error`)
   - IDs for components/interfaces/constraints/decisions must be unique within a charter.
2. `ARCH_REF_RESOLVED` (`error`)
   - interface endpoints and dependency references must point to known components.
3. `ARCH_DECISION_LINKED` (`warn`)
   - accepted decisions should reference at least one affected component/interface.
4. `ARCH_EXCEPTION_COMPLETE` (`error`)
   - exceptions require owner, expiry, and remediation reference.

### B) Topology consistency rules (default-on)

These validate architecture claims against code topology.

5. `ARCH_FORBIDDEN_EDGE` (`error`)
   - disallow explicitly forbidden dependency edges.
6. `ARCH_LAYER_ORDER` (`warn` -> `error` when mature)
   - enforce declared layer direction (e.g., domain cannot depend on infrastructure).
7. `ARCH_INTERFACE_DECLARED` (`warn`)
   - cross-component calls should map to a declared interface entry.
8. `ARCH_ORPHAN_COMPONENT` (`info`)
   - flag components with no incoming/outgoing relationships.

### C) Lifecycle and governance rules (opt-in)

These keep change history meaningful.

9. `ARCH_DELTA_LINK_REQUIRED` (`warn`)
   - boundary-changing deltas should include architecture linkage.
10. `ARCH_EXCEPTION_EXPIRED` (`error`)
   - expired exceptions block readiness until renewed/resolved.
11. `ARCH_CROSS_CHARTER_COMPAT` (`warn` -> `error` selectively)
   - imports must satisfy exported compatibility contracts.

## Conventions that keep architecture low-entropy

### Naming conventions

- Component IDs: `snake_case` nouns (`application_core`)
- Interface IDs: `<from>_to_<to>_<kind>` (`cli_to_app_function_api`)
- Constraint IDs: imperative intent (`no_domain_to_infra`)
- Decision IDs: stable sequence (`dec-transport-boundary`)

### Granularity conventions

- Prefer coarse components first (5-12 per charter)
- Model stable boundaries, not transient code structure
- Add subcomponents only after repeated ambiguity in reviews

### Text conventions

- Limit free-text fields (e.g., rationale ≤ 240 chars)
- Put policy in typed fields, not prose
- Use check IDs in comments and PR notes (`ARCH_FORBIDDEN_EDGE`)

### Change conventions

Require architecture edits only when one of these occurs:

- new component or interface
- dependency rule added/removed/violated
- accepted decision superseded
- cross-charter contract changed

No architecture edit required for intra-component refactors.

## Agent-optimized output conventions

Architecture lint output should always include:

- `rule_id`
- `severity`
- `status` (`pass | fail | unknown`)
- `evidence` (file/edge/reference)
- `suggested_fix` (single actionable step)

This keeps agent loops deterministic and remediation-friendly.

## Alternatives considered (and tradeoffs)

### Option 1: Rich architecture DSL with many rule types

**Pros**

- very expressive
- can model complex enterprise constraints

**Cons**

- high learning curve for agents
- high false-positive risk
- heavy maintenance burden

**Verdict**: rejected for default profile (too much complexity).

### Option 2: Pure prose ADR/process with no linting

**Pros**

- easy to start
- minimal tooling effort

**Cons**

- non-deterministic for agents
- drifts quickly in brownfield repos
- poor CI enforceability

**Verdict**: rejected (too little operational value).

### Option 3: Minimal typed model + compact lint profile (recommended)

**Pros**

- low overhead
- deterministic checks
- incremental enforcement path
- works for both agents and humans

**Cons**

- less expressive than full DSL
- some manual modeling still required

**Verdict**: recommended as the best simplicity/utility balance.

## Recommended default policy for integrations

Use this default until maturity is proven:

- block on: `ARCH_ID_UNIQUE`, `ARCH_REF_RESOLVED`, `ARCH_FORBIDDEN_EDGE`, `ARCH_EXCEPTION_COMPLETE`, `ARCH_EXCEPTION_EXPIRED`
- warn on: `ARCH_LAYER_ORDER`, `ARCH_INTERFACE_DECLARED`, `ARCH_DELTA_LINK_REQUIRED`, `ARCH_CROSS_CHARTER_COMPAT`
- info on: `ARCH_ORPHAN_COMPONENT`, `ARCH_DECISION_LINKED`

Promote warn->error only after 2-4 weeks of low-noise signal.

## Why this avoids documentation burden

- small fixed rule catalog
- conventions define *when not to edit architecture*
- coarse-first modeling prevents overfitting
- lint output includes direct suggested fixes
- progressive enforcement avoids blocking teams before signal is trustworthy

In short: architecture state stays useful as a map because it is minimal, typed, and continuously checked — not because it is verbose.
