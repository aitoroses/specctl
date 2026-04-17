# specctl Dashboard — Design System Specification

## Aesthetic Direction: "Observatory"

Your specifications as a living constellation. Dark void backgrounds, luminous data points,
precise instrument-grade readouts. The dashboard should feel like mission control for your
codebase governance — every element communicates health, every interaction reveals depth.

**Reference products:** Vercel Analytics (data density), Linear (polish + motion), Raycast (dark precision), Stripe Dashboard (typographic hierarchy)

**The ONE memorable thing:** The dependency graph feels alive — nodes glow and pulse based on health, edges trace luminous paths between specs. It's a star map of your specification universe.

---

## Color System

### Core Palette

```css
:root {
  /* Void — the foundation */
  --void-deep:    #050505;   /* Deepest background (graph canvas) */
  --void:         #0A0A0A;   /* Primary background */
  --void-raised:  #111111;   /* Raised surfaces (cards, panels) */
  --void-overlay: #161616;   /* Overlay backgrounds (modals, dropdowns) */

  /* Surface — subtle differentiation */
  --surface-1:    #1A1A1A;   /* Card backgrounds */
  --surface-2:    #222222;   /* Hover states, active tabs */
  --surface-3:    #2A2A2A;   /* Input backgrounds, code blocks */

  /* Border — whisper-thin structure */
  --border-subtle:  rgba(255, 255, 255, 0.06);  /* Default borders */
  --border-default: rgba(255, 255, 255, 0.10);  /* Visible borders */
  --border-strong:  rgba(255, 255, 255, 0.15);  /* Emphasized borders */

  /* Text — four-level hierarchy */
  --text-primary:    #EDEDED;  /* Headings, primary data */
  --text-secondary:  #A0A0A0;  /* Body text, descriptions */
  --text-tertiary:   #666666;  /* Labels, captions, timestamps */
  --text-quaternary: #444444;  /* Disabled, placeholder */

  /* Accent — Emerald (governance = health = green family) */
  --accent:          #10B981;  /* Primary accent — emerald-500 */
  --accent-light:    #34D399;  /* Hover, focus rings */
  --accent-dim:      #059669;  /* Pressed, active states */
  --accent-subtle:   rgba(16, 185, 129, 0.10);  /* Accent backgrounds */
  --accent-glow:     rgba(16, 185, 129, 0.25);  /* Glow effects */

  /* Health Scale — the governance spectrum */
  --health-verified:  #22C55E;  /* 100% verified — green */
  --health-good:      #4ADE80;  /* 75%+ — light green */
  --health-partial:   #EAB308;  /* 50% — amber */
  --health-warning:   #F97316;  /* 25% — orange */
  --health-critical:  #EF4444;  /* 0% — red */

  /* Health glows — for graph nodes and badges */
  --glow-verified:  0 0 12px rgba(34, 197, 94, 0.4);
  --glow-partial:   0 0 12px rgba(234, 179, 8, 0.4);
  --glow-critical:  0 0 12px rgba(239, 68, 68, 0.4);

  /* Status — delta lifecycle */
  --status-open:        #3B82F6;  /* Blue — open */
  --status-in-progress: #8B5CF6;  /* Violet — in progress */
  --status-closed:      #22C55E;  /* Green — closed */
  --status-deferred:    #6B7280;  /* Gray — deferred */

  /* Intent — delta intent icons */
  --intent-add:    #22C55E;  /* Green + */
  --intent-change: #EAB308;  /* Amber ~ */
  --intent-remove: #EF4444;  /* Red - */
  --intent-repair: #F97316;  /* Orange wrench */
}
```

### Health Color Interpolation (CSS)

```css
/* Use for heatmap cells and progress indicators */
.health-gradient {
  /* 3-stop: red → amber → green */
  background: linear-gradient(
    90deg,
    var(--health-critical) 0%,
    var(--health-partial) 50%,
    var(--health-verified) 100%
  );
}
```

### Health Color Function (TypeScript)

```typescript
export function healthColor(pct: number): string {
  if (pct >= 0.75) return 'var(--health-verified)';
  if (pct >= 0.50) return 'var(--health-partial)';
  if (pct >= 0.25) return 'var(--health-warning)';
  return 'var(--health-critical)';
}

// For D3 — continuous interpolation
import { interpolateRgb } from 'd3-interpolate';
const healthScale = (pct: number) => {
  if (pct <= 0.5) {
    return interpolateRgb('#EF4444', '#EAB308')(pct * 2);
  }
  return interpolateRgb('#EAB308', '#22C55E')((pct - 0.5) * 2);
};
```

---

## Typography

### Font Stack

```css
:root {
  /* Display — for the dashboard title and section headers */
  --font-display: 'Geist', 'Inter', system-ui, sans-serif;

  /* Body — for all UI text */
  --font-body: 'Inter', system-ui, -apple-system, sans-serif;

  /* Mono — for data, Gherkin, code, IDs */
  --font-mono: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace;
}
```

> **Note:** Geist (Vercel's typeface) is used for display headings to echo the Vercel aesthetic.
> Inter for body. JetBrains Mono for code. If Geist is unavailable, Inter handles both roles.
> Load via Google Fonts CDN: `Inter:wght@400;500;600;700` + `JetBrains+Mono:wght@400;500`

### Type Scale

| Token | Size | Weight | Line Height | Letter Spacing | Usage |
|-------|------|--------|-------------|----------------|-------|
| `display-xl` | 36px / 2.25rem | 700 | 1.1 | -0.025em | Dashboard title |
| `display` | 28px / 1.75rem | 600 | 1.2 | -0.02em | View titles |
| `heading` | 20px / 1.25rem | 600 | 1.3 | -0.015em | Card titles, section headers |
| `subheading` | 16px / 1rem | 500 | 1.4 | -0.01em | Sub-sections |
| `body` | 14px / 0.875rem | 400 | 1.5 | 0 | Default body text |
| `body-sm` | 13px / 0.8125rem | 400 | 1.5 | 0 | Secondary text |
| `caption` | 12px / 0.75rem | 500 | 1.4 | 0.02em | Labels, timestamps, badges |
| `mono` | 13px / 0.8125rem | 400 | 1.6 | 0 | Code, IDs, Gherkin |
| `mono-sm` | 11px / 0.6875rem | 400 | 1.5 | 0.01em | Tiny mono (REQ-001, D-001) |
| `stat` | 48px / 3rem | 700 | 1 | -0.03em | Big numbers in governance summary |
| `stat-label` | 11px / 0.6875rem | 500 | 1.4 | 0.08em | ALL CAPS labels under stats |

### Tailwind Config

```typescript
// tailwind.config.ts
export default {
  darkMode: 'class',
  theme: {
    extend: {
      fontFamily: {
        display: ['Geist', 'Inter', 'system-ui', 'sans-serif'],
        body: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
      },
      fontSize: {
        'display-xl': ['2.25rem', { lineHeight: '1.1', letterSpacing: '-0.025em', fontWeight: '700' }],
        'display':    ['1.75rem', { lineHeight: '1.2', letterSpacing: '-0.02em', fontWeight: '600' }],
        'heading':    ['1.25rem', { lineHeight: '1.3', letterSpacing: '-0.015em', fontWeight: '600' }],
        'stat':       ['3rem',    { lineHeight: '1', letterSpacing: '-0.03em', fontWeight: '700' }],
      },
    },
  },
};
```

---

## Spacing Scale

8px base grid. Everything aligns to 8px increments.

| Token | Value | Usage |
|-------|-------|-------|
| `space-0.5` | 4px | Inner padding for badges, tight gaps |
| `space-1` | 8px | Inline spacing, icon gaps |
| `space-2` | 16px | Component internal padding |
| `space-3` | 24px | Card padding, section gaps |
| `space-4` | 32px | Between cards, between sections |
| `space-6` | 48px | Major section separators |
| `space-8` | 64px | Page margins, view padding |

---

## Component Patterns

### Glass Card

The signature component. Frosted glass over the void.

```css
.glass-card {
  background: rgba(255, 255, 255, 0.03);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 12px;
  transition: all 0.2s ease;
}

.glass-card:hover {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.10);
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.05);
}

/* Accent variant — for highlighted cards */
.glass-card-accent {
  background: rgba(16, 185, 129, 0.04);
  border-color: rgba(16, 185, 129, 0.12);
}

.glass-card-accent:hover {
  border-color: rgba(16, 185, 129, 0.20);
  box-shadow: 0 0 20px rgba(16, 185, 129, 0.08);
}
```

```
Tailwind equivalent:
bg-white/[0.03] backdrop-blur-xl border border-white/[0.06] rounded-xl
hover:bg-white/[0.05] hover:border-white/10
transition-all duration-200
```

### Status Badge

```
┌─────────────┐
│ ● Verified  │  → green dot + text, bg-green-500/10 text-green-400 rounded-md px-2 py-0.5
└─────────────┘
┌──────────────┐
│ ● Unverified │  → red dot + text, bg-red-500/10 text-red-400
└──────────────┘
┌─────────┐
│ ● Stale │  → amber dot + text, bg-amber-500/10 text-amber-400
└─────────┘
```

Badge anatomy:
- 6px color dot (rounded-full)
- 4px gap
- Caption text (12px, 500 weight)
- Padding: 4px 8px
- Border-radius: 6px
- Background: status color at 10% opacity

### Delta Status Badge

Same pattern, but with delta lifecycle colors:
- Open → blue (`bg-blue-500/10 text-blue-400`)
- In Progress → violet (`bg-violet-500/10 text-violet-400`)
- Closed → green (`bg-green-500/10 text-green-400`)
- Deferred → gray (`bg-gray-500/10 text-gray-400`)

### Intent Icon

| Intent | Icon | Color |
|--------|------|-------|
| add | `+` (in a circle) | Green |
| change | `~` (in a circle) | Amber |
| remove | `-` (in a circle) | Red |
| repair | wrench SVG | Orange |

Rendered as 20x20px SVG icons with the intent color.

### Progress Ring

SVG circular progress indicator for verification percentage.

```
Size: 48x48px (charter cards), 32x32px (compact)
Stroke width: 3px
Track: rgba(255, 255, 255, 0.06)
Fill: health color based on percentage
Center text: percentage in mono font

Animation: stroke-dasharray transition on mount (0.6s ease-out)
```

### Navigation

Minimal sidebar (56px wide collapsed, 200px expanded) OR horizontal top tabs.

**Recommended: Horizontal top tabs** (Vercel-style)

```
┌──────────────────────────────────────────────────┐
│  specctl                                          │
│  ─────────────────────────────────────────────    │
│  [Overview]  [Heatmap]  [Graph]  [Detail ▾]      │
└──────────────────────────────────────────────────┘
```

Tab anatomy:
- 14px text, 500 weight
- Active: `text-white` + 2px bottom border in accent color
- Inactive: `text-gray-500` hover → `text-gray-300`
- Transition: color 0.15s, border 0.15s
- Gap between tabs: 32px

### Skeleton Loading

Pulse animation on `surface-2` backgrounds:

```css
.skeleton {
  background: linear-gradient(
    90deg,
    var(--surface-1) 0%,
    var(--surface-2) 50%,
    var(--surface-1) 100%
  );
  background-size: 200% 100%;
  animation: skeleton-pulse 1.5s ease-in-out infinite;
  border-radius: 6px;
}

@keyframes skeleton-pulse {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
```

### Gherkin Syntax Block

```css
.gherkin-block {
  background: var(--surface-3);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  padding: 16px;
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.6;
}

/* Keyword highlighting */
.gherkin-keyword-feature  { color: #C084FC; font-weight: 600; } /* Purple */
.gherkin-keyword-scenario { color: #60A5FA; font-weight: 600; } /* Blue */
.gherkin-keyword-given    { color: #34D399; }                    /* Emerald */
.gherkin-keyword-when     { color: #FBBF24; }                    /* Amber */
.gherkin-keyword-then     { color: #F87171; }                    /* Red */
.gherkin-keyword-and      { color: #9CA3AF; }                    /* Gray */
```

---

## Layout Structure

### Page Shell

```
┌─ Full viewport ────────────────────────────────────────────┐
│                                                             │
│  ┌─ Header (h: 56px) ───────────────────────────────────┐  │
│  │  ◉ specctl    [Overview] [Heatmap] [Graph]    v1.0.0 │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌─ Content (flex-1, overflow-y: auto) ─────────────────┐  │
│  │                                                       │  │
│  │  ┌─ Max-width container (1280px, centered) ────────┐  │  │
│  │  │                                                  │  │  │
│  │  │  {Active View}                                   │  │  │
│  │  │                                                  │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  │                                                       │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘

Background: var(--void) #0A0A0A
Header: var(--void-raised) with bottom border-subtle
Content: var(--void)
```

### Overview Layout

```
┌─ Governance Summary Bar ──────────────────────────────────────────┐
│                                                                    │
│   48        124         72%           3                            │
│  SPECS   REQUIREMENTS  VERIFIED   OPEN DELTAS                     │
│                                                                    │
│  ── accent gradient line ──────────────────────────────────────── │
└────────────────────────────────────────────────────────────────────┘

┌─ Charter Cards Grid (repeat(auto-fill, minmax(320px, 1fr))) ─────┐
│                                                                    │
│  ┌─ Glass Card ──────────┐  ┌─ Glass Card ──────────┐            │
│  │                        │  │                        │            │
│  │  UI Charter            │  │  Runtime               │            │
│  │  UI specifications     │  │  Runtime system specs   │            │
│  │                        │  │                        │            │
│  │  ◎ 72%    6 specs      │  │  ◎ 100%   2 specs      │            │
│  │  3 open deltas         │  │  0 open deltas         │            │
│  │                        │  │                        │            │
│  │  [work-thread] [e2e-…] │  │  [e2e-journey-suite]   │            │
│  └────────────────────────┘  └────────────────────────┘            │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

### Heatmap Layout

```
┌─ Heatmap View ────────────────────────────────────────────────────┐
│                                                                    │
│  Legend: ■ 0%  ■ 25%  ■ 50%  ■ 75%  ■ 100%                      │
│                                                                    │
│  UI Charter                                                        │
│  ┌──────────┬──────────┬──────────┬──────────┬──────────┐         │
│  │work-thrd │ e2e-core │ e2e-mcp  │e2e-playw │          │         │
│  │  72%     │  85%     │  60%     │  90%     │          │         │
│  │  ■■■■□   │  ■■■■■   │  ■■■□□   │  ■■■■■   │          │         │
│  └──────────┴──────────┴──────────┴──────────┴──────────┘         │
│                                                                    │
│  Runtime Charter                                                   │
│  ┌──────────┬──────────┐                                           │
│  │e2e-journ │          │                                           │
│  │  100%    │          │                                           │
│  │  ■■■■■   │          │                                           │
│  └──────────┴──────────┘                                           │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘

Cell: 120x80px, rounded-lg, background interpolated from health scale
Hover: slight scale(1.02) + tooltip with spec name, status, req count
Click: navigate to spec detail
```

### Graph Layout

```
┌─ Dependency Graph ────────────────────────────────────────────────┐
│                                                                    │
│  Full-bleed SVG canvas (background: var(--void-deep) #050505)     │
│                                                                    │
│              ◉ e2e-core (green glow)                              │
│             / \                                                    │
│            /   \                                                   │
│    ◉ e2e-mcp   ◉ e2e-playwright                                  │
│         \       /                                                  │
│          \     /                                                   │
│     ◉ work-thread (amber glow)                                    │
│                                                                    │
│  Nodes: circles, 16-32px radius based on req count                │
│  Node fill: health color with 0.8 opacity                         │
│  Node stroke: health color at full, 2px                           │
│  Node glow: box-shadow / SVG filter with health color             │
│  Edges: #333 lines, 1px, with small arrowhead markers             │
│  Edge on hover (connected): brightens to #666                     │
│                                                                    │
│  ┌─ Floating tooltip ──────────┐                                  │
│  │  work-thread                │  Glass card, 200px wide          │
│  │  Status: active             │  Appears on node hover           │
│  │  Verified: 72% ◎            │  Positioned near cursor          │
│  │  Open deltas: 3             │                                  │
│  └─────────────────────────────┘                                  │
│                                                                    │
│  Controls: [Zoom +] [Zoom -] [Reset] — bottom-right, glass pills │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

### Spec Detail Layout

```
┌─ Breadcrumb ──────────────────────────────────────────────────────┐
│  Overview / UI / work-thread                                       │
└────────────────────────────────────────────────────────────────────┘

┌─ Spec Header ─────────────────────────────────────────────────────┐
│                                                                    │
│  Work Thread                                [● Active]  Rev 3     │
│  /work/:threadId — chat, session lifecycle                         │
│                                                                    │
│  Scope: ui/src/routes/_app/work/                                   │
│  Checkpoint: 28411f7c                                              │
│  Docs: SPEC.md ↗  JOURNEYS.md ↗                                   │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘

┌─ Two-column below (60/40 or single column on narrow) ────────────┐
│                                                                    │
│  ┌─ Deltas ────────────────┐  ┌─ Requirements ──────────────┐    │
│  │                          │  │                              │    │
│  │  D-001 [+ add] [closed] │  │  REQ-001 [✓ verified]       │    │
│  │  Session lifecycle       │  │  Thread creation             │    │
│  │  current → target        │  │  ┌─ gherkin ──────────────┐ │    │
│  │                          │  │  │ Feature: Thread creation│ │    │
│  │  D-002 [~ change] [open]│  │  │ Scenario: New thread    │ │    │
│  │  Error handling          │  │  │   Given authenticated   │ │    │
│  │  current → target        │  │  │   When creates thread   │ │    │
│  │                          │  │  │   Then thread appears   │ │    │
│  │  D-003 [🔧 repair]      │  │  └─────────────────────────┘ │    │
│  │  [deferred]              │  │  Tests: thread.spec.ts ↗    │    │
│  │                          │  │                              │    │
│  └──────────────────────────┘  │  REQ-002 [✗ unverified]     │    │
│                                │  Error handling              │    │
│  ┌─ Changelog ──────────────┐  │  ┌─ gherkin ──────────────┐ │    │
│  │  Rev 3 — 28411f7c        │  │  │ ...                     │ │    │
│  │  Rev 2 — a1b2c3d4        │  │  └─────────────────────────┘ │    │
│  │  Rev 1 — initial         │  │                              │    │
│  └──────────────────────────┘  └──────────────────────────────┘    │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

---

## Animation System

### Principles
1. **Purposeful** — motion communicates state changes, not decoration
2. **Quick** — 150-300ms for interactions, 400-600ms for page transitions
3. **Eased** — `cubic-bezier(0.16, 1, 0.3, 1)` (expo-out) for entrances, `ease` for hovers

### Timing Tokens

```css
:root {
  --duration-instant: 100ms;   /* Color changes, opacity toggles */
  --duration-fast:    150ms;   /* Hover states, badge transitions */
  --duration-normal:  250ms;   /* Card transitions, tab switches */
  --duration-slow:    400ms;   /* Page transitions, graph animations */
  --duration-reveal:  600ms;   /* Staggered entrance animations */

  --ease-out:    cubic-bezier(0.16, 1, 0.3, 1);    /* Entrances (expo-out) */
  --ease-in-out: cubic-bezier(0.65, 0, 0.35, 1);   /* Symmetric transitions */
  --ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1); /* Playful bounces (tooltips) */
}
```

### Page Load Orchestration

```css
/* Staggered reveal for cards on the overview page */
.card-enter {
  opacity: 0;
  transform: translateY(8px);
  animation: card-reveal var(--duration-reveal) var(--ease-out) forwards;
}

.card-enter:nth-child(1) { animation-delay: 0ms; }
.card-enter:nth-child(2) { animation-delay: 75ms; }
.card-enter:nth-child(3) { animation-delay: 150ms; }
.card-enter:nth-child(4) { animation-delay: 225ms; }

@keyframes card-reveal {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Governance summary numbers count up */
.stat-number {
  animation: stat-count var(--duration-slow) var(--ease-out);
}
```

### Graph Animations

```css
/* Nodes pulse gently based on health */
.graph-node-verified {
  animation: node-pulse-green 3s ease-in-out infinite;
}

@keyframes node-pulse-green {
  0%, 100% { filter: drop-shadow(0 0 4px rgba(34, 197, 94, 0.3)); }
  50%      { filter: drop-shadow(0 0 10px rgba(34, 197, 94, 0.5)); }
}

.graph-node-critical {
  animation: node-pulse-red 2s ease-in-out infinite;
}

@keyframes node-pulse-red {
  0%, 100% { filter: drop-shadow(0 0 4px rgba(239, 68, 68, 0.3)); }
  50%      { filter: drop-shadow(0 0 10px rgba(239, 68, 68, 0.5)); }
}
```

### View Transitions

```css
/* Crossfade between views (React Router) */
.view-enter { opacity: 0; transform: translateY(4px); }
.view-enter-active {
  opacity: 1;
  transform: translateY(0);
  transition: all var(--duration-normal) var(--ease-out);
}
.view-exit { opacity: 1; }
.view-exit-active {
  opacity: 0;
  transition: opacity var(--duration-fast);
}
```

---

## Responsive Behavior (v1: Desktop-First)

| Breakpoint | Width | Layout |
|------------|-------|--------|
| Desktop | >= 1024px | Full layout, 2-column spec detail |
| Tablet | 768-1023px | Single column, cards stack |
| Mobile | < 768px | Hidden (v2), show "Open on desktop" message |

Content max-width: **1280px** centered.

---

## Empty States

When a project has no specs, charters, or data:

```
┌─────────────────────────────────────────────────┐
│                                                   │
│              ◉ (large, dim, pulsing)              │
│                                                   │
│         No specifications found                   │
│                                                   │
│    Run `specctl init` to get started,             │
│    then `specctl charter create` to               │
│    create your first charter.                     │
│                                                   │
│         [Read the docs →]                         │
│                                                   │
└─────────────────────────────────────────────────┘

Style: centered, text-secondary, mono for commands
The pulsing circle is the "waiting to be born" constellation
```

---

## Icon System

Minimal — use text symbols and SVG sparingly:

| Concept | Icon | Rendering |
|---------|------|-----------|
| Verified | `✓` | SVG checkmark in circle, green |
| Unverified | `✗` | SVG cross in circle, red |
| Stale | `⚠` | SVG triangle-alert, amber |
| Add | `+` | SVG plus in circle, green |
| Change | `~` | SVG tilde in circle, amber |
| Remove | `−` | SVG minus in circle, red |
| Repair | wrench | SVG wrench, orange |
| External link | `↗` | SVG arrow-up-right, text-tertiary |
| Expand | `▸` | SVG chevron-right, rotates on expand |

Use [Lucide React](https://lucide.dev/) for consistent SVG icons — lightweight, tree-shakeable.

---

## Atmospheric Details

### Subtle Grain Overlay

```css
/* Optional: adds film grain texture to the void background */
.grain-overlay::before {
  content: '';
  position: fixed;
  inset: 0;
  opacity: 0.015;
  pointer-events: none;
  background-image: url("data:image/svg+xml,..."); /* noise SVG */
  z-index: 9999;
}
```

### Accent Gradient Line

A thin (1px) horizontal gradient line separating sections:

```css
.accent-divider {
  height: 1px;
  background: linear-gradient(
    90deg,
    transparent 0%,
    var(--accent) 50%,
    transparent 100%
  );
  opacity: 0.3;
}
```

### Graph Canvas Background

The dependency graph view uses a deeper void (#050505) with a subtle dot grid:

```css
.graph-canvas {
  background-color: #050505;
  background-image: radial-gradient(circle, #1a1a1a 1px, transparent 1px);
  background-size: 24px 24px;
}
```

---

---

## UX Review (ui-ux-pro-max validated)

### 1. Information Architecture — REVISED

**Finding:** The original 4-tab navigation (Overview / Heatmap / Graph / Detail) has a problem: "Detail" is not a top-level view — it's a drill-down destination. You don't browse "all details." You navigate TO a specific spec's detail from another view.

**Recommendation: 3 tabs + contextual drill-down**

```
Tabs: [Overview]  [Heatmap]  [Graph]
                                          ← These are peer views (browsable)
Drill-down: Overview → Charter → Spec Detail
            Heatmap cell → Spec Detail    ← Detail is a contextual destination
            Graph node → Spec Detail
```

Navigation rules:
- **3 top-level tabs** — Overview, Heatmap, Graph (peer views, always accessible)
- **Spec Detail** — reached by clicking a spec from ANY view (card, cell, or node)
- **Breadcrumb** — appears on Spec Detail: `Overview / UI / work-thread` with clickable segments
- **Back behavior** — browser back returns to the view you came from (not always Overview)
- **URL structure** — `/` (overview), `/heatmap`, `/graph`, `/specs/:charter/:slug` (detail)
- **Active tab** — remains highlighted for the view you navigated FROM when viewing a spec detail (breadcrumb provides context, not a 4th tab)

### 2. Drill-Down Flow — VALIDATED with improvements

```
Overview → click charter card → shows charter's spec list (inline expand or sub-view)
         → click spec → /specs/:charter/:slug (full detail page)

Heatmap  → click cell → /specs/:charter/:slug (direct jump)

Graph    → click node → /specs/:charter/:slug (direct jump)
         → hover node → tooltip (spec summary, don't navigate)
```

**Key UX rules (from ui-ux-pro-max):**
- `back-behavior`: React Router preserves history stack. Browser back works predictably.
- `breadcrumb-web`: 3+ level depth → breadcrumbs required on Spec Detail page.
- `state-preservation`: Navigating back from Spec Detail restores scroll position on the previous view.
- `deep-linking`: All routes are directly linkable — `/specs/ui/work-thread` works as a bookmark.

### 3. D3 Graph Interactions — ENHANCED

**Accessibility concern (Grade D from ui-ux-pro-max chart DB):** Network graphs are fundamentally inaccessible. MUST provide an alternative.

**Required additions:**
- **Adjacency list fallback**: Below the graph, provide a collapsible `<details>` with a table view of all spec dependencies (Node A → depends_on → Node B). Screen-reader accessible.
- **Keyboard navigation**: Tab through nodes (focus ring on active node), Enter to navigate to detail, Escape to deselect.
- **Node count guard**: specctl repos typically have <50 specs → SVG rendering is correct. If >100, add a warning and suggest filtering by charter.

**Interaction refinements:**
- **Hover tooltip delay**: 300ms delay before showing tooltip (prevents flicker during mouse traversal)
- **Drag threshold**: 3px movement threshold before starting drag (prevents accidental drags from clicks)
- **Zoom limits**: Min 0.3x, max 3x — prevent users from losing the graph
- **Reset button**: "Reset view" button (bottom-right, glass pill) returns to default zoom + centers graph
- **Charter filter**: Dropdown or toggle pills above the graph to show/hide charters — reduces visual noise

### 4. Spec Detail Information Hierarchy — REVISED

**Finding:** The original 2-column layout (Deltas | Requirements) treats them as equal. But requirements are the primary content (what the spec GUARANTEES), and deltas are secondary (what WORK remains). Users visit the detail page primarily to answer "what does this spec cover?" not "what work is left?"

**Revised hierarchy:**

```
┌─ Header ─────────────────────────────────────────────────┐
│  Work Thread                    [● Active]  Rev 3        │
│  /work/:threadId — chat, session lifecycle               │
│  Scope: ui/src/routes/_app/work/                         │
│  Docs: SPEC.md ↗                                         │
└──────────────────────────────────────────────────────────┘

┌─ Health Summary Bar ─────────────────────────────────────┐
│  [████████░░] 72% verified   7 requirements   3 deltas   │
└──────────────────────────────────────────────────────────┘

┌─ Requirements (PRIMARY — full width) ────────────────────┐
│                                                           │
│  REQ-001 [✓ verified] Thread creation       @ui          │
│  ┌─ gherkin (collapsed by default, expand on click) ──┐  │
│  │  Feature: Thread creation                           │  │
│  │  Scenario: New thread ...                           │  │
│  └─────────────────────────────────────────────────────┘  │
│  Tests: thread.spec.ts ↗                                  │
│                                                           │
│  REQ-002 [✗ unverified] Error handling      @ui          │
│  ...                                                      │
└──────────────────────────────────────────────────────────┘

┌─ Deltas (SECONDARY — collapsible section) ───────────────┐
│  ▸ 3 open deltas                    [expand/collapse]    │
│                                                           │
│  D-001 [+ add] [closed] Session lifecycle                │
│  D-002 [~ change] [open] Error handling                  │
│  ...                                                      │
└──────────────────────────────────────────────────────────┘

┌─ Changelog (TERTIARY — collapsible) ────────────────────┐
│  ▸ Revision history                 [expand/collapse]    │
└──────────────────────────────────────────────────────────┘
```

**Key changes:**
- Requirements are full-width and primary (not in a 60/40 column split)
- Gherkin blocks are collapsed by default (progressive disclosure) — click to expand
- Deltas section is collapsible — defaults to expanded if there are open deltas, collapsed if all closed
- Changelog is always collapsed by default (tertiary content)
- Health summary bar at top gives instant "how healthy is this spec?" answer

### 5. Additional UX Requirements

**From ui-ux-pro-max Priority 1-3 (CRITICAL/HIGH):**

| Rule | Requirement | Implementation |
|------|-------------|----------------|
| `color-contrast` | All text >= 4.5:1 on dark backgrounds | `#EDEDED` on `#0A0A0A` = 17.4:1 ✓ ; `#A0A0A0` on `#0A0A0A` = 9.3:1 ✓ ; `#666666` on `#0A0A0A` = 4.6:1 ✓ (barely passes) |
| `focus-states` | Visible focus rings on all interactive elements | 2px emerald ring: `focus:ring-2 focus:ring-emerald-500/50` |
| `color-not-only` | Health not conveyed by color alone | Health badges include text label (Verified/Unverified/Stale) + icon (✓/✗/⚠) alongside color |
| `keyboard-nav` | Full keyboard navigation | Tab through nav tabs, cards, cells, graph nodes. Enter to activate. Escape to deselect. |
| `skip-links` | Skip to main content | Hidden link visible on focus: "Skip to dashboard content" |
| `loading-states` | Skeleton screens for > 300ms loads | Shimmer skeletons matching each view's card layout |
| `cursor-pointer` | All clickable elements | Cards, cells, graph nodes, links, tabs |
| `reduced-motion` | Respect `prefers-reduced-motion` | Disable graph pulsing, card stagger reveal, stat count-up. Keep instant state changes. |

**From ui-ux-pro-max Priority 7 (Animation):**

| Rule | Applied |
|------|---------|
| `duration-timing` | Hover: 150ms, view transition: 250ms, graph physics: continuous |
| `easing` | Entrances: `ease-out`, exits: `ease-in`, graph: spring physics |
| `exit-faster-than-enter` | Card exit: 150ms, card enter: 250ms |
| `stagger-sequence` | Charter cards: 75ms stagger per card (max 6 = 450ms total) |
| `interruptible` | All animations cancelable by user interaction |
| `no-blocking-animation` | UI remains interactive during all animations |

**From ui-ux-pro-max Priority 8 (Forms & Feedback):**

| Rule | Applied |
|------|---------|
| `empty-states` | Friendly message + `specctl init` command when no .specs/ found |
| `progressive-disclosure` | Gherkin blocks collapsed by default, deltas collapsible, changelog collapsed |

**From ui-ux-pro-max Priority 10 (Charts):**

| Rule | Applied |
|------|---------|
| `legend-visible` | Heatmap has visible legend (gradient bar with 0%/50%/100%) |
| `tooltip-on-interact` | Graph nodes + heatmap cells show tooltip on hover |
| `empty-data-state` | Graph with no dependencies shows "No spec dependencies found" message |
| `screen-reader-summary` | Graph has `aria-label` describing total nodes and key relationships |
| `touch-target-chart` | Graph nodes have minimum 32px radius (exceeds 44pt for larger nodes) |

---

## Design Quality Checklist

Before shipping, verify:

- [ ] All text is readable (contrast ratio >= 4.5:1 for body, 3:1 for large text)
- [ ] Glass cards have visible borders on all backgrounds
- [ ] Health colors are distinguishable (not just red/green — amber/orange bridge the spectrum)
- [ ] Gherkin blocks are syntax-highlighted with correct keyword colors
- [ ] Graph nodes glow and pulse — the constellation feels alive
- [ ] Staggered card reveal animation on page load
- [ ] Stat numbers use the `stat` type scale (48px, tight tracking)
- [ ] Navigation tabs have clear active/inactive states
- [ ] Empty states are graceful, not broken-looking
- [ ] Screenshot test: does this look Vercel-quality in a screenshot?
