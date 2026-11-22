import { Skeleton } from '../ui/skeleton'

export function CoverageSkeleton() {
  return (
    <div className="p-6 space-y-6">
      <Skeleton className="h-10 w-48 rounded-md" />
      <div className="grid grid-cols-4 gap-3">
        {Array.from({ length: 12 }).map((_, i) => (
          <Skeleton key={i} className="h-10 rounded-md" />
        ))}
      </div>
      <Skeleton className="h-10 w-48 rounded-md" />
      <div className="space-y-2">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-8 rounded-md" />
        ))}
      </div>
    </div>
  )
}
