import type { ChangelogEntry } from '../api/types'

interface ChangelogTimelineProps {
  changelog: ChangelogEntry[]
}

function formatDate(dateStr: string): string {
  try {
    const date = new Date(dateStr)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

    if (diffDays === 0) return `Today, ${date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })}`
    if (diffDays === 1) return `Yesterday, ${date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })}`
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return dateStr
  }
}

export function ChangelogTimeline({ changelog }: ChangelogTimelineProps) {
  if (changelog.length === 0) {
    return (
      <section>
        <h3 className="font-headline text-sm font-bold uppercase tracking-widest text-zinc-500 mb-4 flex items-center gap-2">
          <span className="material-symbols-outlined text-xs">history</span>
          History
        </h3>
        <p className="text-sm text-zinc-500">No history entries.</p>
      </section>
    )
  }

  return (
    <section>
      <h3 className="font-headline text-sm font-bold uppercase tracking-widest text-zinc-500 mb-4 flex items-center gap-2">
        <span className="material-symbols-outlined text-xs">history</span>
        History
      </h3>
      <div className="relative pl-4 space-y-6 before:content-[''] before:absolute before:left-[1px] before:top-0 before:bottom-0 before:w-[1px] before:bg-white/5">
        {changelog.map((entry, i) => (
          <div key={entry.rev} className="relative">
            <div
              className={`absolute -left-[19px] top-1 w-2 h-2 rounded-full border-2 border-surface ${
                i === 0 ? 'bg-primary' : 'bg-zinc-700'
              }`}
            />
            <p className="text-[10px] font-mono text-zinc-500 mb-1">
              {entry.date ? formatDate(entry.date) : `Rev ${entry.rev}`}
            </p>
            <p className="text-xs text-on-surface-variant font-medium">
              {entry.summary}
            </p>
            <p className="text-[10px] text-zinc-500">
              Checkpoint {entry.checkpoint.slice(0, 8)}
            </p>
          </div>
        ))}
      </div>
    </section>
  )
}
