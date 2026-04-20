# Building an Agent-First Verification Harness

> How do I, as an engineer, give my agent the tools to reproducibly
> close the loop on ANYTHING that matters to me and the project?

## What is an Agent Harness?

**Agent = Model + Harness.** The harness is everything EXCEPT the
model: the tools, the feedback loops, the specifications, the
verification systems, and the infrastructure that makes an agent
reliable instead of unpredictable.

In the context of specctl and Agent-First TDD, the harness has
two layers:

**Guides (feedforward)** — steer the agent BEFORE it acts:
- Behavioral specs (SPEC.md) define what the system should do
- specctl governance tracks what's verified and what drifted
- Format templates teach the agent how to write specs
- Journey specs describe what to test and in what order

**Sensors (feedback)** — verify AFTER the agent acts:
- MCP tools that boot, seed, probe, and verify the system
- Phase-budgeted tests that fail fast on regressions
- Proof system that asserts domain-specific outcomes
- Observability artifacts (screenshots, timelines, state dumps)

The steering loop: the agent acts, the harness verifies, failures
update the spec, the agent acts again. Any persistent loop tool
(oh-my-claudecode's ralph, Codex loops, or any retry-until-green
agent) can drive this cycle incrementally.

**specctl's role:** it IS the harness's governance layer. It defines
what behaviors matter (specs), tracks their verification state
(requirements), detects when reality drifts from spec (diff), and
guides the agent back to alignment (next actions). The E2E
framework you build is the harness's sensor layer.

## The Three Questions

Every verification harness answers three questions:

1. **Can my agent define and boot an environment with a given state?**
   — Seeding, isolation, reproducibility. The agent creates any
   precondition on demand and tears it down cleanly.

2. **What tools does the agent have to inspect and drive the system?**
   — Probe (read state), Drive (navigate/call/trigger), Capture
   (screenshots, responses, artifacts). Eyes and hands.

3. **How does the agent encode its learnings in code?**
   — The prompt artifact, phase-by-phase test construction, proof
   system, always-green discipline. Discovery becomes durable tests.

The engineer decides what to build. The agent builds AND uses it.
specctl governs the whole thing — including the harness itself.

## The Harness as MCP Surface

Your system has behaviors that matter. Today, YOU verify them by
poking around. The goal: give your agent the same ability, reproducibly.

**Build an MCP surface that turns your system into something an
agent can boot, seed, probe, and verify.**

Every verification surface — UI, API, MCP, background job — follows
this shape. The four capabilities are universal:

| Capability | What it does | Why the agent needs it |
|------------|-------------|----------------------|
| **Boot** | Start the system under test | Agent needs a running target |
| **Seed** | Create precondition state | Reproducible starting point |
| **Probe** | Read system state | Agent needs "eyes" to observe |
| **Verify** | Assert expected outcomes | Close the loop |

Without Boot, the agent can't start. Without Seed, tests are flaky.
Without Probe, the agent is blind. Without Verify, assertions are manual.

## How Persistent Loops Use the Harness

The harness is infrastructure. The loop is execution. Any persistent
agent loop can consume the harness:

```
Loop iteration N:
  1. Read specctl context → what's drifted? what is actionable now vs inactive audit debt?
  2. Pick the next delta / requirement to work on
  3. Boot the system via MCP → environment ready
  4. Seed the scenario → preconditions set
  5. Discover / implement / test → encode learnings as test code
  6. Run the test → green or red?
  7. If green: specctl req verify → close delta → rev bump
  8. If red: fix → retry from step 5
  9. Loop back to step 1
```

oh-my-claudecode's ralph does this. Codex's persistent sessions do
this. Any tool that can "keep working until done" benefits from a
harness that tells it WHAT to work on (specctl) and HOW to verify
it worked (MCP sensors).

The harness makes the loop **convergent** — each iteration reduces
the gap between spec and reality. Without governance, the loop may
churn. Without sensors, the loop can't verify. Both layers together
make persistent agents productive.

## Deciding What Matters

Before building anything, decide what you need verified:

### The Decision Framework

1. **List your behavioral surfaces** — what does your system expose?
   - UI routes users interact with
   - API endpoints clients call
   - MCP tools agents invoke
   - Background jobs that process work
   - Webhooks that receive events

2. **For each surface, ask: what would break trust if it regressed?**
   - Thread creation fails silently → users lose work
   - Payment API returns wrong amount → financial loss
   - MCP tool returns stale data → agent makes wrong decisions
   - Job fails without notification → silent degradation

3. **Prioritize by blast radius** — start with the surface where
   a regression would hurt most. You don't need to verify everything
   at once. One governed, verified surface is worth more than five
   ungoverned ones.

4. **Map to specctl** — each critical surface becomes a spec.
   Each behavior becomes a requirement. Each requirement gets verified.

## Speccing the Enablement Layer

The framework IS a product. Spec it like one — with specctl.

### What to Spec

Create specs for the infrastructure the agent will build and use:

**1. MCP Server Surface**
- What tools does the server expose?
- What inputs does each tool accept?
- What outputs does each tool return?
- What errors can occur?
- What state transitions do tools cause?

Spec this as a behavioral surface in a SPEC.md. Each tool is a
requirement: "When the agent calls `e2e_boot` with test name X,
the system starts, seeds data, and returns a ready signal."

**2. Seed Client**
- What entities can be seeded? (users, projects, API keys, configs)
- What are the entity relationships? (project requires user)
- What does cleanup look like? (scoped per test, automatic)
- What's the seed API shape? (`s.devUser()`, `s.project(name, opts)`)

The seed client is the contract boundary between framework and
domain. Spec it tightly — when seed methods change, proof factories
and tests update.

**3. Phase Runner**
- How are phases defined? (name, label, budget)
- How are budgets enforced? (Promise.race, timeout)
- What happens on phase enter/exit? (screenshots, timing)
- What happens on failure? (exit screenshot, diagnostic artifacts)

**4. Proof System**
- How are proofs declared? (inline in phase config)
- Who owns verify logic? (domain — proof factories)
- Who owns polling lifecycle? (framework — phase runner)
- What descriptors exist for diagnostics? (kind, path, expected)

**5. Observability**
- What artifacts are produced per test? (screenshots, timelines, state dumps)
- How are failures classified? (product vs framework vs infra)
- How are attempts scoped? (primary vs retry evidence separated)

### The Ouroboros

Use specctl to govern these infrastructure specs alongside product
specs. The governance tool governs the test infrastructure that
verifies the product. In practice:

```
specctl charter create testing
specctl spec create testing:e2e-framework --doc e2e/SPEC.md --scope e2e/
specctl spec create testing:mcp-server --doc e2e/mcp/SPEC.md --scope e2e/mcp/
```

Now when you change the framework, specctl detects drift. When you
add a new MCP tool, you add a requirement. The framework is held to
the same standard as the product.

## Building the Enablement Layer

The agent builds most of this. Your role: spec it, review it,
guide architectural decisions. The agent follows the governance loop:

### Layered Architecture

```
core/           Pure types. No I/O, no framework imports.
                Phase, Proof, ProofContext, JourneyConfig, SeedClientLike

infra/          Process orchestration. Shared by all runners.
                Bootstrap, teardown, process management, runtime inspector

runners/        Thin adapters per execution mode.
  playwright/   Browser-based: fixtures, phase runner, debug collector
  mcp/          Agent-driven: MCP server, tools, registry

domains/        Project-specific. NOT portable.
                Selectors, page interactions, proof factories

tests/          Journey specs. Clean, semantic, reads like the spec.
```

**Core owns contracts.** `Phase`, `Proof`, `ProofContext`,
`JourneyConfig` are defined in core as pure types. Everything else
implements against these contracts.

**Infra owns processes.** Boot a database, start a server, spawn
a tunnel, manage health checks. Shared across runners.

**Runners own execution.** Playwright runner wraps phases in browser
context with screenshots. MCP runner exposes the same phases as
tools the agent calls interactively.

**Domains own verification.** Proof factories define HOW to verify.
The framework decides WHEN and HOW LONG to try. Selectors, page
interactions, and domain-specific assertions live here.

**Tests are semantic.** Reading a journey test should feel like
reading the spec. No framework mechanics, no raw selectors, no
timeout numbers in the test body.

### MCP Server Design

The MCP server is the bridge between agent and infrastructure.
Design tool groups around the four capabilities:

**Lifecycle tools (Boot/Teardown):**
- `boot(test_name)` → start infra, seed data, return ready signal
- `teardown()` → kill all processes, cleanup
- `health()` → check all subsystems
- `status()` → current state (booted, seeded, running, idle)

**Seed tools:**
- `reseed([test_name])` → clear + re-seed without restarting infra
- `refs()` → return seeded entity reference map (IDs, names, keys)

**Probe tools (the "eyes"):**
- `inspect(entity, filter)` → read database/state
- `query(table, filter)` → structured data query
- `workspace_read(id, path)` → read files in sandbox/workspace
- `workspace_tree(id)` → list workspace files
- `workspace_exec(id, cmd)` → execute command in workspace

**Runner tools:**
- `run(test_name)` → execute journey via test runner (Playwright, etc.)
- `list()` → enumerate registered journeys
- `prompt()` → return agent-mode system prompt (browser instructions,
  URLs, available tools, scenario) — this is what gives the agent
  the full context to navigate the real system

**Debug tools (optional but valuable):**
- `session_start/state/stop/logs` — runtime session management
- `thread` — composite view of a thread's full state

### Seed Client Design

The seed client is the most critical design decision. Get it right
and tests are reproducible. Get it wrong and tests are flaky.

**Principles:**
- **Semantic, not mechanical.** `s.devUser()` not `s.insertRow('users', {...})`
- **Relationship-aware.** `s.project('acme', { user })` — project knows it needs a user
- **Scoped cleanup.** Each test's seed data is isolated. Cleanup is automatic.
- **Contract boundary.** When the seed client changes, proof factories update.
  The framework never calls the seed client directly — only `proof.verify()` does.

**Example shape:**
```
seed: async (s) => {
  const user = await s.devUser()
  await s.agent('Bot', { user, tools: ['read_file', 'bash'], model: 'fast' })
  await s.project('acme', { user, repo: 'org/repo' })
  await s.apiKeys(user, { openRouter: true })
}
```

Entity order matters: user before agent, user before project.
The seed client enforces this through its API shape — you can't
create a project without passing a user.

### Phase + Proof Design

**Phases are budgeted containers:**
```
phases: {
  '01_landing': {
    label: 'Navigate and verify entry',
    budget: { expected: '2s', max: '5s' },
  },
  '02_action': {
    label: 'Primary user action',
    budget: { expected: '3s', max: '10s' },
    proofs: {
      'state:created': {
        kind: 'db-state',
        verify: async (ctx) => { /* domain logic */ },
      }
    }
  }
}
```

- `expected` — from DISCOVERY measurements. Informational.
- `max` — failfast timeout. Phase dies if exceeded.
- `proofs` — inline in the phase that verifies them. Domain-owned.

**Proof factories are domain functions:**
```
function fileContains(path, expected) {
  return {
    kind: 'workspace-file',
    path, expected,  // descriptors for diagnostics
    verify: async (ctx) => {
      const content = await ctx.runtime.workspaceRead(ctx.vars.id, path)
      return content.trim() === expected
    }
  }
}
```

The framework polls `verify()` within the phase budget. It knows
nothing about files, workspaces, or content — it just calls verify
and reports using the descriptor fields.

## Using the Enablement Layer

Once built, the agent uses the framework through the Agent-First
methodology (see `agent-first-e2e.md`):

1. **GATE** — check prerequisites exist (instrumentation, endpoints, tools)
2. **SEED** — design the environment via seed tools
3. **DISCOVERY** — boot, navigate/call the real system, mine data:
   - Call `boot(test)` → system starts, data seeded
   - Call `prompt()` → get full context for navigating the system
   - Probe the system with spec in hand: navigate UI / call API / invoke MCP
   - Mine: selectors, response shapes, timings, state transitions
   - Capture evidence: screenshots, response logs
   - Refine the prompt until replay works end-to-end
4. **WRITE** — build the test phase by phase, always green:
   - Start with seed + empty body → green
   - Add phase 1 → run → green
   - Add phase 2 → run → green
   - Continue until test matches prompt

**The prompt is the artifact.** It captures everything learned during
DISCOVERY. Three verb prefixes:
- **DO** — action (navigate, click, type, call)
- **SEE** — observable assertion (visible, gone, contains)
- **CHECK** — data assertion behind the surface (file content, DB row)

## The Journey Test Artifact

The end result of the pipeline is a **journey test** — a self-contained
artifact that encodes everything the agent learned during DISCOVERY.

A journey test has four parts:

### 1. Contract (phases + proofs)

Declares the test structure: what phases exist, their budgets, and
what proofs verify each phase.

```
journeyTest('W1 user creates thread and sees agent respond', {
  tags: ['@journey', '@threads'],

  phases: {
    '01_landing': {
      label: 'Navigate and verify entry',
      budget: { expected: '2s', max: '5s' },
    },
    '02_first_send': {
      label: 'Send message and verify thread creation',
      budget: { expected: '3s', max: '10s' },
      proofs: {
        'url:thread-created': {
          kind: 'url-match',
          verify: async (ctx) => /\/work\/[^/]+$/.test(ctx.page?.url() ?? ''),
        },
      },
    },
    '03_bootstrap': {
      label: 'Session boot and connection',
      budget: { expected: '45s', max: '90s' },
    },
    '04_first_turn': {
      label: 'Agent writes workspace marker',
      budget: { expected: '15s', max: '60s' },
      proofs: {
        'workspace:marker-created': {
          kind: 'workspace-file',
          path: 'marker.txt',
          expected: 'hello-world',
          verify: async (ctx) => {
            const content = await ctx.runtime.workspaceRead(
              ctx.vars.threadId, 'marker.txt')
            return content.trim() === 'hello-world'
          },
        },
      },
    },
  },
```

### 2. Seed (reproducible environment)

Creates the exact precondition state for this journey. Semantic,
relationship-aware, scoped cleanup.

```
  seed: async (s) => {
    const user = await s.devUser()
    await s.agent('Test Bot', {
      user,
      tools: ['read_file', 'write_file', 'bash'],
      model: 'fast-model',
    })
    await s.project('acme-app', { user, repo: 'org/repo' })
    await s.apiKeys(user, { openRouter: true })
  },
```

### 3. Prompt (the discovery artifact)

The prompt IS the spec for this journey, written in phase-first
DO/SEE/CHECK vocabulary. It captures everything the agent learned
during DISCOVERY: selectors, timings, state transitions, edge cases.
An agent reading ONLY the prompt can write the test code.

```
  prompt: `
    ## 01_landing (~2s)
    1. DO  navigate /work
    2. SEE [work-welcome-hero] visible
       SEE [work-page-header] visible
       SEE [work-agent-pill] contains "Test Bot"
    3. DO  select project "acme-app" via composer picker

    ## 02_first_send (~3s)
    1. DO  type message in [work-chat-input]
       DO  click [work-chat-send]
    2. SEE URL → /work/<threadId>
       SEE [work-thread-page] visible
       SEE [work-boot-card][data-state="running"] visible

    ## 03_bootstrap (~30-90s)
    1. SEE [work-session-status] contains "Connected"
       SEE [work-boot-card] gone

    ## 04_first_turn (~10-15s)
    1. CHECK workspace_read(threadId, "marker.txt") = "hello-world"
  `,
```

The prompt sections map 1:1 to phases. If they diverge, the prompt
is stale. When modifying a test, update both.

### 4. Test Body (semantic, phase-by-phase)

The actual test code reads like the prompt. Domain helpers hide
Playwright mechanics. Each action lives inside a budgeted phase.

```
  async ({ page, expect, phase }) => {
    let threadId

    await phase('01_landing', async ({ screenshot }) => {
      await page.goto('/work')
      await expect(page.locator(SEL.welcomeHero)).toBeVisible()
      await expect(page.locator(SEL.agentPill)).toContainText('Test Bot')
      await screenshot('work-landing')
    })

    await phase('02_first_send', async ({ screenshot, proof }) => {
      await sendMessage(page, 'Write marker.txt with exactly hello-world.')
      threadId = page.url().split('/work/')[1] ?? ''
      await screenshot('thread-created')
      await proof('url:thread-created')
    })

    await phase('03_bootstrap', async ({ screenshot }) => {
      await expect(page.locator(SEL.sessionStatus))
        .toContainText('Connected', { timeout: 90_000 })
      await screenshot('connected')
    })

    await phase('04_first_turn', async ({ proof }) => {
      await proof('workspace:marker-created', { threadId })
    })
  }
)
```

### What Makes This Shape Work

- **Contract declares intent** — phases, budgets, proofs are pure data
- **Seed ensures reproducibility** — same state every run, scoped cleanup
- **Prompt captures discovery** — the agent's learnings become durable
- **Test body is semantic** — reads like the spec, not like framework API calls
- **Proofs close the loop** — domain-owned verify functions, framework-owned polling
- **Always green** — built phase by phase, never break the previous phase

This artifact shape works for any surface. For a backend API test,
replace `page.goto()` with `api.call()`, replace `screenshot()` with
`captureResponse()`, and replace `page.locator()` with response shape
assertions. The four parts (contract, seed, prompt, body) remain the same.

## The Full Loop

```
Engineer decides what matters
  → specs it with specctl (behavioral surfaces + requirements)
  → agent builds the MCP enablement layer (also specctl-governed)
  → agent uses DISCOVERY to probe the real system
  → agent writes tests phase-by-phase, always green
  → specctl tracks verification (req verify + test files)
  → CI runs tests, specctl detects drift
  → loop: drift detected → delta → fix → verify → rev bump
```

Every piece is governed. Every piece is verifiable. The agent has
eyes (Probe), hands (Seed/Boot), and a brain (the spec + prompt).
The engineer decides what matters. The system proves it works.
