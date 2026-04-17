import type { ReactNode, KeyboardEvent } from 'react'

interface CardProps {
  children: ReactNode
  className?: string
  onClick?: () => void
  hoverable?: boolean
}

export function Card({ children, className = '', onClick, hoverable = false }: CardProps) {
  const base = 'glass-card rounded-xl'
  const hover = hoverable || onClick ? 'hover:brightness-110 transition-all duration-150' : ''
  const cursor = onClick || hoverable ? 'cursor-pointer' : ''

  return (
    <div
      className={`${base} ${hover} ${cursor} ${className}`}
      onClick={onClick}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
      onKeyDown={onClick ? (e: KeyboardEvent<HTMLDivElement>) => { if (e.key === 'Enter' || e.key === ' ') onClick() } : undefined}
    >
      {children}
    </div>
  )
}
