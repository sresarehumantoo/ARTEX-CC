import { request, type FullConfig } from '@playwright/test'

export default async function globalSetup(config: FullConfig) {
  const { baseURL, storageState } = config.projects[0].use
  const password = process.env.E2E_PASSWORD
  if (!password) throw new Error('E2E_PASSWORD is required with E2E_BASE_URL')

  const ctx = await request.newContext({ baseURL })
  const status = await (await ctx.get('/api/auth/status')).json()
  if (status.auth_required) {
    const endpoint = status.initialized ? '/api/auth/login' : '/api/auth/init'
    const data = status.initialized
      ? { password }
      : { password, setup_token: process.env.E2E_SETUP_TOKEN ?? '' }
    const res = await ctx.post(endpoint, { data })
    if (!res.ok()) throw new Error(`${endpoint}: ${res.status()} ${await res.text()}`)
  }
  await ctx.storageState({ path: storageState as string })
  await ctx.dispose()
}
