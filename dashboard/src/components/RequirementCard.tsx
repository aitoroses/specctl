import { useState } from 'react'
import { GherkinBlock } from './GherkinBlock'
import type { Requirement } from '../api/types'

interface RequirementCardProps {
  requirement: Requirement
}

export function RequirementCard({ requirement }: RequirementCardProps) {
  const isVerified = requirement.verification === 'verified'
  const [expanded, setExpanded] = useState(isVerified)

  return (
    <div
      className={`glass-card rounded-lg overflow-hidden${
        !isVerified ? ' opacity-80 hover:opacity-100 transition-opacity' : ''
      }`}
    >
      {/* Header */}
      <div className="p-6 border-b border-white/5 flex items-start justify-between">
        <div className="flex gap-4">
          <div className="mt-1">
            {isVerified ? (
              <span
                className="material-symbols-outlined text-secondary"
                style={{ fontVariationSettings: "'FILL' 1" }}
              >
                verified
              </span>
            ) : (
              <span className="material-symbols-outlined text-zinc-600">
                hourglass_empty
              </span>
            )}
          </div>
          <div>
            <span
              className={`text-[10px] font-mono mb-1 block ${
                isVerified ? 'text-primary' : 'text-zinc-500'
              }`}
            >
              {requirement.id}
            </span>
            <h4 className="text-on-surface font-medium">{requirement.title}</h4>
            {requirement.tags.length > 0 && (
              <p className="text-xs text-zinc-400 mt-1">
                {requirement.tags.join(', ')}
              </p>
            )}
          </div>
        </div>
        <button
          onClick={() => setExpanded(prev => !prev)}
          className="cursor-pointer"
          aria-expanded={expanded}
        >
          <span className="material-symbols-outlined text-zinc-600 hover:text-zinc-200">
            {expanded ? 'expand_less' : 'expand_more'}
          </span>
        </button>
      </div>

      {/* Gherkin (expanded) */}
      {expanded && requirement.gherkin && (
        <GherkinBlock gherkin={requirement.gherkin} />
      )}
    </div>
  )
}
