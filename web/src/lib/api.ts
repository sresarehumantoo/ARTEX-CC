import { AUTH_REQUIRED_EVENT } from './auth'
import type { EngagementData, GraphData, Finding, FindingDetail, Asset, TrafficPage, TrafficDetail, Activity, CoverageStats, CoverageGaps, Credential, Constraint } from './types'

export type { Asset } from './types'

const BASE = '/api'

function notifyUnauthorized(res: Response) {
  if (res.status === 401) window.dispatchEvent(new Event(AUTH_REQUIRED_EVENT))
}

async function fetchJSON<T>(path: string, params?: Record<string, string>): Promise<T> {
  const url = new URL(path, window.location.origin)
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v) url.searchParams.set(k, v)
    }
  }
  const res = await fetch(url.toString())
  notifyUnauthorized(res)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export function getEngagement() {
  return fetchJSON<EngagementData>(`${BASE}/engagement`)
}

export function getGraph() {
  return fetchJSON<GraphData>(`${BASE}/graph`)
}

export function getFindings() {
  return fetchJSON<Finding[]>(`${BASE}/findings`)
}

export function getFindingDetail(id: number) {
  return fetchJSON<FindingDetail>(`${BASE}/findings/${id}`)
}

export function getAssets(params?: { type?: string; dsl?: string; limit?: string; offset?: string }) {
  return fetchJSON<{ assets: Asset[]; total: number }>(`${BASE}/assets`, params)
}

export interface AssetListResponse {
  assets: Asset[]
  total: number
}

export async function fetchAssets(params: {
  type?: string
  dsl?: string
  limit?: number
  offset?: number
}): Promise<AssetListResponse> {
  const sp = new URLSearchParams()
  if (params.type) sp.set('type', params.type)
  if (params.dsl) sp.set('dsl', params.dsl)
  sp.set('limit', String(params.limit ?? 50))
  sp.set('offset', String(params.offset ?? 0))

  const res = await fetch(`${BASE}/assets?${sp}`)
  notifyUnauthorized(res)
  if (!res.ok) throw new Error(`GET /api/assets: ${res.status}`)
  return res.json()
}

export async function fetchAsset(id: number): Promise<Asset> {
  const res = await fetch(`${BASE}/assets/${id}`)
  notifyUnauthorized(res)
  if (!res.ok) throw new Error(`GET /api/assets/${id}: ${res.status}`)
  return res.json()
}

export function getAssetFindings(assetId: number) {
  return fetchJSON<Finding[]>(`${BASE}/assets/${assetId}/findings`)
}

export function getTraffic(params?: Record<string, string>) {
  return fetchJSON<TrafficPage>(`${BASE}/traffic`, params)
}

export function getTrafficDetail(id: string) {
  return fetchJSON<TrafficDetail>(`${BASE}/traffic/${id}`)
}

export function getActivity(params?: { limit?: string; kind?: string; node_id?: string }) {
  return fetchJSON<Activity[]>(`${BASE}/activity`, params)
}

export function getCoverage() {
  return fetchJSON<CoverageStats>(`${BASE}/coverage`)
}

export function getCoverageGaps() {
  return fetchJSON<CoverageGaps>(`${BASE}/coverage/gaps`)
}

export function getCredentials() {
  return fetchJSON<Credential[]>(`${BASE}/credentials`)
}

export function getConstraints() {
  return fetchJSON<Constraint[]>(`${BASE}/constraints`)
}
