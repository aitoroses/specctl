interface EmptyStateProps {
  title: string
  description: string
  command?: string
  icon?: string
}

export function EmptyState({ title, description, command, icon = 'satellite_alt' }: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-24 text-center">
      <div className="relative mb-8">
        <div className="w-16 h-16 rounded-xl glass-card flex items-center justify-center">
          <span className="material-symbols-outlined text-zinc-600 text-3xl">{icon}</span>
        </div>
        <div className="absolute inset-0 rounded-xl border border-primary/10 animate-pulse pointer-events-none" />
      </div>

      <h3 className="font-headline font-bold text-on-surface text-lg mb-2 tracking-tight">{title}</h3>
      <p className="text-zinc-400 text-sm max-w-xs mb-6 leading-relaxed">{description}</p>

      {command && (
        <code className="font-mono text-[11px] text-primary bg-primary/10 px-4 py-2 rounded border border-primary/20 uppercase tracking-widest">
          {command}
        </code>
      )}
    </div>
  )
}
