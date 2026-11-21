import { lazy, Suspense, useCallback, useEffect, useState } from 'react'
import { Routes, Route } from 'react-router-dom'
import { ErrorBoundary } from './components/ErrorBoundary'
import { ShortcutsHelp } from './components/ShortcutsHelp'
import { Sidebar } from './components/Sidebar'
import { Spinner } from './components/ui/spinner'
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts'
import { useSSERefresh } from './hooks/useSSERefresh'
import { Dashboard } from './pages/Dashboard'

const Graph = lazy(() => import('./pages/Graph'))
const Findings = lazy(() => import('./pages/Findings'))
const Traffic = lazy(() => import('./pages/Traffic'))
const Assets = lazy(() => import('./pages/Assets'))

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

  const [showHelp, setShowHelp] = useState(false)
  const toggleHelp = useCallback(() => setShowHelp(v => !v), [])

  useEffect(() => {
    window.addEventListener('toggle-shortcuts-help', toggleHelp)
    return () => window.removeEventListener('toggle-shortcuts-help', toggleHelp)
  }, [toggleHelp])

  return (
    <div className="flex h-screen bg-slate-900">
      <Sidebar />
      <main className="flex-1 overflow-y-auto">
        <ErrorBoundary>
          <Suspense fallback={<PageLoader />}>
            <Routes>
              <Route path="/" element={<Dashboard />} />
              <Route path="/graph" element={<Graph />} />
              <Route path="/findings" element={<Findings />} />
              <Route path="/traffic" element={<Traffic />} />
              <Route path="/assets" element={<Assets />} />
            </Routes>
          </Suspense>
        </ErrorBoundary>
      </main>
      <ShortcutsHelp open={showHelp} onClose={() => setShowHelp(false)} />
    </div>
  )
}
