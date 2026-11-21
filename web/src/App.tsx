import { lazy, Suspense } from 'react'
import { Routes, Route } from 'react-router-dom'
import { Sidebar } from './components/Sidebar'
import { Dashboard } from './pages/Dashboard'

const Graph = lazy(() => import('./pages/Graph'))
const Findings = lazy(() => import('./pages/Findings'))
const Traffic = lazy(() => import('./pages/Traffic'))
const Assets = lazy(() => import('./pages/Assets'))

function PageLoader() {
  return (
    <div className="flex items-center justify-center h-full text-slate-400">
      Loading...
    </div>
  )
}

export default function App() {
  return (
    <div className="flex h-screen bg-slate-900">
      <Sidebar />
      <main className="flex-1 overflow-y-auto">
        <Suspense fallback={<PageLoader />}>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/graph" element={<Graph />} />
            <Route path="/findings" element={<Findings />} />
            <Route path="/traffic" element={<Traffic />} />
            <Route path="/assets" element={<Assets />} />
          </Routes>
        </Suspense>
      </main>
    </div>
  )
}
