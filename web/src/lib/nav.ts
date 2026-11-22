import {
  Bot,
  Clock,
  FileText,
  GitBranch,
  Globe,
  KeyRound,
  LayoutDashboard,
  MessageSquare,
  Plug,
  Radio,
  ScrollText,
  Server,
  Settings as SettingsIcon,
  Shield,
  Target,
  Wrench,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export interface NavItem {
  to: string
  icon: LucideIcon
  label: string
  badgeKey?: 'findings' | 'assets'
}

export interface NavGroup {
  label: string
  items: NavItem[]
}

export const navGroups: NavGroup[] = [
  {
    label: 'Functions',
    items: [
      { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
      { to: '/chat', icon: MessageSquare, label: 'Chat' },
      { to: '/graph', icon: GitBranch, label: 'Graph' },
      { to: '/findings', icon: Shield, label: 'Findings', badgeKey: 'findings' },
      { to: '/traffic', icon: Globe, label: 'Traffic' },
      { to: '/activity', icon: Clock, label: 'Activity' },
      { to: '/assets', icon: Server, label: 'Assets', badgeKey: 'assets' },
      { to: '/coverage', icon: Target, label: 'Coverage' },
      { to: '/credentials', icon: KeyRound, label: 'Credentials' },
      { to: '/report', icon: FileText, label: 'Report' },
    ],
  },
  {
    label: 'System',
    items: [
      { to: '/agents', icon: Bot, label: 'Agents' },
      { to: '/mcp', icon: Plug, label: 'MCP' },
      { to: '/tools', icon: Wrench, label: 'Tools' },
      { to: '/intercept', icon: Radio, label: 'Intercept' },
      { to: '/logs', icon: ScrollText, label: 'Logs' },
      { to: '/settings', icon: SettingsIcon, label: 'Settings' },
    ],
  },
]
