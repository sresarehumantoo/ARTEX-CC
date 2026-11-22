import { Skeleton } from '../ui/skeleton'

export function SettingsSkeleton() {
  return (
    <div className="p-6 space-y-6">
      <Skeleton className="h-12 rounded-lg" />
      <Skeleton className="h-32 rounded-lg" />
      <Skeleton className="h-12 rounded-lg" />
      <Skeleton className="h-12 rounded-lg" />
    </div>
  )
}
