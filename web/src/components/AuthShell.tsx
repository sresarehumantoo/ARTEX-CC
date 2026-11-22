import type { ReactNode } from 'react'
import { ShieldCheck } from 'lucide-react'

interface AuthShellProps {
  title: string
  description: string
  children: ReactNode
}

export function AuthShell({ title, description, children }: AuthShellProps) {
  return (
    <div className="flex h-dvh">
      <div className="hidden flex-col items-center justify-center bg-primary p-12 text-center lg:flex lg:w-1/3">
        <div className="relative flex items-center justify-center">
          <div className="absolute size-80 rounded-full border border-primary-foreground/10" />
          <div className="absolute size-60 rounded-full border border-primary-foreground/15" />
          <div className="absolute size-40 rounded-full border border-primary-foreground/20" />
          <ShieldCheck className="relative size-20 text-primary-foreground" />
        </div>
        <p className="relative mt-24 text-xl font-medium tracking-tight text-primary-foreground">cc-artex</p>
      </div>
      <div className="flex w-full items-center justify-center bg-background p-8 lg:w-2/3">
        <div className="w-full max-w-md space-y-10 py-24 lg:py-32">
          <div className="space-y-4 text-center">
            <h2 className="text-2xl font-medium tracking-tight">{title}</h2>
            <p className="mx-auto max-w-xl text-muted-foreground">{description}</p>
          </div>
          {children}
        </div>
      </div>
    </div>
  )
}
