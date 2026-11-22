import { Link } from 'react-router-dom'
import { FileQuestion } from 'lucide-react'

export default function NotFound() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-4 p-6">
      <FileQuestion className="h-16 w-16 text-slate-300 dark:text-slate-700" />
      <div className="text-center">
        <h1 className="text-4xl font-bold text-foreground">404</h1>
        <p className="mt-2 text-sm text-muted-foreground">Page not found</p>
      </div>
      <Link
        to="/"
        className="mt-2 rounded-md bg-muted px-4 py-2 text-sm font-medium text-foreground hover:bg-slate-300 dark:hover:bg-slate-700 transition-colors"
      >
        Back to Dashboard
      </Link>
    </div>
  )
}
