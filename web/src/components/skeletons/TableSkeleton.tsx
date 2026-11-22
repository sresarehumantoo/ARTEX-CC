import { Skeleton } from '../ui/skeleton'

export function TableSkeleton({
  rows = 8,
  columns = 5,
  filterBar = false,
}: {
  rows?: number
  columns?: number
  filterBar?: boolean
}) {
  return (
    <div className="space-y-0">
      {filterBar && (
        <div className="flex items-center gap-3 border-b border-border px-4 py-2">
          <Skeleton className="h-9 w-44" />
          <Skeleton className="h-9 w-28" />
          <Skeleton className="h-9 w-36" />
        </div>
      )}

      <div className="w-full">
        <div className="flex gap-2 border-b border-border px-3 py-3">
          {Array.from({ length: columns }).map((_, i) => (
            <Skeleton
              key={i}
              className="h-3"
              style={{ flex: i === 0 ? '0 0 80px' : '1' }}
            />
          ))}
        </div>

        {Array.from({ length: rows }).map((_, row) => (
          <div
            key={row}
            className="flex items-center gap-2 border-b border-border px-3 py-3"
          >
            {Array.from({ length: columns }).map((_, col) => (
              <Skeleton
                key={col}
                className="h-4"
                style={{ flex: col === 0 ? '0 0 80px' : '1' }}
              />
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
