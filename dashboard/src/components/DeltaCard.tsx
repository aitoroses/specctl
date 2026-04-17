import type { Delta } from '../api/types'

interface DeltaCardProps {
  delta: Delta
}

const statusColors: Record<Delta['status'], { border: string; idColor: string; labelBg: string; labelColor: string }> = {
  open:        { border: 'border-error/50',     idColor: 'text-error',     labelBg: 'bg-error/10',  labelColor: 'text-error/80' },
  in_progress: { border: 'border-primary/50',   idColor: 'text-primary',   labelBg: 'bg-primary/10', labelColor: 'text-primary/80' },
  closed:      { border: 'border-secondary/50', idColor: 'text-secondary', labelBg: 'bg-white/5',    labelColor: 'text-zinc-500' },
  deferred:    { border: 'border-zinc-600/50',  idColor: 'text-zinc-500',  labelBg: 'bg-white/5',    labelColor: 'text-zinc-500' },
}

const statusLabels: Record<Delta['status'], string> = {
  open:        'OPEN',
  in_progress: 'IN PROGRESS',
  closed:      'RESOLVED',
  deferred:    'DEFERRED',
}

export function DeltaCard({ delta }: DeltaCardProps) {
  const colors = statusColors[delta.status] ?? statusColors.open

  return (
    <div className={`glass-card rounded-lg p-4 border-l-4 ${colors.border}`}>
      <div className="flex justify-between items-start mb-2">
        <span className={`text-[10px] font-mono ${colors.idColor}`}>{delta.id}</span>
        <span className={`text-[9px] font-mono ${colors.labelColor} ${colors.labelBg} px-1.5 py-0.5 rounded`}>
          {statusLabels[delta.status]}
        </span>
      </div>
      <h5 className="text-xs font-medium mb-1">{delta.area}</h5>
      <p className="text-[11px] text-zinc-400 line-clamp-2">
        {delta.notes || `${delta.current} → ${delta.target}`}
      </p>
    </div>
  )
}
