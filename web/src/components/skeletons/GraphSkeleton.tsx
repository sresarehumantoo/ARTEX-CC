import { Skeleton } from '../ui/skeleton'

export function GraphSkeleton() {
  return (
    <div className="h-full w-full relative bg-muted overflow-hidden">
      <svg className="absolute inset-0 w-full h-full" aria-hidden="true">
        <line
          x1="20%" y1="25%" x2="50%" y2="15%"
          className="stroke-slate-200 dark:stroke-slate-800 animate-pulse"
          strokeWidth="2"
        />
        <line
          x1="50%" y1="15%" x2="75%" y2="30%"
          className="stroke-slate-200 dark:stroke-slate-800 animate-pulse"
          strokeWidth="2"
        />
        <line
          x1="20%" y1="25%" x2="35%" y2="55%"
          className="stroke-slate-200 dark:stroke-slate-800 animate-pulse"
          strokeWidth="2"
        />
        <line
          x1="50%" y1="15%" x2="55%" y2="50%"
          className="stroke-slate-200 dark:stroke-slate-800 animate-pulse"
          strokeWidth="2"
        />
        <line
          x1="75%" y1="30%" x2="65%" y2="65%"
          className="stroke-slate-200 dark:stroke-slate-800 animate-pulse"
          strokeWidth="2"
        />
        <line
          x1="35%" y1="55%" x2="55%" y2="50%"
          className="stroke-slate-200 dark:stroke-slate-800 animate-pulse"
          strokeWidth="2"
        />
      </svg>

      <Skeleton className="absolute rounded-lg" style={{ left: '17%', top: '20%', width: 120, height: 48 }} />
      <Skeleton className="absolute rounded-lg" style={{ left: '45%', top: '10%', width: 140, height: 48 }} />
      <Skeleton className="absolute rounded-lg" style={{ left: '70%', top: '25%', width: 110, height: 48 }} />
      <Skeleton className="absolute rounded-lg" style={{ left: '30%', top: '50%', width: 130, height: 48 }} />
      <Skeleton className="absolute rounded-lg" style={{ left: '50%', top: '45%', width: 100, height: 48 }} />
      <Skeleton className="absolute rounded-lg" style={{ left: '60%', top: '60%', width: 120, height: 48 }} />
    </div>
  )
}
