import { RequirementCard } from './RequirementCard'
import type { Requirement } from '../api/types'

interface RequirementListProps {
  requirements: Requirement[]
}

export function RequirementList({ requirements }: RequirementListProps) {
  return (
    <section className="col-span-12 lg:col-span-8 space-y-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="font-headline text-lg font-semibold flex items-center gap-2">
          <span className="material-symbols-outlined text-primary">list_alt</span>
          Functional Requirements
        </h3>
        <div className="flex gap-2">
          <span className="text-[10px] font-mono text-zinc-500">
            {requirements.length} {requirements.length === 1 ? 'ITEM' : 'ITEMS'}
          </span>
        </div>
      </div>

      {requirements.length === 0 ? (
        <p className="text-sm text-zinc-500">No requirements defined.</p>
      ) : (
        requirements.map(req => (
          <RequirementCard key={req.id} requirement={req} />
        ))
      )}
    </section>
  )
}
