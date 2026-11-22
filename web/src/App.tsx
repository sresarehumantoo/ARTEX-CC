import { lazy, Suspense, useCallback, useEffect, useState } from 'react'
import { Routes, Route } from 'react-router-dom'
import { CommandPalette } from './components/CommandPalette'
import { ConnectionBanner } from './components/ConnectionBanner'
import { ErrorBoundary } from './components/ErrorBoundary'
import { ShortcutsHelp } from './components/ShortcutsHelp'
import { Sidebar } from './components/Sidebar'
import { Spinner } from './components/ui/spinner'
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts'
import { useDocumentTitle } from './hooks/useDocumentTitle'
import { useSSERefresh } from './hooks/useSSERefresh'
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

function PageLoader() {
  return (
    <div className="flex items-center justify-center h-64">
      <Spinner className="h-8 w-8" />
    </div>
  )
}

export default function App() {
  useSSERefresh()
  useKeyboardShortcuts()
  useDocumentTitle()

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
      <Sidebar />
      <main className="flex-1 overflow-y-auto">
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
