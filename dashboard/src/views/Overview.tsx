import { useOverview } from '../api/queries'
import { GovernanceSummary } from '../components/GovernanceSummary'
import { CharterCard } from '../components/CharterCard'
import { EmptyState } from '../components/EmptyState'
import { Skeleton } from '../components/Skeleton'

function LoadingSkeleton() {
  return (
    <div>
      {/* Governance summary skeleton */}
      <div className="glass-card rounded-lg p-6 mb-8">
        <div className="flex gap-10 flex-wrap">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="flex flex-col gap-2">
              <Skeleton className="h-6 w-16" />
              <Skeleton className="h-2 w-20" />
            </div>
          ))}
        </div>
      </div>
      {/* Charter cards skeleton */}
      <div
        className="grid gap-6"
        style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))' }}
      >
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-44 rounded-lg" />
        ))}
      </div>
    </div>
  )
}

export function Overview() {
  const { data, isLoading, isError } = useOverview()

  if (isLoading) return <LoadingSkeleton />

  if (isError || !data) {
    return (
      <EmptyState
        title="Failed to load overview"
        description="Could not fetch governance data. Make sure specctl is running."
        command="specctl dashboard"
      />
    )
  }

  if (data.charters.length === 0) {
    return (
      <EmptyState
        title="No specifications found"
        description="Run specctl init to get started, then specctl charter create to create your first charter."
        command="specctl init"
      />
    )
  }

  return (
    <div>
      <GovernanceSummary data={data} />

      <div
        className="grid gap-6"
        style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))' }}
        role="list"
        aria-label="Charter cards"
      >
        {data.charters.map((charter, index) => (
          <div key={charter.name} role="listitem">
            <CharterCard charter={charter} index={index} />
          </div>
        ))}
      </div>
    </div>
  )
}
