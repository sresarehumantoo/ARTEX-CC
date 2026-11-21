import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Eye, EyeOff, Copy, Key, Search, X } from 'lucide-react'
import { getCredentials } from '../lib/api'
import { Spinner } from '../components/ui/spinner'

export default function Credentials() {
  const [filter, setFilter] = useState('')
  const [revealed, setRevealed] = useState<Set<number>>(() => new Set())

  const { data: credentials, isLoading, error } = useQuery({
    queryKey: ['credentials'],
    queryFn: getCredentials,
  })

  const filtered = useMemo(() => {
    if (!credentials) return []
    if (!filter) return credentials
    const q = filter.toLowerCase()
    return credentials.filter(
      (c) =>
        c.domain.toLowerCase().includes(q) ||
        c.username.toLowerCase().includes(q),
    )
  }, [credentials, filter])

  function toggleReveal(id: number) {
    setRevealed((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  function copyPassword(password: string) {
    navigator.clipboard.writeText(password)
  }

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 md:px-4 py-2 md:py-3">
        <div className="flex items-center gap-2">
          <Key className="h-5 w-5 text-slate-500 dark:text-slate-400" />
          <h1 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Credentials</h1>
        </div>
      </div>

      <div className="border-b border-slate-200 dark:border-slate-800 px-3 md:px-4 py-2">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter by domain or username"
            className="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-md py-2 pl-9 pr-9 text-sm text-slate-800 dark:text-slate-200 placeholder:text-slate-400 dark:placeholder:text-slate-600 focus:outline-none focus:ring-1 focus:ring-slate-500 focus:border-slate-500"
          />
          {filter && (
            <button
              onClick={() => setFilter('')}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 hover:text-slate-700 dark:hover:text-slate-300"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      <div className="flex-1 overflow-auto">
        {error ? (
          <div className="flex items-center justify-center h-48 text-sm text-rose-400">
            Failed to load credentials: {(error as Error).message}
          </div>
        ) : isLoading ? (
          <div className="flex items-center justify-center h-48">
            <Spinner className="h-8 w-8" />
          </div>
        ) : filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-48 gap-2 text-slate-500">
            <Key className="h-10 w-10 text-slate-400 dark:text-slate-600" />
            <p className="text-sm">
              {credentials && credentials.length > 0
                ? 'No credentials match your filter'
                : 'No credentials captured yet'}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left min-w-[600px]">
              <thead>
                <tr className="border-b border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50">
                  <th className="px-3 py-2 text-xs font-medium text-slate-500 uppercase tracking-wider">Domain</th>
                  <th className="px-3 py-2 text-xs font-medium text-slate-500 uppercase tracking-wider">Username</th>
                  <th className="px-3 py-2 text-xs font-medium text-slate-500 uppercase tracking-wider">Password</th>
                  <th className="px-3 py-2 text-xs font-medium text-slate-500 uppercase tracking-wider">Source</th>
                  <th className="px-3 py-2 text-xs font-medium text-slate-500 uppercase tracking-wider">Note</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((cred) => (
                  <tr
                    key={cred.id}
                    className="border-b border-slate-200 dark:border-slate-800 hover:bg-slate-100/50 dark:hover:bg-slate-800/50 transition-colors"
                  >
                    <td className="px-3 py-2 text-slate-800 dark:text-slate-200 text-sm">{cred.domain}</td>
                    <td className="px-3 py-2 text-slate-800 dark:text-slate-200 text-sm font-mono">{cred.username}</td>
                    <td className="px-3 py-2">
                      <div className="flex items-center gap-1.5">
                        <span className="text-sm text-slate-800 dark:text-slate-200 font-mono">
                          {revealed.has(cred.id) ? (cred.password ?? '') : '••••••••'}
                        </span>
                        <button
                          onClick={() => toggleReveal(cred.id)}
                          className="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors"
                          title={revealed.has(cred.id) ? 'Hide password' : 'Show password'}
                        >
                          {revealed.has(cred.id) ? (
                            <EyeOff className="w-3.5 h-3.5" />
                          ) : (
                            <Eye className="w-3.5 h-3.5" />
                          )}
                        </button>
                        {cred.password && (
                          <button
                            onClick={() => copyPassword(cred.password!)}
                            className="p-1 rounded hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors"
                            title="Copy password"
                          >
                            <Copy className="w-3.5 h-3.5" />
                          </button>
                        )}
                      </div>
                    </td>
                    <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm">{cred.source ?? ''}</td>
                    <td className="px-3 py-2 text-slate-500 dark:text-slate-400 text-sm">{cred.note ?? ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
