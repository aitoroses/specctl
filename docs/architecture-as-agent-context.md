# Architecture as Agent Context

## What this is

A small, typed read model that gives agent harnesses the architectural facts
they need at the moment they need them.

It is **not** an enforcement system. It is **not** a replacement for static
analyzers. It is **not** a documentation framework. It is the architectural
equivalent of `specctl context` for behavioral state: a compact, diffable
artifact that an agent consults before changing code, and updates when its
change implies an architectural fact.

## Why specctl needs it

`specctl` already governs behavioral intent (requirements, deltas, revisions,
verification). Architectural intent — boundaries, allowed dependencies,
accepted decisions — currently lives in prose, code structure, and reviewer
memory. Agents cannot reliably consult any of those.

The cost of that gap shows up as:

- agents proposing changes that cross boundaries they did not know existed
- decisions getting silently re-litigated because prior rationale is not in context
- cross-surface coupling appearing without anyone declaring it

The fix is not heavier process. It is making architecture **legible to the
agent loop**.

## Design principles

1. **Read model first.** The primary deliverable is what `context` returns,
   not what CI blocks.
2. **Declared, not proven.** The artifact is the source of truth. Validation
   is structural by default; topology checks are opt-in adapters around tools
   the team already has.
3. **`unknown` is honest.** Most architectural claims are not mechanically
   verifiable. Saying so is better than fabricating green check marks.
4. **Few primitives.** Components, interfaces, constraints, decisions. Nothing
   else ships in v1.
5. **Coarse first.** 5–12 components per artifact. Subcomponents only after
   repeated ambiguity.
6. **Trigger-driven updates.** Edit only when a boundary, rule, or accepted
   decision actually changes. Refactors inside a component require nothing.

## Code architecture vs system architecture

The two have different units, different evidence, and different shapes. They
should not share an artifact.

| | **Code architecture** | **System architecture** |
|---|---|---|
| Units | packages, modules, layers | services, processes, data stores |
| Interfaces | function/type boundaries | network/API/event contracts |
| Evidence (when available) | static analysis of source | IaC, manifests, runtime traces |
| Natural shape | DAG (acyclic dependencies are a goal) | graph (cycles are normal) |
| Change cadence | per PR | per release/deploy |
| Stability | medium | high |
| Cross-charter relevance | rare | frequent |

This produces two artifacts per charter, with the same schema but different
defaults:

```text
.specs/<charter>/code.arch.yaml
.specs/<charter>/system.arch.yaml
```

Either may be absent. A charter with no system architecture (a pure library)
ships only `code.arch.yaml`. The cross-charter concept collapses into
`system.arch.yaml` references that happen to point across charters — no
separate primitive needed.

## The model

Four primitives, identical schema in both artifacts.

```yaml
format: architecture/v1
kind: code | system
components:
  - id: application_core
    responsibility: workflow semantics and lifecycle rules
    owner: specctl:cli
interfaces:
  - id: cli_to_app
    from: cli_surface
    to: application_core
    kind: function_api | http | event | cli
    stability: experimental | stable | deprecated
constraints:
  - id: no_domain_to_infra
    rule: "domain_model -> infrastructure_adapters = forbidden"
    severity: guidance | advisory | required
    check: null | { adapter: "...", config: {...} }
decisions:
  - id: dec-transport-split
    title: "MCP adapter remains a transport boundary"
    status: proposed | accepted | deprecated
    supersedes: []
    rationale: "<= 240 chars"
```

### Severity, redefined for a context system

- **guidance**: the agent should consider it; not blocking.
- **advisory**: the agent must acknowledge it in the delta; not blocking.
- **required**: the agent must satisfy it or open an exception; blocks
  `rev bump` only when a check exists and reports `fail`.

Severity describes **how the agent should treat the rule**, not what CI does.
CI integration is an emergent consequence, not the primary product.

### Exceptions

Time-bounded, owner-attributed, remediation-linked. Same shape as the original
proposal — kept because they're the right escape hatch — but only meaningful
for `required` constraints with a real check. Otherwise unnecessary.

## How agents see it

### `specctl context <charter:slug>`

Existing behavioral output gains an `architecture` block:

```yaml
architecture:
  code:
    components_touched: [application_core, cli_surface]
    active_constraints:
      - id: no_domain_to_infra
        severity: required
        check_status: pass
    relevant_decisions:
      - id: dec-transport-split
        status: accepted
  system:
    status: untracked
  next:
    - "no architecture edit needed for this delta"
```

The agent now plans against architectural facts without re-reading prose.

### `specctl arch context <charter:slug>`

Full read of both artifacts plus computed facts (orphan components, stale
exceptions, decisions referenced by current delta). Used when the agent
explicitly needs deeper context.

### Mutations

Minimal command surface, mirroring existing `req` / `delta` patterns:

```bash
specctl arch component  add | update | remove
specctl arch interface  add | update | remove
specctl arch constraint add | update | remove
specctl arch decision   add | supersede | deprecate
specctl arch exception  add | resolve
```

No `arch check` command in v1. Checks run as part of `specctl context` and
`specctl req verify`, surfaced inline.

### MCP

One read tool and one mutate tool. That's it.

- `specctl_arch_context`
- `specctl_arch_mutate`

`arch_check` and `arch_diff` are deferred until evidence adapters exist worth
calling separately.

## Evidence: the honest version

Evidence has three tiers. Only the first is required.

### Tier 1: structural validation (always-on)

Cheap, deterministic, no external tools.

- IDs unique within an artifact
- references resolve (interface endpoints, decision links, exception scopes)
- enum values valid
- exceptions well-formed (owner, expiry, remediation)
- accepted decisions reference at least one component or interface

These are the only rules that ship enabled by default.

### Tier 2: topology adapters (opt-in)

A constraint may declare a `check` block that names an adapter. The adapter
runs whatever tool already exists and reports `pass | fail | unknown`.

```yaml
constraints:
  - id: no_domain_to_infra
    rule: "domain_model -> infrastructure_adapters = forbidden"
    severity: required
    check:
      adapter: depguard
      config:
        rules: [...]
```

specctl does not implement language-specific analyzers. It defines the adapter
contract and ships zero adapters in v1. Teams wire their existing tooling.

### Tier 3: runtime / system evidence (future)

Out of scope for v1. Documented as a future direction so the model does not
have to change to accommodate it later.

### Why `unknown` is the default

For most constraints in most repos, no adapter will be configured. The status
will be `unknown`. That is correct: specctl is reporting the truth that the
claim is declared but not mechanically verified. The agent reads the claim and
must respect it; the system does not pretend it has proof.

## Lint catalog (v1)

Five rules. All structural. Always on.

| ID | Severity | Description |
|---|---|---|
| `ARCH_ID_UNIQUE` | error | IDs unique within an artifact |
| `ARCH_REF_RESOLVED` | error | All references point to known entities |
| `ARCH_DECISION_LINKED` | warn | Accepted decisions reference ≥1 component/interface |
| `ARCH_EXCEPTION_COMPLETE` | error | Exceptions have owner, expiry, remediation |
| `ARCH_EXCEPTION_EXPIRED` | error | Expired exceptions block `rev bump` |

Topology rules (`ARCH_FORBIDDEN_EDGE`, `ARCH_LAYER_ORDER`,
`ARCH_INTERFACE_DECLARED`) are deferred. They require adapters specctl does
not yet have, and they belong to the constraint's `check` block, not a global
catalog.

## Conventions

### Naming

- Component IDs: `snake_case` nouns (`application_core`)
- Interface IDs: `<from>_to_<to>` (`cli_to_app`)
- Constraint IDs: imperative intent (`no_domain_to_infra`)
- Decision IDs: stable sequence (`dec-transport-boundary`)

### Granularity

- 5–12 components per artifact. Cap, not target.
- Coarse boundaries first; subcomponents only after repeated ambiguity.
- Free-text fields capped (rationale ≤ 240 chars).

### When to edit

Edit an architecture artifact only when one of these is true:

- a component or interface is added/removed
- a dependency rule is added, removed, or knowingly violated
- an accepted decision is superseded
- a system-level export/import changes

Refactors inside a component, renames, and behavioral-only changes require
nothing.

## Operating model

The agent loop becomes:

```text
specctl context <charter>
  -> read behavioral + architectural state
delta open
  -> if change crosses a boundary: arch mutate (component/interface/decision)
implementation
specctl req verify
  -> structural arch validation runs inline
  -> any configured topology adapters run inline
delta close
specctl rev bump
  -> blocks only on: unresolved required+failing checks, expired exceptions
```

Architecture is additional evidence in the existing loop. There is no
separate process, no separate ceremony, no separate review track.

## Rollout

A single phase to start.

**Phase 1 (this proposal):** ship the read model, the mutation commands, the
five structural lint rules, and the `architecture` block in `specctl context`.
Commit one real `code.arch.yaml` for `specctl:cli` as the dogfood example.

That is the entire deliverable.

Subsequent phases are listed as future work, not committed:

- topology adapter contract + reference adapter for one language
- `system.arch.yaml` adoption across charters
- delta ↔ arch linkage as a typed field
- CI policy templates

Each future phase stands or falls on its own evidence. None are required for
the read model to be useful.

## Success criteria

The proposal succeeds if, after Phase 1:

- agents reliably surface relevant architectural facts when planning a delta
- at least one prior decision avoids being re-litigated because it appeared in
  `context`
- editing the architecture artifact takes less time than reviewers spend
  reconstructing the same facts from prose

It does not need to catch violations. That is a different system, and one
specctl is not the right place to build.

## What this proposal explicitly drops from the prior version

- enforcement framing as the primary value
- the cross-charter primitive (folded into `system.arch.yaml`)
- the topology lint rules in v1 (deferred to adapter contract)
- the multi-stage dogfood plan (collapsed to "ship one example artifact")
- separate `arch check` and `arch diff` commands (folded into existing flows)
- evidence claims the system cannot back up
