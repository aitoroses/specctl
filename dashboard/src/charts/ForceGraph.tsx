import { useRef, useEffect, useState, useCallback } from 'react'
import * as d3 from 'd3'
import type { GraphData, GraphNode } from '../api/types'

interface ForceGraphProps {
  data: GraphData
  width: number
  height: number
  onNodeClick: (charter: string, slug: string) => void
}

interface TooltipState {
  node: GraphNode
  x: number
  y: number
}

// Simulation node extends GraphNode with D3 force positions
interface SimNode extends GraphNode {
  x: number
  y: number
  fx: number | null
  fy: number | null
  vx: number
  vy: number
  index: number
}

interface SimEdge extends d3.SimulationLinkDatum<SimNode> {
  source: SimNode
  target: SimNode
}

// Updated health scale: error #FFB4AB → amber #EAB308 → primary #4EDEA3
const healthScale = (pct: number): string => {
  if (pct <= 0.5) {
    return d3.interpolateRgb('#FFB4AB', '#EAB308')(pct * 2)
  }
  return d3.interpolateRgb('#EAB308', '#4EDEA3')((pct - 0.5) * 2)
}

function healthLevel(pct: number): string {
  if (pct >= 0.75) return 'verified'
  if (pct >= 0.50) return 'partial'
  return 'critical'
}

function nodePulseClass(pct: number): string {
  if (pct >= 0.75) return 'graph-node-verified'
  if (pct < 0.25) return 'graph-node-critical'
  return ''
}

export function ForceGraph({ data, width, height, onNodeClick }: ForceGraphProps) {
  const svgRef = useRef<SVGSVGElement>(null)
  const containerRef = useRef<SVGGElement>(null)
  const zoomRef = useRef<d3.ZoomBehavior<SVGSVGElement, unknown> | null>(null)
  const simulationRef = useRef<d3.Simulation<SimNode, SimEdge> | null>(null)
  const [tooltip, setTooltip] = useState<TooltipState | null>(null)
  const hoverTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Zoom controls
  const zoomIn = useCallback(() => {
    if (!svgRef.current || !zoomRef.current) return
    d3.select(svgRef.current).transition().duration(300).call(zoomRef.current.scaleBy, 1.4)
  }, [])

  const zoomOut = useCallback(() => {
    if (!svgRef.current || !zoomRef.current) return
    d3.select(svgRef.current).transition().duration(300).call(zoomRef.current.scaleBy, 0.7)
  }, [])

  const zoomReset = useCallback(() => {
    if (!svgRef.current || !zoomRef.current) return
    d3.select(svgRef.current).transition().duration(500).call(zoomRef.current.transform, d3.zoomIdentity)
  }, [])

  useEffect(() => {
    if (!svgRef.current || !containerRef.current) return

    const svg = d3.select(svgRef.current)
    const g = d3.select(containerRef.current)

    // Clear previous content
    g.selectAll('*').remove()

    // Clone data to avoid mutating props
    const nodes: SimNode[] = data.nodes.map((n) => ({ ...n, x: 0, y: 0, fx: null, fy: null, vx: 0, vy: 0, index: 0 }))
    const edges: SimEdge[] = data.edges.map((e) => ({
      source: nodes.find((n) => n.id === e.source)!,
      target: nodes.find((n) => n.id === e.target)!,
    }))

    // Charter clustering positions
    const charters = [...new Set(nodes.map((n) => n.charter))]
    const charterAngle = (charter: string) => {
      const idx = charters.indexOf(charter)
      return (idx / charters.length) * 2 * Math.PI
    }
    const clusterRadius = Math.min(width, height) * 0.25

    // Force simulation
    const simulation = d3.forceSimulation<SimNode>(nodes)
      .force('link', d3.forceLink<SimNode, SimEdge>(edges).id((d) => d.id).distance(100))
      .force('charge', d3.forceManyBody().strength(-200))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collide', d3.forceCollide(30))
      .force('clusterX', d3.forceX<SimNode>((d) =>
        width / 2 + clusterRadius * Math.cos(charterAngle(d.charter))
      ).strength(0.15))
      .force('clusterY', d3.forceY<SimNode>((d) =>
        height / 2 + clusterRadius * Math.sin(charterAngle(d.charter))
      ).strength(0.15))

    simulationRef.current = simulation

    // Edge elements
    const edgeGroup = g.append('g').attr('class', 'edges')
    const edgeElements = edgeGroup
      .selectAll('line')
      .data(edges)
      .join('line')
      .attr('stroke', '#555')
      .attr('stroke-width', 1.5)
      .attr('marker-end', 'url(#arrowhead)')

    // Node elements
    const nodeGroup = g.append('g').attr('class', 'nodes')
    const nodeElements = nodeGroup
      .selectAll<SVGCircleElement, SimNode>('circle')
      .data(nodes)
      .join('circle')
      .attr('r', (d) => Math.max(16, Math.sqrt(d.req_count || 1) * 8))
      .attr('fill', (d) => {
        const color = d3.color(healthScale(d.verified_pct))
        if (color) color.opacity = 0.85
        return color?.formatRgb() ?? healthScale(d.verified_pct)
      })
      .attr('stroke', (d) => healthScale(d.verified_pct))
      .attr('stroke-width', 2)
      .attr('filter', (d) => `url(#glow-${healthLevel(d.verified_pct)})`)
      .attr('class', (d) => nodePulseClass(d.verified_pct))
      .attr('cursor', 'pointer')
      .attr('role', 'button')
      .attr('tabindex', 0)
      .attr('aria-label', (d) => `${d.label}: ${Math.round(d.verified_pct * 100)}% verified, ${d.status}`)

    // Label elements
    const labelGroup = g.append('g').attr('class', 'labels')
    const labelElements = labelGroup
      .selectAll<SVGTextElement, SimNode>('text')
      .data(nodes)
      .join('text')
      .text((d) => d.label)
      .attr('fill', '#bbcabf')  // on-surface-variant
      .attr('font-size', '10px')
      .attr('font-family', "'JetBrains Mono', monospace")
      .attr('text-anchor', 'middle')
      .attr('dy', (d) => Math.max(16, Math.sqrt(d.req_count || 1) * 8) + 14)
      .attr('pointer-events', 'none')
      .attr('letter-spacing', '0.02em')

    // Drag behavior with 3px threshold
    let dragStartX = 0
    let dragStartY = 0
    let isDragging = false

    const drag = d3.drag<SVGCircleElement, SimNode>()
      .on('start', (event, d) => {
        dragStartX = event.x
        dragStartY = event.y
        isDragging = false
        if (!event.active) simulation.alphaTarget(0.3).restart()
        d.fx = d.x
        d.fy = d.y
      })
      .on('drag', (event, d) => {
        const dx = event.x - dragStartX
        const dy = event.y - dragStartY
        if (!isDragging && Math.sqrt(dx * dx + dy * dy) > 3) {
          isDragging = true
        }
        if (isDragging) {
          d.fx = event.x
          d.fy = event.y
        }
      })
      .on('end', (event, d) => {
        if (!event.active) simulation.alphaTarget(0)
        if (!isDragging) {
          d.fx = null
          d.fy = null
          const slug = d.id.includes(':') ? d.id.split(':')[1] : d.label
          onNodeClick(d.charter, slug)
        }
      })

    nodeElements.call(drag)

    // Double-click to release pinned node
    nodeElements.on('dblclick', (_event, d) => {
      d.fx = null
      d.fy = null
      simulation.alphaTarget(0.1).restart()
      setTimeout(() => simulation.alphaTarget(0), 500)
    })

    // Hover tooltip with 300ms delay
    nodeElements
      .on('mouseenter', (event, d) => {
        if (hoverTimeout.current) clearTimeout(hoverTimeout.current)
        hoverTimeout.current = setTimeout(() => {
          const svgRect = svgRef.current?.getBoundingClientRect()
          if (!svgRect) return
          setTooltip({
            node: d,
            x: event.clientX - svgRect.left + 12,
            y: event.clientY - svgRect.top + 12,
          })
        }, 300)
      })
      .on('mousemove', (event) => {
        if (!tooltip) return
        const svgRect = svgRef.current?.getBoundingClientRect()
        if (!svgRect) return
        setTooltip((prev) =>
          prev ? { ...prev, x: event.clientX - svgRect.left + 12, y: event.clientY - svgRect.top + 12 } : null
        )
      })
      .on('mouseleave', () => {
        if (hoverTimeout.current) clearTimeout(hoverTimeout.current)
        setTooltip(null)
      })

    // Keyboard navigation on nodes
    nodeElements.on('keydown', (event: KeyboardEvent, d) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault()
        const slug = d.id.includes(':') ? d.id.split(':')[1] : d.label
        onNodeClick(d.charter, slug)
      }
    })

    // Zoom behavior
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.3, 3.0])
      .on('zoom', (event) => {
        g.attr('transform', event.transform)
      })

    svg.call(zoom)
    zoomRef.current = zoom

    // Tick
    simulation.on('tick', () => {
      edgeElements
        .attr('x1', (d) => d.source.x)
        .attr('y1', (d) => d.source.y)
        .attr('x2', (d) => d.target.x)
        .attr('y2', (d) => d.target.y)

      nodeElements
        .attr('cx', (d) => d.x)
        .attr('cy', (d) => d.y)

      labelElements
        .attr('x', (d) => d.x)
        .attr('y', (d) => d.y)
    })

    return () => {
      simulation.stop()
      if (hoverTimeout.current) clearTimeout(hoverTimeout.current)
    }
  }, [data, width, height, onNodeClick])

  return (
    <div className="relative w-full h-full">
      <svg
        ref={svgRef}
        viewBox={`0 0 ${width} ${height}`}
        className="w-full h-full rounded-lg"
        style={{
          backgroundColor: '#0E0E0E',
          backgroundImage: 'radial-gradient(circle, #1a1a1a 1px, transparent 1px)',
          backgroundSize: '24px 24px',
        }}
        aria-label={`Spec dependency graph with ${data.nodes.length} nodes and ${data.edges.length} connections`}
      >
        <defs>
          {/* Arrowhead marker */}
          <marker
            id="arrowhead"
            viewBox="0 0 10 10"
            refX={28}
            refY={5}
            markerWidth={6}
            markerHeight={6}
            orient="auto-start-reverse"
          >
            <path d="M 0 0 L 10 5 L 0 10 Z" fill="#555" />
          </marker>

          {/* Glow filters — palette-matched colors */}
          {/* Verified: primary #4EDEA3 */}
          <filter id="glow-verified" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur in="SourceGraphic" stdDeviation="3" result="blur" />
            <feColorMatrix
              in="blur"
              type="matrix"
              values="0 0 0 0 0.306  0 0 0 0 0.871  0 0 0 0 0.639  0 0 0 0.4 0"
            />
            <feMerge>
              <feMergeNode />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
          {/* Partial: amber #EAB308 */}
          <filter id="glow-partial" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur in="SourceGraphic" stdDeviation="3" result="blur" />
            <feColorMatrix
              in="blur"
              type="matrix"
              values="0 0 0 0 0.918  0 0 0 0 0.702  0 0 0 0 0.031  0 0 0 0.4 0"
            />
            <feMerge>
              <feMergeNode />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
          {/* Critical: error #FFB4AB */}
          <filter id="glow-critical" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur in="SourceGraphic" stdDeviation="3" result="blur" />
            <feColorMatrix
              in="blur"
              type="matrix"
              values="0 0 0 0 1.0  0 0 0 0 0.706  0 0 0 0 0.671  0 0 0 0.4 0"
            />
            <feMerge>
              <feMergeNode />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        <g ref={containerRef} />
      </svg>

      {/* Tooltip — glass-card style */}
      {tooltip && (
        <div
          className="absolute pointer-events-none z-50 glass-card rounded-lg px-3 py-2.5 min-w-[180px]"
          style={{
            left: tooltip.x,
            top: tooltip.y,
            boxShadow: '0 8px 24px rgba(0,0,0,0.4)',
          }}
        >
          <div className="font-headline font-semibold text-on-surface text-sm mb-1.5">{tooltip.node.label}</div>
          <div className="flex flex-col gap-0.5">
            <span className="text-[10px] font-mono uppercase tracking-widest text-on-surface-variant">
              Status: <span className="text-on-surface">{tooltip.node.status}</span>
            </span>
            <span className="text-[10px] font-mono uppercase tracking-widest text-on-surface-variant">
              Verified: <span className="text-primary">{Math.round(tooltip.node.verified_pct * 100)}%</span>
            </span>
            <span className="text-[10px] font-mono uppercase tracking-widest text-on-surface-variant">
              Reqs: <span className="text-on-surface">{tooltip.node.req_count}</span>
            </span>
            <span className="text-[10px] font-mono uppercase tracking-widest text-outline">
              {tooltip.node.charter}
            </span>
          </div>
        </div>
      )}

      {/* Zoom controls — glass pill */}
      <div className="absolute bottom-4 right-4 flex items-center glass-card rounded-full overflow-hidden">
        <button
          onClick={zoomIn}
          className="px-3 py-2 text-on-surface-variant hover:text-on-surface hover:bg-white/5 transition-all duration-150 cursor-pointer"
          aria-label="Zoom in"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" /><line x1="11" y1="8" x2="11" y2="14" /><line x1="8" y1="11" x2="14" y2="11" />
          </svg>
        </button>
        <div style={{ width: 1, height: 16, background: 'rgba(255,255,255,0.08)' }} />
        <button
          onClick={zoomOut}
          className="px-3 py-2 text-on-surface-variant hover:text-on-surface hover:bg-white/5 transition-all duration-150 cursor-pointer"
          aria-label="Zoom out"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" /><line x1="8" y1="11" x2="14" y2="11" />
          </svg>
        </button>
        <div style={{ width: 1, height: 16, background: 'rgba(255,255,255,0.08)' }} />
        <button
          onClick={zoomReset}
          className="px-3 py-2 text-on-surface-variant hover:text-on-surface hover:bg-white/5 transition-all duration-150 cursor-pointer"
          aria-label="Reset zoom"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="15 3 21 3 21 9" /><polyline points="9 21 3 21 3 15" /><line x1="21" y1="3" x2="14" y2="10" /><line x1="3" y1="21" x2="10" y2="14" />
          </svg>
        </button>
      </div>
    </div>
  )
}
