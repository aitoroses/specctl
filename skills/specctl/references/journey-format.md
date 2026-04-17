# JOURNEYS.md Format

Journey specifications describe natural user stories derived from
behavioral requirements. Each journey is something a real person
would live in one sitting — not a test case.

## Frontmatter

```yaml
---
spec: <spec-slug>
source: <path/to/SPEC.md>
crystallized: <YYYY-MM-DD>
convention: "{Prefix}{N}-{semantic-name}"
---
```

## Naming Convention

Journey IDs use a scope prefix + sequential number + semantic name:

```
{Prefix}{N}-{semantic-name}
```

Derive the prefix from the spec's primary scope:
- Route `/work` → `W` → `W1-create-thread`
- Route `/dashboard` → `D` → `D1-first-visit`
- Feature `checkout` → `C` → `C1-complete-purchase`

## Coverage Matrix

Every requirement must appear in at least one journey:

```markdown
| Requirement | Journeys |
|-------------|----------|
| R1: Thread creation | W1, W3 |
| R2: Session bootstrap | W1, W4 |
```

## Per-Journey Structure

```markdown
## W1-create-thread

> A developer opens the work page for the first time, creates a thread
> by sending a message, watches the session boot, and sees the agent
> respond with a workspace file.

**Covers:** R1 (thread creation), R2 (session bootstrap), R5 (workspace write)

### Phases

#### 01_landing (fast)
1. DO  navigate to /work
2. SEE welcome hero visible
3. SEE agent pill contains bot name

#### 02_first_send (fast)
1. DO  type message in composer
2. DO  click send
3. SEE URL changes to /work/<threadId>
4. CHECK thread created in database

#### 03_bootstrap (slow)
1. SEE session status shows "Connected"
2. SEE boot card disappears

#### 04_first_turn (moderate)
1. CHECK workspace file contains expected content
```

## Phase Guidelines

- **Numbered** — two-digit prefix (01-99), snake_case name
- **Timing words** — fast, moderate, slow, long. NEVER absolute numbers.
  Exact durations come from DISCOVERY, not journey planning.
- **DO/SEE/CHECK only** — no selectors, no mutations, no seed functions
- **Each phase tells one beat** of the user story

## Story Quality

Each journey must:
- Read like a real user experience, not a test procedure
- Have a clear beginning (entry point) and end (verified state)
- Cover specific requirements (listed in Covers: line)
- Flow naturally — a person would do these steps in this order

## Error-Path Journeys

For journeys that validate failure behavior, note the expected failure:

```markdown
## W5-invalid-provider

> A developer starts a session with a misconfigured provider.
> The system shows a clear error instead of hanging.

**Covers:** R8 (error handling)
**Tags:** @red
```

## Relationship to specctl

Journeys bridge specs and tests:
- **Input:** SPEC.md requirements (Gherkin scenarios)
- **Output:** JOURNEYS.md (natural user stories with DO/SEE/CHECK phases)
- **Next step:** convergence plan (`references/convergence-pattern.md`)
  then implementation following Agent-First methodology (`references/agent-first-e2e.md`)

If oh-my-claudecode (Claude Code) or oh-my-codex (Codex) is installed,
the spec-to-journeys consensus loop (Planner → Designer → Critic) can
be automated. Without orchestration plugins, follow the workflow
manually: draft journeys from requirements, review for UX realism,
verify coverage completeness, iterate until all requirements are covered.
