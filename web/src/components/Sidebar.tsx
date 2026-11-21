import { NavLink } from 'react-router-dom'
import { LayoutDashboard, GitBranch, Shield, Globe, Server } from 'lucide-react'
import { cn } from '../lib/utils'

const links = [
  { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/graph', icon: GitBranch, label: 'Graph' },
  { to: '/findings', icon: Shield, label: 'Findings' },
  { to: '/traffic', icon: Globe, label: 'Traffic' },
  { to: '/assets', icon: Server, label: 'Assets' },
] as const

export function Sidebar() {
  return (
    <aside className="flex h-screen w-14 md:w-56 flex-col bg-slate-950 border-r border-slate-800">
      <div className="flex items-center gap-2 px-3 py-5 md:px-5">
        <Shield className="h-6 w-6 shrink-0 text-emerald-500" />
        <span className="hidden md:block text-lg font-semibold tracking-tight text-slate-100">
          cc-artex
        </span>
      </div>
      <nav className="flex-1 space-y-1 px-2 py-2 md:px-3">
        {links.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'bg-slate-800 text-slate-100'
                  : 'text-slate-400 hover:bg-slate-800/50 hover:text-slate-200',
              )
            }
          >
            <Icon className="h-4 w-4 shrink-0" />
            <span className="hidden md:inline">{label}</span>
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
