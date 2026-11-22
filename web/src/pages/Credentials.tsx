import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Eye, EyeOff, Copy, Key, Search, Download } from 'lucide-react'
import { getCredentials } from '../lib/api'
import type { Credential } from '../lib/types'
import { downloadFile } from '../lib/export'
import { cn } from '../lib/utils'
import { CredentialsSkeleton } from '../components/skeletons/CredentialsSkeleton'
import { useToast } from '../hooks/useToast'

function exportCredentialsCSV(creds: Credential[]) {
  const headers = ['Domain', 'Username', 'Source', 'Note']
  const rows = creds.map(c => [c.domain, c.username, c.source ?? '', c.note ?? ''])
  const csv = [headers, ...rows].map(r => r.map(v => `"${v.replace(/"/g, '""')}"`).join(',')).join('\n')
  downloadFile(csv, `credentials-${new Date().toISOString().slice(0, 10)}.csv`, 'text/csv')
}

export default function Credentials() {
  const { toast } = useToast()
  const [filter, setFilter] = useState('')
  const [domainFilter, setDomainFilter] = useState('')
  const [revealed, setRevealed] = useState<Set<number>>(() => new Set())

  const { data: credentials, isLoading, error, refetch } = useQuery({
    queryKey: ['credentials'],
    queryFn: getCredentials,
  })

  const domains = useMemo(() => {
    if (!credentials) return []
    return [...new Set(credentials.map(c => c.domain))].sort()
  }, [credentials])

  const filtered = useMemo(() => {
    if (!credentials) return []
    let list = credentials
    if (domainFilter) list = list.filter(c => c.domain === domainFilter)
    if (filter) {
      const q = filter.toLowerCase()
      list = list.filter(
        (c) =>
          c.domain.toLowerCase().includes(q) ||
          c.username.toLowerCase().includes(q) ||
          (c.source ?? '').toLowerCase().includes(q) ||
          (c.note ?? '').toLowerCase().includes(q),
      )
    }
    return list
  }, [credentials, domainFilter, filter])

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
    toast('Password copied', { variant: 'success' })
  }

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 px-3 md:px-4 py-2 md:py-3">
        <div className="flex items-center gap-2">
          <Key className="h-5 w-5 text-slate-500 dark:text-slate-400" />
          <h1 className="text-lg font-semibold text-slate-900 dark:text-slate-100">Credentials</h1>
          {credentials && (
            <span className="rounded-full bg-slate-200 dark:bg-slate-700 px-2 py-0.5 text-xs font-medium text-slate-600 dark:text-slate-300">
              {credentials.length}
            </span>
          )}
          <div className="ml-auto">
            <button
              onClick={() => {
                if (filtered.length > 0) {
                  exportCredentialsCSV(filtered)
                  toast('Credentials exported (passwords excluded)', { variant: 'success' })
                }
              }}
              disabled={filtered.length === 0}
              className="inline-flex items-center gap-1.5 rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 px-2.5 py-1.5 text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              title="Export credentials as CSV (passwords excluded)"
            >
              <Download className="h-3.5 w-3.5" />
              Export
            </button>
          </div>
        </div>
      </div>

      <div className="flex flex-col sm:flex-row sm:items-center gap-2 px-3 md:px-4 py-2 border-b border-slate-200 dark:border-slate-800">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-slate-400" />
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Search credentials..."
            className="w-full bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-700 rounded-md py-2 pl-9 pr-3 text-sm text-slate-800 dark:text-slate-200 placeholder:text-slate-400 dark:placeholder:text-slate-600 focus:outline-none focus:ring-1 focus:ring-slate-500"
          />
        </div>
        {domains.length > 1 && (
          <select
            value={domainFilter}
            onChange={(e) => setDomainFilter(e.target.value)}
            className="rounded-md border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 py-2 text-sm text-slate-700 dark:text-slate-300"
          >
            <option value="">All domains</option>
            {domains.map(d => <option key={d} value={d}>{d}</option>)}
          </select>
        )}
      </div>

      {domains.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 px-3 md:px-4 py-1.5 border-b border-slate-200 dark:border-slate-800 text-xs">
          {domains.map(d => {
            const count = credentials!.filter(c => c.domain === d).length
            return (
              <button
                key={d}
                type="button"
                onClick={() => setDomainFilter(domainFilter === d ? '' : d)}
                className={cn(
                  'rounded-full px-2 py-0.5 font-medium transition-colors',
                  domainFilter === d
                    ? 'bg-emerald-500/15 text-emerald-500'
                    : 'bg-slate-500/10 text-slate-500 hover:bg-slate-500/20',
                )}
              >
                {d} {count}
              </button>
            )
          })}
        </div>
      )}

      {(filter || domainFilter) && credentials && (
        <div className="text-xs text-slate-400 dark:text-slate-500 px-3 md:px-4 py-1">
          Showing {filtered.length} of {credentials.length}
        </div>
      )}

      <div className="flex-1 overflow-auto">
        {error ? (
          <div className="flex items-center justify-center h-48">
            <div className="rounded-lg border border-rose-200 dark:border-rose-800/50 bg-rose-50 dark:bg-rose-900/20 p-4">
              <p className="text-sm text-rose-600 dark:text-rose-400">Failed to load credentials: {(error as Error).message}</p>
              <button
                type="button"
                onClick={() => refetch()}
                className="mt-2 text-xs font-medium text-rose-500 hover:text-rose-700 dark:hover:text-rose-300"
              >
                Try again
              </button>
            </div>
          </div>
        ) : isLoading ? (
          <CredentialsSkeleton />
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
