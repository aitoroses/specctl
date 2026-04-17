import type { OverviewData } from '../api/types'

interface StatBlockProps {
  value: string
  label: string
  highlight?: boolean
}

function StatBlock({ value, label, highlight = false }: StatBlockProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className={`font-headline font-bold leading-none tracking-tight text-2xl ${highlight ? 'text-secondary' : 'text-on-surface'}`}>
        {value}
      </span>
      <span className="font-mono text-[10px] uppercase tracking-widest text-zinc-500">
        {label}
      </span>
    </div>
  )
}

interface GovernanceSummaryProps {
  data: OverviewData
}

export function GovernanceSummary({ data }: GovernanceSummaryProps) {
  const verifiedPct = `${Math.round(data.verified_pct * 100)}%`

  return (
    <section aria-label="Governance summary" className="glass-card rounded-xl p-6 mb-8 relative overflow-hidden">
      {/* Ambient glow */}
      <div className="absolute top-0 right-0 w-48 h-48 bg-primary/5 rounded-full blur-3xl -mr-24 -mt-24 pointer-events-none" />

      <div className="flex gap-10 flex-wrap relative z-10">
        <StatBlock value={String(data.total_specs)} label="Specs" />
        <StatBlock value={String(data.total_requirements)} label="Requirements" />
        <StatBlock value={verifiedPct} label="Verified" highlight />
        <StatBlock value={String(data.open_deltas)} label="Open Deltas" />
      </div>
    </section>
  )
}
