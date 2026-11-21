import { useQuery } from '@tanstack/react-query'

export interface SearchResult {
  type: 'finding' | 'asset' | 'node'
  id: number | string
  title: string
  subtitle: string
  url: string
}

interface SearchResponse {
  results: SearchResult[]
}

export function useSearch(query: string) {
  return useQuery({
    queryKey: ['search', query],
    queryFn: async (): Promise<SearchResponse> => {
      if (!query || query.length < 2) return { results: [] }
      const res = await fetch(`/api/search?q=${encodeURIComponent(query)}`)
      if (!res.ok) throw new Error('Search failed')
      return res.json() as Promise<SearchResponse>
    },
    enabled: query.length >= 2,
    staleTime: 30_000,
  })
}
