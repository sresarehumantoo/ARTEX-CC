import { useId } from 'react'

export function BrandMark({ className }: { className?: string }) {
  const id = useId()
  const bg = `${id}-bg`
  const edge = `${id}-edge`
  return (
    <svg viewBox="0 0 64 64" className={className} role="img" aria-label="ARTEX-CC">
      <defs>
        <linearGradient id={bg} x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#0f172a" />
          <stop offset="1" stopColor="#020617" />
        </linearGradient>
        <linearGradient id={edge} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#38bdf8" />
          <stop offset="1" stopColor="#22d3ee" />
        </linearGradient>
      </defs>
      <rect width="64" height="64" rx="14" fill={`url(#${bg})`} />
      <path
        d="M32 15 14.5 50M32 15 49.5 50M21.5 36h21"
        fill="none"
        stroke={`url(#${edge})`}
        strokeWidth="4"
        strokeLinecap="round"
      />
      <circle cx="32" cy="15" r="8.5" fill="none" stroke="#38bdf8" strokeWidth="2" opacity=".45" />
      <circle cx="32" cy="15" r="5" fill="#e0f2fe" />
      <circle cx="21.5" cy="36" r="4" fill="#38bdf8" />
      <circle cx="42.5" cy="36" r="4" fill="#38bdf8" />
      <circle cx="14.5" cy="50" r="4.5" fill="#22d3ee" />
      <circle cx="49.5" cy="50" r="4.5" fill="#22d3ee" />
    </svg>
  )
}
