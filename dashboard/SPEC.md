---
spec: dashboard
charter: specctl
format: behavioral-spec
---

# specctl Dashboard

Rich HTML governance portal for visualizing spec state. A `specctl dashboard` subcommand that generates a Vite+React SPA — either as static HTML or a live dev server — showing charter health, spec heatmaps, dependency graphs, and detailed spec breakdowns. Designed for human consumption, not machines.

## Dashboard CLI

The `specctl dashboard` subcommand operates in two modes. By default it generates a self-contained HTML bundle to `.specs/dashboard/` that can be opened directly in a browser. With `--serve`, it starts a local HTTP server that serves the React SPA with a REST API backend, watching `.specs/` for changes.

The Go binary embeds the pre-built React app via `go:embed dashboard/dist/*`. No Node.js is required at runtime. The build step (`make dashboard`) compiles the React app, and `go build` embeds the result.

### Data Model

- **DashboardConfig**: port (int, default 3847), output_dir (string, default ".specs/dashboard/"), serve (bool)
- **Static mode**: reads all `.specs/` YAML, assembles DashboardData JSON, pre-fetches all API responses, writes index.html with embedded data
- **Serve mode**: starts net/http server, serves embedded SPA at `/`, REST API at `/api/*`, fsnotify watcher on `.specs/`

### Contracts

Static mode output:
```
.specs/dashboard/
  index.html        # SPA with embedded data
  assets/           # CSS, JS chunks
```

Serve mode:
```
$ specctl dashboard --serve
Serving dashboard at http://localhost:3847
Watching .specs/ for changes...
Press Ctrl+C to stop
```

### Invariants

- Static mode never starts a server — it writes files and exits
- Serve mode never writes to the filesystem — it only reads and serves
- The embedded SPA is identical in both modes; only the data source differs (embedded JSON vs REST API)
- No Node.js dependency at runtime — the React app is pre-built and embedded

## Requirement: Dashboard CLI subcommand

```gherkin requirement
@dashboard @manual
Feature: Dashboard CLI subcommand
```

### Scenarios

```gherkin scenario
Scenario: Static HTML generation
  Given a project with .specs/ directory containing charters and specs
  When the user runs specctl dashboard
  Then a self-contained HTML bundle is written to .specs/dashboard/
  And the output includes index.html with embedded CSS and JS
  And all spec data is pre-fetched and embedded as JSON
```

```gherkin scenario
Scenario: Live server mode
  Given a project with .specs/ directory
  When the user runs specctl dashboard --serve
  Then an HTTP server starts on localhost port 3847
  And the React SPA is served at the root path
  And REST API endpoints are available under /api/
  And the server watches .specs/ for file changes
```

## REST API

The Go server exposes REST endpoints that the React SPA consumes via React Query. Each endpoint returns JSON. All endpoints reuse the existing specctl application-layer services for reading spec state.

### Data Model

- **OverviewData**: total_charters (int), total_specs (int), total_requirements (int), verified_pct (float), open_deltas (int), charters (CharterSummary[])
- **CharterSummary**: name, title, spec_count, verified_pct, open_deltas, groups (GroupSummary[])
- **SpecDetail**: slug, charter, title, status, rev, checkpoint, scope[], deltas (Delta[]), requirements (Requirement[]), documents, changelog[]
- **GraphData**: nodes (GraphNode[]), edges (GraphEdge[])
- **GraphNode**: id (charter:slug), label, charter, status, verified_pct, group
- **GraphEdge**: source, target (depends_on relationship)
- **HeatmapData**: charters (HeatmapCharter[]) each containing specs with slug, verified_pct, status

### Contracts

GET /api/overview:
```json
{
  "total_charters": 2,
  "total_specs": 6,
  "total_requirements": 24,
  "verified_pct": 0.72,
  "open_deltas": 3,
  "charters": [
    { "name": "ui", "title": "UI", "spec_count": 4, "verified_pct": 0.75, "open_deltas": 2 }
  ]
}
```

GET /api/specs/:charter/:slug:
```json
{
  "slug": "work-thread",
  "charter": "ui",
  "title": "Work Thread",
  "status": "active",
  "rev": 3,
  "deltas": [
    { "id": "D-001", "area": "Session lifecycle", "intent": "add", "status": "closed" }
  ],
  "requirements": [
    { "id": "REQ-001", "title": "Thread creation", "verification": "verified", "tags": ["ui"], "gherkin": "..." }
  ]
}
```

### Invariants

- All endpoints return JSON with consistent structure
- CORS headers are set for localhost development
- Endpoints reuse application-layer logic — no duplicate data reading
- Serve mode returns fresh data on each request (no caching)

## Requirement: REST API endpoints

```gherkin requirement
@dashboard @manual
Feature: REST API endpoints
```

### Scenarios

```gherkin scenario
Scenario: Overview endpoint returns aggregate stats
  Given a project with 2 charters and 6 specs
  When GET /api/overview is called
  Then the response contains total charters, specs, requirements counts
  And verification percentage is calculated across all specs
```

```gherkin scenario
Scenario: Spec detail endpoint returns full breakdown
  Given a spec with deltas and requirements
  When GET /api/specs/ui/work-thread is called
  Then the response includes deltas with status and intent
  And requirements with Gherkin blocks and verification status
```

```gherkin scenario
Scenario: Graph endpoint returns dependency data
  When GET /api/graph is called
  Then the response includes nodes array with spec id and health
  And edges array with source and target from depends_on
```

## Vite React SPA

The frontend is a Vite+React single-page application in `dashboard/`. It uses Tailwind CSS with a dark-first design (#0A0A0A backgrounds), React Query for data fetching, and React Router for navigation between 4 views. Typography uses Inter for UI and JetBrains Mono for code/data.

### Data Model

- **Router**: 4 routes — / (overview), /heatmap, /graph, /specs/:charter/:slug (detail)
- **React Query**: queries per API endpoint with stale time and refetch config
- **Theme**: dark-only, CSS variables for colors, Tailwind config for utilities

### Invariants

- No Terrarium design tokens — standalone identity for open-source release
- Dark mode only (v1) — no light/dark toggle
- All data fetched via React Query — no prop drilling of API data
- Navigation between views uses client-side routing (no full page reloads)

## Requirement: SPA foundation and navigation

```gherkin requirement
@dashboard @ui @manual
Feature: SPA foundation and navigation
```

### Scenarios

```gherkin scenario
Scenario: SPA renders with dark theme
  Given the dashboard is loaded in a browser
  Then the background color is dark (#0A0A0A or similar)
  And text uses Inter font family
  And code blocks use JetBrains Mono font family
```

```gherkin scenario
Scenario: Navigation between views
  Given the dashboard is loaded
  When the user clicks a navigation tab
  Then the view transitions without a full page reload
  And the URL updates to reflect the current view
```

## Charter Overview View

The landing page shows all charters as glass-effect cards with aggregated health statistics. Each card displays the charter name, title, spec count, verification percentage, and open delta count. An overall governance health summary sits at the top. Cards are clickable to drill into a charter's specs.

### Data Model

- **OverviewData** consumed from GET /api/overview
- **CharterCard**: rendered per charter with name, title, spec_count, verified_pct (as progress bar), open_deltas (as badge)
- **GovernanceSummary**: total specs, total requirements, overall verified %, total open deltas

### Invariants

- All charters visible on landing page — no pagination for v1
- Health percentage shown as both number and visual indicator (progress bar or ring)
- Glass-effect cards with backdrop-filter blur on dark background

## Requirement: Charter Overview view

```gherkin requirement
@dashboard @ui @manual
Feature: Charter Overview view
```

### Scenarios

```gherkin scenario
Scenario: Overview displays all charters with health stats
  Given a project with 2 charters containing specs
  When the user opens the dashboard
  Then each charter is displayed as a card
  And each card shows spec count and verification percentage
  And each card shows open delta count
  And an overall governance summary is displayed at the top
```

```gherkin scenario
Scenario: Charter card drill-down
  Given the overview is displayed
  When the user clicks a charter card
  Then the view shows that charter's specs listed with their health status
```

## Health Heatmap View

A color-coded matrix visualization of all specs organized by charter. Colors range from green (fully verified) through yellow (partially verified/stale) to red (unverified/open gaps). Each cell shows the spec slug and verification percentage. Cells are clickable, navigating to the spec detail page.

### Data Model

- **HeatmapData** consumed from GET /api/heatmap
- **HeatmapCell**: spec slug, verified_pct, status, charter (for grouping)
- **ColorScale**: 0% = red (#EF4444), 50% = yellow (#EAB308), 100% = green (#22C55E)

### Invariants

- Specs are grouped by charter in the matrix layout
- Color interpolation is continuous (not stepped)
- Empty charters (no specs) show a placeholder cell

## Requirement: Health Heatmap view

```gherkin requirement
@dashboard @ui @manual
Feature: Health Heatmap view
```

### Scenarios

```gherkin scenario
Scenario: Heatmap renders color-coded spec matrix
  Given a project with specs at varying verification levels
  When the user navigates to the heatmap view
  Then each spec is shown as a colored cell
  And fully verified specs are green
  And unverified specs are red
  And partially verified specs show interpolated color
```

```gherkin scenario
Scenario: Heatmap cell navigation
  Given the heatmap is displayed
  When the user clicks a spec cell
  Then the view navigates to that spec's detail page
```

## Dependency Graph View

A D3.js force-directed graph showing spec dependency relationships. Nodes represent specs, colored by health status (green/yellow/red). Edges represent `depends_on` relationships with directional arrows. The graph supports physics simulation with drag, zoom, and pan. Nodes show tooltips on hover and navigate to spec detail on click. Charter grouping is visible as clusters or background regions.

### Data Model

- **GraphData** consumed from GET /api/graph
- **D3 ForceSimulation**: forceLink (edges), forceManyBody (repulsion), forceCenter, forceCollide
- **Node rendering**: circle with radius proportional to requirement count, fill color from health scale
- **Edge rendering**: line with arrow marker, stroke from source node color

### Invariants

- Graph uses D3 force simulation with configurable parameters
- Nodes are draggable — dragged nodes become fixed, double-click releases
- Zoom and pan via mouse wheel and drag on background
- Tooltip shows spec title, status, verification %, open deltas on hover

## Requirement: Dependency Graph view

```gherkin requirement
@dashboard @ui @manual
Feature: Dependency Graph view
```

### Scenarios

```gherkin scenario
Scenario: Graph renders spec dependencies
  Given a project with specs that have depends_on relationships
  When the user navigates to the graph view
  Then specs are displayed as nodes colored by health status
  And dependencies are shown as directional edges between nodes
  And the graph uses physics simulation for layout
```

```gherkin scenario
Scenario: Graph interaction
  Given the dependency graph is displayed
  When the user drags a node
  Then the node position updates and physics re-simulates
  When the user hovers over a node
  Then a tooltip shows spec title, status, and verification percentage
  When the user clicks a node
  Then the view navigates to that spec's detail page
```

## Spec Detail View

The deep-dive view showing everything about a single spec. Displays title, status badge, revision, checkpoint, and governed scope. Lists all deltas with status badges (open/in-progress/closed/deferred) and intent icons. Shows requirements with rendered Gherkin blocks, verification status (verified/unverified/stale), lifecycle indicators, and test file references. Includes document links and changelog/revision history.

### Data Model

- **SpecDetail** consumed from GET /api/specs/:charter/:slug
- **DeltaCard**: id, area, intent (icon), status (badge), current, target, notes
- **RequirementCard**: id, title, gherkin (syntax-highlighted), verification (icon), lifecycle, test_files[]
- **ChangelogEntry**: rev, checkpoint, summary, date

### Invariants

- Gherkin blocks are syntax-highlighted with Given/When/Then keywords colored
- Verification status uses consistent iconography (checkmark/cross/warning)
- Delta intent displayed as icon (+ for add, ~ for change, - for remove, wrench for repair)
- Document links open in new tab

## Requirement: Spec Detail view

```gherkin requirement
@dashboard @ui @manual
Feature: Spec Detail view
```

### Scenarios

```gherkin scenario
Scenario: Detail page shows full spec breakdown
  Given a spec with deltas and requirements
  When the user navigates to the spec detail page
  Then the spec title, status, revision, and scope are displayed
  And all deltas are listed with status badges and intent icons
  And all requirements are listed with rendered Gherkin blocks
  And verification status is shown per requirement
```

```gherkin scenario
Scenario: Requirement verification display
  Given a requirement with verification status verified and test files
  When the spec detail page is rendered
  Then the requirement shows a verified checkmark
  And test file paths are displayed as links
```
