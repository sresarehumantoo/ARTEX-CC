export interface AuthStatus {
  initialized: boolean
  auth_required: boolean
}

export const AUTH_REQUIRED_EVENT = 'auth-required'

async function post(path: string, body: unknown, fallback: string): Promise<void> {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (res.ok) return
  const data = await res.json().catch(() => null)
  throw new Error(data?.error ?? fallback)
}

export async function checkAuthStatus(): Promise<AuthStatus> {
  const res = await fetch('/api/auth/status')
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  return res.json()
}

export function login(password: string): Promise<void> {
  return post('/api/auth/login', { password }, 'Login failed')
}

export function initAuth(password: string, setupToken: string): Promise<void> {
  return post('/api/auth/init', { password, setup_token: setupToken }, 'Setup failed')
}

export function changePassword(current: string, newPassword: string): Promise<void> {
  return post(
    '/api/auth/change-password',
    { current_password: current, new_password: newPassword },
    'Password change failed',
  )
}

export async function logout(): Promise<void> {
  await fetch('/api/auth/logout', { method: 'POST' }).catch(() => undefined)
  window.location.assign(import.meta.env.BASE_URL + 'login')
}
