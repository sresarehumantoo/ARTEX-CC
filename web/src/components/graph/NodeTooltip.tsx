import { cn } from '../../lib/utils'

export function NodeTooltip({ lines, className }: { lines: string[]; className?: string }) {
  return (
    <div className={cn(
      'absolute -top-2 left-1/2 -translate-x-1/2 -translate-y-full',
      'invisible group-hover:visible opacity-0 group-hover:opacity-100',
      'transition-opacity duration-150 pointer-events-none z-50',
      'max-w-[250px] rounded-md bg-slate-900 dark:bg-slate-700 px-2.5 py-1.5 shadow-lg',
      className,
    )}>
      {lines.map((line, i) => (
        <p key={i} className="text-[11px] text-slate-100 leading-relaxed truncate">{line}</p>
      ))}
      <div className="absolute left-1/2 -translate-x-1/2 top-full w-0 h-0 border-x-[5px] border-x-transparent border-t-[5px] border-t-slate-900 dark:border-t-slate-700" />
    </div>
  )
}
