import { type VariantProps, cva } from 'class-variance-authority'
import type { HTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

const badgeVariants = cva(
  'inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium',
  {
    variants: {
      variant: {
        default: 'border-slate-200 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300',
        secondary: 'border-transparent bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-300',
        outline: 'border-slate-300 dark:border-slate-600 text-slate-700 dark:text-slate-300',
        critical: 'border-rose-500/20 bg-rose-500/10 text-rose-500',
        high: 'border-orange-500/20 bg-orange-500/10 text-orange-500',
        medium: 'border-amber-500/20 bg-amber-500/10 text-amber-500',
        low: 'border-blue-500/20 bg-blue-500/10 text-blue-500',
        info: 'border-slate-500/20 bg-slate-500/10 text-slate-400',
        get: 'border-emerald-500/20 bg-emerald-500/10 text-emerald-400',
        post: 'border-blue-500/20 bg-blue-500/10 text-blue-400',
        put: 'border-amber-500/20 bg-amber-500/10 text-amber-400',
        delete: 'border-rose-500/20 bg-rose-500/10 text-rose-400',
        patch: 'border-violet-500/20 bg-violet-500/10 text-violet-400',
      },
    },
    defaultVariants: { variant: 'default' },
  },
)

export type BadgeVariant = NonNullable<VariantProps<typeof badgeVariants>['variant']>

export function Badge({
  className,
  variant,
  ...props
}: HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />
}

export function severityVariant(severity: string): BadgeVariant {
  switch (severity.toLowerCase()) {
    case 'critical': return 'critical'
    case 'high': return 'high'
    case 'medium': return 'medium'
    case 'low': return 'low'
    case 'info': return 'info'
    default: return 'secondary'
  }
}
