import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { cn } from '../lib/utils'

interface FindingsTimelineProps {
  findings: Array<{
    id: number
    severity: string
    vulnclass: string
    created_at?: string
  }>
  className?: string
}

const SEVERITY_COLORS: Record<string, string> = {
  critical: '#ef4444',
  high: '#f97316',
  medium: '#eab308',
  low: '#3b82f6',
  info: '#6b7280',
}

const SEVERITY_LABELS: Record<string, string> = {
  critical: 'Critical',
  high: 'High',
  medium: 'Medium',
  low: 'Low',
  info: 'Info',
}

function seededJitter(id: number): number {
  let h = id * 2654435761
  h = ((h >>> 16) ^ h) * 0x45d9f3b
  h = ((h >>> 16) ^ h) * 0x45d9f3b
  h = (h >>> 16) ^ h
  return ((h & 0xffff) / 0xffff) * 2 - 1
}

function formatTickLabel(date: Date, rangeMs: number): string {
  if (rangeMs < 86400000) {
    return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  }
  if (rangeMs < 86400000 * 30) {
    return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  }
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: '2-digit' })
}

function formatTooltipTime(date: Date): string {
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function niceTickStep(rawRange: number): number {
  const niceSteps = [
    60000, 300000, 600000, 1800000, 3600000,
    7200000, 14400000, 43200000, 86400000,
    172800000, 604800000, 2592000000,
  ]
  const rawStep = rawRange / 4
  return niceSteps.find(s => s >= rawStep) ?? rawStep
}

export default function FindingsTimeline({ findings, className }: FindingsTimelineProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState({ w: 0, h: 0 })

  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    const ro = new ResizeObserver(entries => {
      const { width, height } = entries[0].contentRect
      setSize({ w: width, h: height })
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const timeFindings = useMemo(() => {
    return findings
      .filter(f => f.created_at)
      .map(f => ({ ...f, ts: new Date(f.created_at!).getTime() }))
      .filter(f => !isNaN(f.ts))
      .sort((a, b) => a.ts - b.ts)
  }, [findings])

  const useRealTime = timeFindings.length >= 2
  const data = useMemo(() => {
    if (useRealTime) return timeFindings
    return [...findings]
      .sort((a, b) => a.id - b.id)
      .map((f, i) => ({ ...f, ts: i }))
  }, [findings, timeFindings, useRealTime])

  const margin = { top: 16, right: 24, bottom: 28, left: 24 }
  const dotRadius = 5
  const jitterRange = 14
  const plotW = Math.max(0, size.w - margin.left - margin.right)
  const plotH = Math.max(0, size.h - margin.top - margin.bottom)
  const centerY = margin.top + plotH / 2
  const axisY = size.h - margin.bottom

  const minTs = data.length > 0 ? data[0].ts : 0
  const maxTs = data.length > 0 ? data[data.length - 1].ts : 0
  const range = maxTs - minTs

  const xOf = useCallback(
    (ts: number) => {
      if (range === 0) return margin.left + plotW / 2
      return margin.left + ((ts - minTs) / range) * plotW
    },
    [minTs, range, margin.left, plotW],
  )

  const yOf = useCallback(
    (id: number) => centerY + seededJitter(id) * jitterRange,
    [centerY, jitterRange],
  )

  const ticks = useMemo(() => {
    if (data.length < 2) return []
    if (useRealTime && range > 0) {
      const step = niceTickStep(range)
      const values: number[] = []
      let t = Math.ceil(minTs / step) * step
      while (t <= maxTs) {
        values.push(t)
        t += step
      }
      if (values.length === 0) return [minTs, maxTs]
      if (values.length === 1) {
        if (values[0] > minTs) values.unshift(minTs)
        else values.push(maxTs)
      }
      return values
    }
    const count = Math.min(data.length, 5)
    return Array.from({ length: count }, (_, i) => {
      const idx = Math.round((i / (count - 1)) * (data.length - 1))
      return data[idx].ts
    })
  }, [data, useRealTime, range, minTs, maxTs])

  const [tooltip, setTooltip] = useState<{
    finding: (typeof data)[0]
    px: number
    py: number
  } | null>(null)

  const handleMouseMove = useCallback(
    (e: React.MouseEvent<SVGSVGElement>) => {
      if (data.length < 2 || plotW === 0) return
      const rect = e.currentTarget.getBoundingClientRect()
      const mx = e.clientX - rect.left
      const my = e.clientY - rect.top

      let closest: (typeof data)[0] | null = null
      let closestDist = Infinity
      for (const f of data) {
        const px = xOf(f.ts)
        const py = yOf(f.id)
        const dist = Math.hypot(mx - px, my - py)
        if (dist < 24 && dist < closestDist) {
          closestDist = dist
          closest = f
        }
      }

      if (closest) {
        setTooltip({ finding: closest, px: xOf(closest.ts), py: yOf(closest.id) })
      } else {
        setTooltip(null)
      }
    },
    [data, plotW, xOf, yOf],
  )

  const handleMouseLeave = useCallback(() => setTooltip(null), [])

  if (data.length < 2 || size.w === 0) {
    return <div ref={containerRef} className={cn('relative', className)} />
  }

  return (
    <div ref={containerRef} className={cn('relative', className)}>
      <svg
        className="h-full w-full"
        width={size.w}
        height={size.h}
        onMouseMove={handleMouseMove}
        onMouseLeave={handleMouseLeave}
      >
        <line
          x1={margin.left}
          y1={axisY}
          x2={size.w - margin.right}
          y2={axisY}
          className="stroke-slate-300 dark:stroke-slate-700"
          strokeWidth={1}
        />

        {ticks.map((t, i) => {
          const x = xOf(t)
          const label = useRealTime
            ? formatTickLabel(new Date(t), range)
            : `#${data[t]?.id ?? t}`
          return (
            <g key={i}>
              <line
                x1={x}
                y1={axisY}
                x2={x}
                y2={axisY + 4}
                className="stroke-slate-300 dark:stroke-slate-700"
                strokeWidth={1}
              />
              <text
                x={x}
                y={axisY + 16}
                textAnchor="middle"
                className="fill-slate-500 dark:fill-slate-500"
                fontSize={10}
              >
                {label}
              </text>
            </g>
          )
        })}

        {data.map(f => {
          const cx = xOf(f.ts)
          const cy = yOf(f.id)
          const color = SEVERITY_COLORS[f.severity] || '#6b7280'
          const isHovered = tooltip?.finding.id === f.id
          return (
            <circle
              key={f.id}
              cx={cx}
              cy={cy}
              r={isHovered ? dotRadius + 1 : dotRadius}
              fill={color}
              className="stroke-white dark:stroke-slate-900"
              strokeWidth={2}
            />
          )
        })}
      </svg>

      {tooltip && (
        <div
          className="pointer-events-none absolute z-10 rounded bg-slate-800 px-2 py-1 text-xs text-white shadow-lg dark:bg-slate-700"
          style={{
            left: tooltip.px,
            top: tooltip.py - 40,
            transform: 'translateX(-50%)',
          }}
        >
          <div>
            <span
              className="mr-1.5 inline-block h-2 w-2 rounded-full"
              style={{ backgroundColor: SEVERITY_COLORS[tooltip.finding.severity] || '#6b7280' }}
            />
            <span className="font-medium">
              {SEVERITY_LABELS[tooltip.finding.severity] || tooltip.finding.severity}
            </span>
            {' '}
            <span className="text-slate-300">{tooltip.finding.vulnclass}</span>
          </div>
          <div className="text-muted-foreground">
            {useRealTime ? formatTooltipTime(new Date(tooltip.finding.ts)) : `Finding #${tooltip.finding.id}`}
          </div>
        </div>
      )}
    </div>
  )
}
