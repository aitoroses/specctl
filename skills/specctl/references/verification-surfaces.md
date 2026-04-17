# Verification Surfaces

The Agent-First methodology is identical across all surfaces. What
changes is the **client** the agent uses for DISCOVERY and the
**proof system** for verification.

## Universal MCP API

Every test surface should expose four operation categories via MCP.
Tool names below are illustrative — adapt the naming convention to
your project. The four categories are the invariant; names are not.

| Operation | Purpose | Examples |
|-----------|---------|---------|
| **Boot** | Start the system under test | `e2e_boot`, `api_start`, `mcp_serve` |
| **Seed** | Create precondition state | `e2e_reseed`, `db_seed`, `fixture_load` |
| **Probe** | Read system state | `workspace_read`, `db_query`, `api_call` |
| **Verify** | Assert expected outcomes | `workspace_exec`, `health_check` |

The agent uses these operations during DISCOVERY (probing the real
system) and WRITE (implementing test assertions).

## UI Surface

**Client:** headed browser via agent-browser or similar automation tool.

**DISCOVERY:** navigate pages, observe visual states, mine data-testid
selectors, capture screenshots, measure timings.

**Proof types:**
- `url-match` — URL matches expected pattern
- `element-visible` — data-testid element present and visible
- `element-text` — element contains expected text
- `screenshot-diff` — visual regression comparison

**Test infrastructure:**
- Playwright (or Cypress, Puppeteer) as test runner
- Phase-based test structure with budget enforcement
- Auto screenshots on phase enter/exit
- Domain helpers for page interactions (selectors, flows)

**MCP tools for UI E2E:**
```
e2e_boot(test)          — start infra + seed + browser
e2e_prompt()            — agent-mode system prompt with browser instructions
e2e_refs()              — seeded entity reference map
e2e_reseed([test])      — reset state without restart
e2e_inspect(query)      — read Convex/DB state
e2e_workspace_read(id, path) — read sandbox files
e2e_run(test)           — execute via Playwright
e2e_teardown()          — cleanup
```

## Backend API Surface

**Client:** mcporter (MCP-over-HTTP transport) or direct HTTP client
wrapped as MCP tools.

**DISCOVERY:** call endpoints, observe response shapes, mine schemas,
verify status codes and error formats, measure latencies.

**Proof types:**
- `response-shape` — JSON response matches expected schema
- `status-code` — HTTP status matches expected value
- `header-present` — response includes required headers
- `db-state` — database row matches expected values after API call

**Test infrastructure:**
- Contract test framework (supertest, httpx, etc.)
- Schema validation (JSON Schema, Zod, pydantic)
- Database fixtures for precondition state
- Domain helpers for API call sequences

**MCP tools for API E2E:**
```
api_boot()              — start the API server
api_seed(fixture)       — load database fixtures
api_call(method, path, body) — make HTTP request, return response
api_db_query(table, filter)  — read database state
api_health()            — check server readiness
api_teardown()          — cleanup
```

**Building with mcporter:** mcporter wraps any HTTP API as an MCP
server. Define endpoints as MCP tools, and the agent interacts with
your API through standard MCP calls — like Postman but agent-first.
Implement the MCP server using the standard SDK for your language
(Python: `mcp`, TypeScript: `@modelcontextprotocol/sdk`).

## MCP Tool Surface

**Client:** direct MCP tool calls. The system under test IS an MCP
server, so the "eyes" are the tools themselves.

**DISCOVERY:** call each tool with various inputs, observe outputs,
verify error handling, probe capabilities.

**Proof types:**
- `tool-output` — tool returns expected result shape
- `tool-error` — tool returns expected error code
- `side-effect` — tool causes expected state change (verified via probe)
- `idempotency` — calling tool twice produces same result

**Test infrastructure:**
- MCP client harness that connects to the server
- Fixture system for server state setup
- Domain helpers for common tool call sequences

**MCP tools for MCP E2E:**
```
mcp_boot(config)        — start the MCP server under test
mcp_call(tool, input)   — call a tool and return the result
mcp_list_tools()        — enumerate available tools
mcp_state(query)        — read server internal state
mcp_teardown()          — cleanup
```

## Background Job Surface

**Client:** MCP tools wrapping job triggers + database/queue probes.

**DISCOVERY:** trigger jobs, observe queue state, verify database
mutations, check side effects (emails sent, files written).

**Proof types:**
- `job-completed` — job reached terminal state
- `queue-drained` — all items processed
- `side-effect-file` — expected file written with expected content
- `db-mutation` — database row updated as expected

**MCP tools for Background Job E2E:**
```
job_boot()              — start the job system
job_seed(scenario)      — create precondition data
job_trigger(name, input) — enqueue or trigger a job
job_status(id)          — check job state
job_db_query(table)     — read database after job completes
job_teardown()          — cleanup
```

## Cross-Surface Principles

1. **MCP is the universal adapter** — every surface is testable through MCP tools
2. **Boot/Seed/Probe/Verify** — the four operation categories compose any test surface
3. **Domain owns verification** — proof factories live in the domain layer, not the framework
4. **Framework owns lifecycle** — phase budgets, polling, reporting are framework concerns
5. **DISCOVERY is always interactive** — the agent probes the real system, not mocks
6. **Build the MCP server using standard SDKs** — Python (`mcp`), TypeScript (`@modelcontextprotocol/sdk`), Go (`go-sdk`)
