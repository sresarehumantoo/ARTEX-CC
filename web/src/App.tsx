import { lazy, Suspense, useCallback, useEffect, useState, type ReactNode } from 'react'
import { Routes, Route, useLocation, useNavigate } from 'react-router-dom'
import { Menu } from 'lucide-react'
import { CommandPalette } from './components/CommandPalette'
import { ConnectionBanner } from './components/ConnectionBanner'
import { ErrorBoundary } from './components/ErrorBoundary'
import { ShortcutsHelp } from './components/ShortcutsHelp'
import { Sidebar } from './components/Sidebar'
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts'
import { useDocumentTitle } from './hooks/useDocumentTitle'
import { useSSERefresh } from './hooks/useSSERefresh'
import { AUTH_REQUIRED_EVENT, checkAuthStatus } from './lib/auth'
import { Dashboard } from './pages/Dashboard'

const Graph = lazy(() => import('./pages/Graph'))
const Findings = lazy(() => import('./pages/Findings'))
const Traffic = lazy(() => import('./pages/Traffic'))
const Activity = lazy(() => import('./pages/Activity'))
const Assets = lazy(() => import('./pages/Assets'))
const Coverage = lazy(() => import('./pages/Coverage'))
const Report = lazy(() => import('./pages/Report'))
const Credentials = lazy(() => import('./pages/Credentials'))
const Search = lazy(() => import('./pages/Search'))
const Settings = lazy(() => import('./pages/Settings'))
const NotFound = lazy(() => import('./pages/NotFound'))
const Login = lazy(() => import('./pages/Login'))
const Setup = lazy(() => import('./pages/Setup'))

function PageLoader() {
  return (
    <div className="p-6 space-y-4 animate-pulse">
      <div className="h-6 w-48 rounded bg-slate-200 dark:bg-slate-700" />
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        {[1, 2, 3, 4].map(i => (
          <div key={i} className="h-24 rounded-lg bg-slate-200 dark:bg-slate-700" />
        ))}
      </div>
      <div className="h-64 rounded-lg bg-slate-200 dark:bg-slate-700" />
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

  useEffect(() => {
    window.addEventListener('toggle-shortcuts-help', toggleHelp)
    return () => window.removeEventListener('toggle-shortcuts-help', toggleHelp)
  }, [toggleHelp])

  useEffect(() => {
    window.addEventListener('toggle-command-palette', togglePalette)
    return () => window.removeEventListener('toggle-command-palette', togglePalette)
  }, [togglePalette])

  return (
    <div className="flex h-screen bg-white dark:bg-slate-900">
      <Sidebar mobileOpen={mobileMenuOpen} onMobileClose={() => setMobileMenuOpen(false)} />
      <main className="flex-1 overflow-y-auto">
        <button
          type="button"
          className="md:hidden flex items-center gap-2 px-3 py-2 border-b border-slate-200 dark:border-slate-800 w-full bg-slate-50 dark:bg-slate-950 text-slate-600 dark:text-slate-400"
          onClick={() => setMobileMenuOpen(true)}
          aria-label="Open menu"
        >
          <Menu className="h-5 w-5" />
          <span className="text-sm font-medium">Menu</span>
        </button>
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
              <Route path="/search" element={<Search />} />
              <Route path="/settings" element={<Settings />} />
              <Route path="*" element={<NotFound />} />
            </Routes>
          </Suspense>
        </ErrorBoundary>
      </main>
      <ShortcutsHelp open={showHelp} onClose={() => setShowHelp(false)} />
      <CommandPalette open={showPalette} onClose={() => setShowPalette(false)} />
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
