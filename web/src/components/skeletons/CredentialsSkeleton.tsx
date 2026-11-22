import { Skeleton } from '../ui/skeleton'

export function CredentialsSkeleton() {
  return (
    <div className="p-4 space-y-4">
      <Skeleton className="h-9 w-64 rounded-md" />
      <div className="space-y-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex items-center gap-4">
            <Skeleton className="h-5 w-32" />
            <Skeleton className="h-5 w-24" />
            <Skeleton className="h-5 w-20" />
            <Skeleton className="h-5 w-28" />
            <Skeleton className="h-5 w-36" />
          </div>
        ))}
      </div>
    </div>
  )
}
