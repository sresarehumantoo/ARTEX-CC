import { lazy, Suspense, useCallback, useEffect, useState, type ReactNode } from 'react'
import { Routes, Route, useLocation, useNavigate } from 'react-router-dom'
import { CommandPalette } from './components/CommandPalette'
import { ConnectionBanner } from './components/ConnectionBanner'
import { ErrorBoundary } from './components/ErrorBoundary'
import { ShortcutsHelp } from './components/ShortcutsHelp'
import { Sidebar } from './components/Sidebar'
import { Header } from './components/layout/Header'
import { Toaster } from './components/ui/sonner'
import { usePreferences } from './stores/preferences'
import { cn } from './lib/utils'
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts'
import { useDocumentTitle } from './hooks/useDocumentTitle'
import { useSSERefresh } from './hooks/useSSERefresh'
import { AUTH_REQUIRED_EVENT, checkAuthStatus } from './lib/auth'
import { Dashboard } from './pages/Dashboard'
import { DemoBanner } from './demo/DemoBanner'

const Tasks = lazy(() => import('./pages/Tasks'))
const Commands = lazy(() => import('./pages/Commands'))
const Usage = lazy(() => import('./pages/Usage'))
const Workspace = lazy(() => import('./pages/Workspace'))
const Graph = lazy(() => import('./pages/Graph'))
const Findings = lazy(() => import('./pages/Findings'))
const Traffic = lazy(() => import('./pages/Traffic'))
const Activity = lazy(() => import('./pages/Activity'))
const Assets = lazy(() => import('./pages/Assets'))
const Coverage = lazy(() => import('./pages/Coverage'))
const Report = lazy(() => import('./pages/Report'))
const Credentials = lazy(() => import('./pages/Credentials'))
const Search = lazy(() => import('./pages/Search'))
const Chat = lazy(() => import('./pages/Chat'))
const Agents = lazy(() => import('./pages/Agents'))
const Mcp = lazy(() => import('./pages/Mcp'))
const Tools = lazy(() => import('./pages/Tools'))
const Intercept = lazy(() => import('./pages/Intercept'))
const Logs = lazy(() => import('./pages/Logs'))
const Settings = lazy(() => import('./pages/Settings'))
const NotFound = lazy(() => import('./pages/NotFound'))
const Login = lazy(() => import('./pages/Login'))
const Setup = lazy(() => import('./pages/Setup'))

function PageLoader() {
  return (
    <div className="p-6 space-y-4 animate-pulse">
      <div className="h-6 w-48 rounded bg-muted" />
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[1, 2, 3, 4].map(i => (
          <div key={i} className="h-24 rounded-lg bg-muted" />
        ))}
      </div>
      <div className="h-64 rounded-lg bg-muted" />
    </div>
  )
}

function MainLayout() {
  useSSERefresh()
  useKeyboardShortcuts()
  useDocumentTitle()

  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const [showHelp, setShowHelp] = useState(false)
  const [showPalette, setShowPalette] = useState(false)
  const toggleHelp = useCallback(() => setShowHelp(v => !v), [])
  const togglePalette = useCallback(() => setShowPalette(v => !v), [])
  const { sidebar_collapsed, setSidebarCollapsed, content_layout } = usePreferences()

  const toggleSidebar = useCallback(() => {
    if (window.matchMedia('(min-width: 768px)').matches) setSidebarCollapsed(!sidebar_collapsed)
    else setMobileMenuOpen(v => !v)
  }, [sidebar_collapsed, setSidebarCollapsed])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'b') {
        e.preventDefault()
        toggleSidebar()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [toggleSidebar])

  useEffect(() => {
    window.addEventListener('toggle-shortcuts-help', toggleHelp)
    return () => window.removeEventListener('toggle-shortcuts-help', toggleHelp)
  }, [toggleHelp])

  useEffect(() => {
    window.addEventListener('toggle-command-palette', togglePalette)
    return () => window.removeEventListener('toggle-command-palette', togglePalette)
  }, [togglePalette])

  return (
    <div className="flex h-screen bg-background text-foreground">
      <Sidebar mobileOpen={mobileMenuOpen} onMobileClose={() => setMobileMenuOpen(false)} />
      <div className="flex min-w-0 flex-1 flex-col">
        {import.meta.env.MODE === 'demo' && <DemoBanner />}
        <Header onToggleSidebar={toggleSidebar} onOpenPalette={() => setShowPalette(true)} />
        <main className={cn('flex-1 overflow-y-auto', content_layout === 'centered' && 'mx-auto w-full max-w-screen-2xl')}>
          <ConnectionBanner />
          <ErrorBoundary>
            <Suspense fallback={<PageLoader />}>
              <Routes>
                <Route path="/" element={<Dashboard />} />
                <Route path="/graph" element={<Graph />} />
                <Route path="/findings" element={<Findings />} />
                <Route path="/traffic" element={<Traffic />} />
                <Route path="/activity" element={<Activity />} />
                <Route path="/assets" element={<Assets />} />
                <Route path="/coverage" element={<Coverage />} />
                <Route path="/credentials" element={<Credentials />} />
                <Route path="/report" element={<Report />} />
                <Route path="/tasks" element={<Tasks />} />
                <Route path="/chat" element={<Chat />} />
                <Route path="/agents" element={<Agents />} />
                <Route path="/mcp-servers" element={<Mcp />} />
                <Route path="/tools" element={<Tools />} />
                <Route path="/intercept" element={<Intercept />} />
                <Route path="/commands" element={<Commands />} />
                <Route path="/usage" element={<Usage />} />
                <Route path="/workspace" element={<Workspace />} />
                <Route path="/logs" element={<Logs />} />
                <Route path="/search" element={<Search />} />
                <Route path="/settings" element={<Settings />} />
                <Route path="*" element={<NotFound />} />
              </Routes>
            </Suspense>
          </ErrorBoundary>
        </main>
      </div>
      <ShortcutsHelp open={showHelp} onClose={() => setShowHelp(false)} />
      <CommandPalette open={showPalette} onClose={() => setShowPalette(false)} />
      <Toaster />
    </div>
  )
}

function AuthGate({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const location = useLocation()
  const path = location.pathname

  useEffect(() => {
    if (path === '/login' || path === '/setup') return
    let cancelled = false
    checkAuthStatus()
      .then(({ initialized, auth_required }) => {
        if (!cancelled && auth_required && !initialized) navigate('/setup', { replace: true })
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [path, navigate])

  useEffect(() => {
    const onUnauthorized = () => {
      if (path === '/login' || path === '/setup') return
      navigate('/login', { replace: true, state: { from: path + location.search } })
    }
    window.addEventListener(AUTH_REQUIRED_EVENT, onUnauthorized)
    return () => window.removeEventListener(AUTH_REQUIRED_EVENT, onUnauthorized)
  }, [path, location.search, navigate])

  return children
}

export default function App() {
  return (
    <AuthGate>
      <Suspense fallback={<PageLoader />}>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route path="/setup" element={<Setup />} />
          <Route path="*" element={<MainLayout />} />
        </Routes>
      </Suspense>
    </AuthGate>
  )
}
