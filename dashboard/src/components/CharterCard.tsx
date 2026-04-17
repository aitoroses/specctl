import { useNavigate } from 'react-router-dom'
import type { CharterSummary } from '../api/types'
import { Card } from './Card'
import { ProgressRing } from './ProgressRing'
import { Badge } from './Badge'

interface CharterCardProps {
  charter: CharterSummary
  index: number
}

export function CharterCard({ charter, index }: CharterCardProps) {
  const navigate = useNavigate()

  const handleSpecClick = (slug: string) => {
    navigate(`/specs/${charter.name}/${slug}`)
  }

  return (
    <div
      className="card-enter"
      style={{ animationDelay: `${index * 75}ms` }}
    >
      <Card className="p-5 flex flex-col gap-4 relative overflow-hidden">
        {/* Subtle ambient glow top-right */}
        <div className="absolute top-0 right-0 w-24 h-24 bg-primary/5 rounded-full blur-2xl -mr-12 -mt-12 pointer-events-none" />

        {/* Header */}
        <div className="relative z-10">
          <h2 className="font-headline font-bold text-on-surface leading-tight text-lg tracking-tight">
            {charter.name}
          </h2>
          <p className="text-zinc-400 mt-1 text-xs leading-relaxed">
            {charter.title}
          </p>
        </div>

        {/* Spec list */}
        {charter.specs.length > 0 && (
          <div className="flex flex-col gap-1.5 relative z-10">
            {charter.specs.map((spec) => (
              <button
                key={spec.slug}
                onClick={() => handleSpecClick(spec.slug)}
                className="flex items-center justify-between px-3 py-2 rounded-md bg-white/[0.03] hover:bg-white/[0.07] border border-white/5 hover:border-white/10 transition-all duration-150 text-left group cursor-pointer"
              >
                <div className="flex items-center gap-2.5 min-w-0">
                  <span className="font-mono text-xs text-on-surface group-hover:text-primary-container transition-colors truncate">
                    {spec.slug}
                  </span>
                  <span className="text-[10px] text-zinc-500 truncate hidden sm:inline">
                    {spec.title}
                  </span>
                </div>
                <div className="flex items-center gap-2 shrink-0 ml-2">
                  <span className="font-mono text-[10px] text-zinc-500">
                    {Math.round(spec.verified_pct * 100)}%
                  </span>
                  <span className="material-symbols-outlined text-sm text-zinc-600 group-hover:text-zinc-400 transition-colors">
                    arrow_forward
                  </span>
                </div>
              </button>
            ))}
          </div>
        )}

        {/* Bottom: progress ring + stats */}
        <div className="flex items-center justify-between mt-auto pt-4 border-t border-white/5 relative z-10">
          <div className="flex items-center gap-3">
            <ProgressRing percentage={charter.verified_pct} size={44} strokeWidth={2} />
            <div className="flex flex-col gap-1.5">
              <span className="font-mono text-[10px] uppercase tracking-widest text-zinc-500">
                {charter.spec_count} {charter.spec_count === 1 ? 'spec' : 'specs'}
              </span>
              {charter.open_deltas > 0 ? (
                <Badge variant="open" label={`${charter.open_deltas} delta${charter.open_deltas !== 1 ? 's' : ''}`} />
              ) : (
                <span className="font-mono text-[9px] uppercase tracking-widest text-zinc-600">no open deltas</span>
              )}
            </div>
          </div>
        </div>
      </Card>
    </div>
  )
}
