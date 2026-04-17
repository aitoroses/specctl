export interface OverviewData {
  total_charters: number
  total_specs: number
  total_requirements: number
  verified_pct: number
  open_deltas: number
  charters: CharterSummary[]
}

export interface CharterSpecEntry {
  slug: string
  title: string
  verified_pct: number
}

export interface CharterSummary {
  name: string
  title: string
  spec_count: number
  verified_pct: number
  open_deltas: number
  specs: CharterSpecEntry[]
}

export interface SpecDetail {
  slug: string
  charter: string
  title: string
  status: string
  rev: number
  checkpoint: string
  scope: string[]
  deltas: Delta[]
  requirements: Requirement[]
  documents: {
    primary: string
    secondary?: string[]
  }
  changelog: ChangelogEntry[]
}

export interface Delta {
  id: string
  area: string
  intent: 'add' | 'change' | 'remove' | 'repair'
  status: 'open' | 'in_progress' | 'closed' | 'deferred'
  current: string
  target: string
  notes: string
  affects_requirements: string[]
}

export interface Requirement {
  id: string
  title: string
  tags: string[]
  gherkin: string
  lifecycle: 'active' | 'superseded' | 'withdrawn'
  verification: 'unverified' | 'verified' | 'stale'
  test_files: string[]
}

export interface GraphData {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface GraphNode {
  id: string
  label: string
  charter: string
  status: string
  verified_pct: number
  req_count: number
  group: string
}

export interface GraphEdge {
  source: string
  target: string
}

export interface ChangelogEntry {
  rev: number
  checkpoint: string
  summary: string
  date: string
}

// Static data injected by Go in static mode
declare global {
  interface Window {
    __SPECCTL_DATA__?: {
      overview?: OverviewData
      graph?: GraphData
      [key: string]: unknown
    }
  }
}
