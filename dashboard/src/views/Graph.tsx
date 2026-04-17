import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useGraph } from '../api/queries'
import { ForceGraph } from '../charts/ForceGraph'
import { Skeleton } from '../components/Skeleton'
import { EmptyState } from '../components/EmptyState'

export function Graph() {
  const { data, isLoading } = useGraph()
  const navigate = useNavigate()
  const wrapperRef = useRef<HTMLDivElement>(null)
  const [dimensions, setDimensions] = useState({ width: 960, height: 600 })

  // Resize observer to track container size
  useEffect(() => {
    const el = wrapperRef.current
    if (!el) return

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (!entry) return
      const { width, height } = entry.contentRect
      if (width > 0 && height > 0) {
        setDimensions({ width, height })
      }
    })

    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  const handleNodeClick = useCallback(
    (charter: string, slug: string) => {
      navigate(`/specs/${charter}/${slug}`)
    },
    [navigate]
  )

  if (isLoading) {
    return (
      <div>
        <h1 className="font-headline font-bold text-on-surface text-2xl tracking-tight mb-6">
          Dependency Graph
        </h1>
        <Skeleton className="rounded-lg w-full" style={{ height: 600 }} />
      </div>
    )
  }

  if (!data || data.nodes.length === 0) {
    return (
      <div>
        <h1 className="font-headline font-bold text-on-surface text-2xl tracking-tight mb-6">
          Dependency Graph
        </h1>
        <EmptyState
          title="No specifications found"
          description="Create specs with dependencies to see the dependency graph."
          command="specctl spec create --charter <name>"
        />
      </div>
    )
  }

  return (
    <div>
      <h1 className="font-headline font-bold text-on-surface text-2xl tracking-tight mb-6">
        Dependency Graph
      </h1>

      {/* Graph container — full available height */}
      <div
        ref={wrapperRef}
        className="rounded-lg overflow-hidden"
        style={{ height: 'calc(100vh - 56px - 48px - 60px - 32px)' }}
      >
        <ForceGraph
          data={data}
          width={dimensions.width}
          height={dimensions.height}
          onNodeClick={handleNodeClick}
        />
      </div>

      {/* Accessibility fallback: adjacency list table */}
      <details className="mt-6">
        <summary className="text-on-surface-variant text-[10px] font-mono uppercase tracking-widest cursor-pointer hover:text-on-surface transition-colors duration-150">
          View as table
        </summary>
        <div className="mt-3 overflow-x-auto">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr style={{ borderBottom: '1px solid rgba(42, 42, 42, 0.8)' }}>
                <th className="text-left py-2 px-3 text-outline text-[10px] font-mono uppercase tracking-widest font-medium">Spec</th>
                <th className="text-left py-2 px-3 text-outline text-[10px] font-mono uppercase tracking-widest font-medium">Charter</th>
                <th className="text-left py-2 px-3 text-outline text-[10px] font-mono uppercase tracking-widest font-medium">Depends On</th>
                <th className="text-left py-2 px-3 text-outline text-[10px] font-mono uppercase tracking-widest font-medium">Status</th>
                <th className="text-right py-2 px-3 text-outline text-[10px] font-mono uppercase tracking-widest font-medium">Verified</th>
              </tr>
            </thead>
            <tbody>
              {data.nodes.map((node) => {
                const deps = data.edges
                  .filter((e) => e.source === node.id)
                  .map((e) => {
                    const target = data.nodes.find((n) => n.id === e.target)
                    return target?.label ?? e.target
                  })

                return (
                  <tr
                    key={node.id}
                    className="hover:bg-surface-container-low/50"
                    style={{ borderBottom: '1px solid rgba(28, 27, 27, 0.8)' }}
                  >
                    <td className="py-2 px-3 text-on-surface font-mono text-xs">{node.label}</td>
                    <td className="py-2 px-3 text-on-surface-variant font-mono text-xs">{node.charter}</td>
                    <td className="py-2 px-3 text-on-surface-variant font-mono text-xs">
                      {deps.length > 0 ? deps.join(', ') : '—'}
                    </td>
                    <td className="py-2 px-3 text-on-surface-variant font-mono text-xs">{node.status}</td>
                    <td className="py-2 px-3 text-right text-on-surface-variant font-mono text-xs">
                      {Math.round(node.verified_pct * 100)}%
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      </details>
    </div>
  )
}
