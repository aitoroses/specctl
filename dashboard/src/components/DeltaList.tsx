import { DeltaCard } from './DeltaCard'
import type { Delta } from '../api/types'

interface DeltaListProps {
  deltas: Delta[]
}

export function DeltaList({ deltas }: DeltaListProps) {
  return (
    <section>
      <h3 className="font-headline text-sm font-bold uppercase tracking-widest text-zinc-500 mb-4 flex items-center gap-2">
        <span className="material-symbols-outlined text-xs">analytics</span>
        Delta Tracking
      </h3>
      {deltas.length === 0 ? (
        <p className="text-sm text-zinc-500">No deltas recorded.</p>
      ) : (
        <div className="space-y-3">
          {deltas.map(delta => (
            <DeltaCard key={delta.id} delta={delta} />
          ))}
        </div>
      )}
    </section>
  )
}
