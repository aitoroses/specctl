import { useParams, Link } from 'react-router-dom'
import { useSpec } from '../api/queries'
import { Skeleton } from '../components/Skeleton'
import { RequirementList } from '../components/RequirementList'
import { DeltaList } from '../components/DeltaList'
import { ChangelogTimeline } from '../components/ChangelogTimeline'

export function SpecDetail() {
  const { charter, slug } = useParams<{ charter: string; slug: string }>()
  const { data: spec, isLoading, isError } = useSpec(charter ?? '', slug ?? '')

  if (isLoading) {
    return (
      <div>
        <Skeleton className="h-4 w-48 mb-6" />
        <Skeleton className="h-40 rounded-xl mb-10" />
        <div className="grid grid-cols-12 gap-8">
          <div className="col-span-12 lg:col-span-8 space-y-4">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="h-32 rounded-lg" />
            <Skeleton className="h-32 rounded-lg" />
          </div>
          <div className="col-span-12 lg:col-span-4 space-y-4">
            <Skeleton className="h-6 w-32" />
            <Skeleton className="h-24 rounded-lg" />
            <Skeleton className="h-24 rounded-lg" />
          </div>
        </div>
      </div>
    )
  }

  if (isError || !spec) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <div className="w-3 h-3 rounded-full bg-error mb-4 opacity-60" />
        <p className="text-zinc-500 text-sm">
          Could not load spec <span className="font-mono text-on-surface">{charter}/{slug}</span>.
        </p>
      </div>
    )
  }

  const verified = spec.requirements.filter(r => r.verification === 'verified').length
  const pct = spec.requirements.length === 0 ? 0 : Math.round((verified / spec.requirements.length) * 1000) / 10

  return (
    <div className="card-enter">
      {/* Breadcrumb */}
      <nav className="flex items-center gap-2 mb-6 font-mono text-[10px] uppercase tracking-widest text-zinc-500">
        <Link to="/" className="hover:text-primary cursor-pointer transition-colors">
          Overview
        </Link>
        <span className="material-symbols-outlined text-[12px]">chevron_right</span>
        <span className="hover:text-primary cursor-pointer transition-colors">
          {spec.charter}
        </span>
        <span className="material-symbols-outlined text-[12px]">chevron_right</span>
        <span className="text-on-surface">{spec.slug}</span>
      </nav>

      {/* Glass Header Card */}
      <div className="glass-card rounded-xl p-8 mb-10 relative overflow-hidden">
        {/* Ambient glow */}
        <div className="absolute top-0 right-0 w-64 h-64 bg-primary/5 rounded-full blur-3xl -mr-32 -mt-32" />

        <div className="flex flex-col md:flex-row md:items-end justify-between gap-6 relative z-10">
          {/* Left: Title + Badge + Description */}
          <div>
            <div className="flex items-center gap-4 mb-2">
              <h2 className="text-4xl font-headline font-bold tracking-tight text-on-surface">
                {spec.title}
              </h2>
              <span className="px-3 py-1 bg-primary/10 border border-primary/20 text-primary text-[10px] font-mono rounded-full flex items-center gap-1.5 uppercase tracking-tighter">
                <span className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" />
                {spec.status}
              </span>
            </div>
            {spec.scope.length > 0 && (
              <p className="text-zinc-400 max-w-2xl text-sm leading-relaxed">
                {spec.scope.join(' / ')}
              </p>
            )}
          </div>

          {/* Right: Stats Grid */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-8 border-l border-white/5 pl-8">
            <div>
              <span className="block text-[10px] text-zinc-500 font-mono uppercase tracking-widest mb-1">
                Rev
              </span>
              <span className="text-xl font-headline font-medium text-on-surface">
                {spec.rev}.0.0
              </span>
            </div>
            <div>
              <span className="block text-[10px] text-zinc-500 font-mono uppercase tracking-widest mb-1">
                Scope
              </span>
              <span className="text-xl font-headline font-medium text-on-surface">
                {spec.scope[0] ?? 'Global'}
              </span>
            </div>
            <div>
              <span className="block text-[10px] text-zinc-500 font-mono uppercase tracking-widest mb-1">
                Checkpoint
              </span>
              <span className="text-xl font-headline font-medium text-on-surface">
                {spec.checkpoint ? spec.checkpoint.slice(0, 7) : '---'}
              </span>
            </div>
            <div>
              <span className="block text-[10px] text-zinc-500 font-mono uppercase tracking-widest mb-1">
                Coverage
              </span>
              <span className="text-xl font-headline font-medium text-secondary">
                {pct}%
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Bento Grid Content */}
      <div className="grid grid-cols-12 gap-8">
        {/* Primary: Requirements (left) */}
        <RequirementList requirements={spec.requirements} />

        {/* Sidebar (right) */}
        <aside className="col-span-12 lg:col-span-4 space-y-8">
          <DeltaList deltas={spec.deltas} />
          <ChangelogTimeline changelog={spec.changelog} />
        </aside>
      </div>
    </div>
  )
}
