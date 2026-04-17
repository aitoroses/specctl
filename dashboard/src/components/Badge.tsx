export type BadgeVariant =
  | 'verified'
  | 'unverified'
  | 'stale'
  | 'open'
  | 'in_progress'
  | 'closed'
  | 'deferred'
  | 'active'
  | 'superseded'
  | 'withdrawn'

interface BadgeProps {
  variant: BadgeVariant
  label: string
}

const variantStyles: Record<BadgeVariant, { dot: string; bg: string; text: string; glow?: string }> = {
  verified:    { dot: 'bg-secondary',          bg: 'bg-secondary/10',         text: 'text-secondary' },
  unverified:  { dot: 'bg-error',              bg: 'bg-error/10',             text: 'text-error' },
  stale:       { dot: 'bg-amber-400',          bg: 'bg-amber-400/10',         text: 'text-amber-400' },
  open:        { dot: 'bg-error',              bg: 'bg-error/10',             text: 'text-error',       glow: '0 0 6px rgba(255,180,171,0.3)' },
  in_progress: { dot: 'bg-primary',            bg: 'bg-primary/10',           text: 'text-primary' },
  closed:      { dot: 'bg-secondary',          bg: 'bg-secondary/10',         text: 'text-secondary' },
  deferred:    { dot: 'bg-zinc-500',           bg: 'bg-white/5',              text: 'text-zinc-500' },
  active:      { dot: 'bg-primary animate-pulse', bg: 'bg-primary/10',        text: 'text-primary',     glow: '0 0 8px rgba(78,222,163,0.2)' },
  superseded:  { dot: 'bg-amber-400',          bg: 'bg-amber-400/10',         text: 'text-amber-400' },
  withdrawn:   { dot: 'bg-zinc-600',           bg: 'bg-white/5',              text: 'text-zinc-500' },
}

export function Badge({ variant, label }: BadgeProps) {
  const styles = variantStyles[variant] ?? variantStyles.deferred

  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded font-mono text-[9px] uppercase tracking-widest ${styles.bg} ${styles.text}`}
      style={styles.glow ? { boxShadow: styles.glow } : undefined}
    >
      <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${styles.dot}`} />
      {label}
    </span>
  )
}
