import { useEffect, useState } from 'react'

function healthColor(pct: number): string {
  if (pct >= 0.75) return '#4ae176'   // secondary
  if (pct >= 0.50) return '#eab308'   // amber
  if (pct >= 0.25) return '#f97316'   // orange
  return '#ffb4ab'                     // error
}

interface ProgressRingProps {
  percentage: number  // 0 to 1
  size?: number
  strokeWidth?: number
  showLabel?: boolean
}

export function ProgressRing({ percentage, size = 48, strokeWidth = 2, showLabel = true }: ProgressRingProps) {
  const [animated, setAnimated] = useState(false)
  const radius = (size - strokeWidth * 2) / 2
  const circumference = radius * 2 * Math.PI
  const color = healthColor(percentage)
  const pct = Math.max(0, Math.min(1, percentage))

  const strokeDashoffset = animated
    ? circumference - pct * circumference
    : circumference

  useEffect(() => {
    const id = setTimeout(() => setAnimated(true), 50)
    return () => clearTimeout(id)
  }, [])

  const fontSize = size <= 32 ? 8 : size <= 48 ? 10 : 12
  const label = `${Math.round(pct * 100)}%`

  return (
    <svg
      width={size}
      height={size}
      viewBox={`0 0 ${size} ${size}`}
      aria-label={`${label} verified`}
      role="img"
    >
      {/* Track — surface-container-highest (#353534) */}
      <circle
        cx={size / 2}
        cy={size / 2}
        r={radius}
        fill="none"
        stroke="#353534"
        strokeWidth={strokeWidth}
      />
      {/* Progress */}
      <circle
        cx={size / 2}
        cy={size / 2}
        r={radius}
        fill="none"
        stroke={color}
        strokeWidth={strokeWidth}
        strokeLinecap="round"
        strokeDasharray={circumference}
        strokeDashoffset={strokeDashoffset}
        transform={`rotate(-90 ${size / 2} ${size / 2})`}
        style={{ transition: 'stroke-dashoffset 0.6s cubic-bezier(0.16, 1, 0.3, 1)' }}
      />
      {/* Center label */}
      {showLabel && (
        <text
          x={size / 2}
          y={size / 2}
          textAnchor="middle"
          dominantBaseline="central"
          fill={color}
          fontSize={fontSize}
          fontFamily="'JetBrains Mono', monospace"
          fontWeight="500"
        >
          {label}
        </text>
      )}
    </svg>
  )
}
