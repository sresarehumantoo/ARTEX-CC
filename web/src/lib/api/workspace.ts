export interface WorkspaceEntry {
  name: string
  type: 'file' | 'dir'
  size: number
  modified: string
}

const BASE = '/api/workspace'

async function check(res: Response) {
  if (res.ok) return res
  let message = `${res.status} ${res.statusText}`
  try {
    const body = await res.json()
    if (body?.error) message = body.error
  } catch {
    // keep the status text
  }
  throw new Error(message)
}

const q = (path: string) => `?path=${encodeURIComponent(path)}`

export function joinPath(dir: string, name: string) {
  return dir ? `${dir}/${name}` : name
}

export function fileUrl(path: string, download = false) {
  return `${BASE}/file${q(path)}${download ? '&download=1' : ''}`
}

export async function listWorkspace(path: string) {
  const res = await check(await fetch(`${BASE}${q(path)}`))
  return ((await res.json()) as WorkspaceEntry[] | null) ?? []
}

export async function readWorkspaceFile(path: string) {
  const res = await check(await fetch(fileUrl(path)))
  return res.text()
}

export async function writeWorkspaceFile(path: string, content: BodyInit) {
  await check(await fetch(`${BASE}/file${q(path)}`, { method: 'PUT', body: content }))
}

export async function deleteWorkspacePath(path: string) {
  await check(await fetch(`${BASE}/file${q(path)}`, { method: 'DELETE' }))
}

export async function makeWorkspaceDir(path: string) {
  await check(await fetch(`${BASE}/mkdir${q(path)}`, { method: 'POST' }))
}
