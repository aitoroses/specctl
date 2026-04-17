# Brownfield Crystallization

Turn an existing codebase into a governed specctl surface without
creating documentation sprawl, false verification claims, or
unstable topology.

This is the rigorous brownfield adoption workflow. Use it when the
repo has meaningful behavior in docs, ADRs, plans, code, and tests
but no specctl governance. The lightweight steps in SKILL.md give
the overview — this reference provides the gated methodology.

## Core Principles

- Group **specs by external behavioral surface** (what callers observe)
- Group **charters by invariant or contract family**
- Let `specctl_context` + `next` drive governed actions
- Use **primary docs** for meaning, **secondary docs** for refinement,
  **tests** for verification maturity only
- Never silently reconcile contradictions
- Prefer crystallization over premature coverage
- Widen scope only through explicit review gates

## Source Precedence

When sources disagree, apply this hierarchy (highest wins):

1. Approved governed `.specs/` docs (already-governed surfaces)
2. Primary docs for not-yet-governed surfaces
3. ADR invariants
4. Approved planning artifacts
5. Explicit specctl lifecycle state
6. Secondary docs
7. Process/audit log
8. Tests (verification maturity and drift evidence only)
9. Code (implementation evidence only)

If sources still conflict, emit explicit outputs:
- **Defer** — not enough clarity to govern yet
- **Open delta** — track the contradiction as a gap
- **Mark drift** — specctl diff shows the divergence
- **Create repair task** — fix the contradiction before governing

## Verification Maturity Labels

Use exactly these labels when classifying requirement evidence:

| Label | Meaning | specctl action |
|-------|---------|---------------|
| **verified** | Specific test file maps clearly to the requirement | `specctl_requirement_verify` with `--test-file` |
| **partially verified** | Tests cover some scenarios but not all | Verify with available files, open delta for gaps |
| **unverified** | No test evidence exists | Register requirement, leave unverified |
| **manual-only** | Verified by inspection, not automated test | `@manual` tag on the requirement |

A requirement is **verified** only when a specific test artifact maps
to it clearly. Do NOT mark verified because:
- The doc claims the behavior exists
- The code appears to implement it
- A test file exists nearby but doesn't test this specific behavior

## Requirement-to-Document Matching

For every requirement candidate:

1. Identify the target external surface (what a caller observes)
2. Identify the primary doc (the meaning owner)
3. Identify supporting secondary docs
4. Identify relevant ADR invariants
5. Call `specctl_context` and follow `next`
6. Only register the requirement AFTER governed prose already exists

**Never force a requirement into a spec just because the code
contains it.** The spec documents observable behavior from the
external surface. Code that has no external behavioral contract
is not a requirement — it's implementation.

## The 8-Phase Workflow

### Phase 1 — Brownfield Grounding

Read the repo. Understand what exists before changing anything.

1. Scan for existing artifacts: specs, ADRs, planning docs, contracts,
   tests, relevant code
2. Call `specctl_init` → creates `.specs/` + config with detected prefixes
3. Call `specctl_context` → learn whether any governance already exists
4. Record the current state in a process log
5. If intent is still ambiguous, run a clarification pass (deep-interview
   or direct questioning) before planning

**Output:** inventory of existing behavioral material + initial process log.

### Phase 2 — Clarify the Governance Model

Lock these decisions before planning deeper:

- **Governance home:** `.specs/` (always)
- **Spec grouping:** by external behavioral surface
- **Charter grouping:** by invariant/contract family
- **Coverage target:** which surfaces matter most?
- **Explicit deferrals:** what is NOT being governed this wave?
- **Process log path:** where the session is recorded

For most brownfield repos the defaults are right:
- `.specs/` is governance home
- Specs group by external behavioral surface
- Charters group by invariant/contract family

### Phase 3 — Consensus Planning

Produce an approved planning package before governed writing:

The plan must define:
- Source precedence (use the hierarchy above)
- Ownership boundaries (which doc owns which surface)
- Contradiction outputs (defer / delta / drift / repair)
- Verification maturity rubric (the 4 labels above)
- Topology freeze gate (what must be approved before writing)
- First-wave policy (which surfaces to govern first)
- Widening rules (how later waves get admitted)

With oh-my-claudecode: use `/ralplan` for Planner → Architect → Critic
consensus. Without: produce the plan manually and review before proceeding.

### Phase 4 — Freeze-Evidence Package

Build a review package proving the first wave is safe to govern.
Five artifacts:

**1. Surface Extraction Matrix**

For each candidate surface:

| Field | Content |
|-------|---------|
| Actor/caller | Who interacts with this surface? |
| External behavior | What does the caller observe? |
| Primary doc | Which doc owns the meaning? |
| Secondary docs | Supporting references |
| Applicable ADRs | Invariants that constrain this surface |
| Code touchpoints | Key files implementing the behavior |
| Test evidence | Existing tests + maturity label |
| Overlap risks | Does this surface overlap another? |
| Recommendation | Standalone / merge / defer |

**2. Source Precedence Map**

Which doc is the meaning owner for each surface? Where do sources
disagree? What's the resolution?

**3. Req→Doc Matching Matrix**

For each candidate requirement: which doc section contains the
behavioral contract? Is the prose written? Is the Gherkin parseable?

**4. Verification Maturity Map**

For each candidate requirement: which tests prove it? What's the
maturity label? What's missing?

**5. First-Wave Shortlist**

The conservative list of surfaces to govern first. Strongest
candidates: clear meaning owner, existing tests, low overlap risk.

### Phase 5 — Topology Freeze Review

Before any governed writing, approve:

- [ ] Charter policy (names, grouping, count)
- [ ] Split/merge rules (when to combine or separate surfaces)
- [ ] Ownership boundaries (which doc owns which surface)
- [ ] Source precedence (tiebreaker hierarchy)
- [ ] Contradiction handling (defer/delta/drift/repair)
- [ ] Verification rubric (4 maturity labels)
- [ ] First-wave shortlist (approved surfaces only)

**If any part is unresolved, keep the work read-only.**

### Phase 6 — Govern Approved Surfaces Sequentially

For each approved surface, one at a time:

1. Create or reuse the charter → `specctl_charter_create`
2. Create the spec entry → `specctl_spec_create`
3. Write the normalized governed SPEC.md (five-layer format)
4. Attach primary and secondary docs → `specctl_doc_add`
5. Add one delta → `specctl_delta_add`
6. Add requirement blocks matching the spec headings → `specctl_requirement_add`
7. Verify each requirement with concrete test files → `specctl_requirement_verify`
8. Close the delta → `specctl_delta_close`
9. Bump the revision → `specctl_revision_bump`
10. Checkpoint the process log

**Do not widen scope mid-iteration.**

### Phase 7 — Re-verify After Each Surface

After each governed surface:

- Call `specctl_context` → confirm the spec is valid
- Confirm requirement verification status is correct
- Run the repo's test/lint commands (no regressions introduced)
- Update the process log with the new governed state

### Phase 8 — Hold or Widen Through a New Gate

After a wave completes:

- **Stop automatic widening** — do not keep going
- Create a new review artifact for deferred candidates
- Admit only the strongest next candidate, if explicitly approved
- Keep all others deferred until another review says otherwise

This is the discipline that prevents brownfield adoption from
becoming documentation sprawl. Each wave is governed, verified,
and frozen before the next begins.

## Process/Audit Log

Maintain one process log per crystallization session. The log is
canonical only for workflow history — never for governed meaning.

The log tracks:
- Session metadata (who, when, why)
- Mode timeline (which phase, which surface)
- Evidence gathered (what was found in the repo)
- Decisions taken (and why)
- Contradictions/drift found
- Artifact creation and state evolution
- Current approved state
- Deferred/open questions

The log must NOT become the meaning owner for surfaces or
requirements. The governed `.specs/` docs own meaning.

## Example Progression

A real brownfield crystallization on a Python runtime service:

1. Control-plane daemon lifecycle and capabilities
2. Data-plane session runtime lifecycle
3. Execution materialization and launch plan
4. Session event stream and monitor routing
5. Narrow auth-provider metadata lookup
6. Narrow dashboard mode / same-origin proxy

Each surface was governed sequentially, verified with real tests,
and frozen before the next was admitted. The process took multiple
waves with explicit review gates between them.

## Success Criteria

The crystallization is successful when:

- [ ] `.specs/` contains only approved, low-entropy governed surfaces
- [ ] Every governed surface has one clear meaning home (primary doc)
- [ ] Every governed requirement has explicit verification status
- [ ] Verification uses real test files or explicit `@manual` evidence
- [ ] Deferred surfaces stay deferred until explicitly reviewed
- [ ] The process log explains what happened without becoming a meaning source
- [ ] A future agent could repeat the workflow on another repo
