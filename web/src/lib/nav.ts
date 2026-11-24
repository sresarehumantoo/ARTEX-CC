import {
  BarChart3,
  Bot,
  ClipboardList,
  Clock,
  FileText,
  FolderOpen,
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
  Terminal,
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
    label: 'Operations',
    items: [
      { to: '/tasks', icon: ClipboardList, label: 'Tasks' },
      { to: '/commands', icon: Terminal, label: 'Commands' },
      { to: '/usage', icon: BarChart3, label: 'Usage' },
      { to: '/workspace', icon: FolderOpen, label: 'Workspace' },
    ],
  },
  {
    label: 'System',
    items: [
      { to: '/agents', icon: Bot, label: 'Agents' },
      { to: '/mcp-servers', icon: Plug, label: 'MCP' },
      { to: '/tools', icon: Wrench, label: 'Tools' },
      { to: '/intercept', icon: Radio, label: 'Intercept' },
      { to: '/logs', icon: ScrollText, label: 'Logs' },
      { to: '/settings', icon: SettingsIcon, label: 'Settings' },
    ],
  },
]
