import { test, expect, type Page } from '@playwright/test'

const pages: [string, string][] = [
  ['', 'Dashboard'],
  ['chat', 'Recon acme.example'],
  ['graph', 'Graph'],
  ['findings', 'concatenated into a PostgreSQL query'],
  ['traffic', 'api.acme.example'],
  ['activity', 'Reported critical'],
  ['assets', 'portal.acme.example'],
  ['coverage', 'Coverage'],
  ['credentials', 'svc-backup@acme.example'],
  ['report', 'Weak password on svc-backup service account'],
  ['tasks', 'Obtain domain admin'],
  ['commands', 'subfinder'],
  ['usage', 'API authorization testing'],
  ['workspace', 'scope.txt'],
  ['agents', 'Pentest Lead'],
  ['mcp-servers', 'artex'],
  ['tools', 'insert_assets'],
  ['intercept', 'Require approval for shell commands'],
  ['logs', 'tool called'],
  ['search?q=acme', 'portal.acme.example'],
  ['settings', 'Settings'],
]

function watch(page: Page) {
  const offsite: string[] = []
  const errors: string[] = []
  page.on('request', req => {
    const url = new URL(req.url())
    if (url.origin !== 'http://localhost:4173' || !url.pathname.startsWith('/demo/')) offsite.push(req.url())
  })
  page.on('pageerror', err => errors.push(err.message))
  page.on('console', msg => {
    if (msg.type() === 'error') errors.push(msg.text())
  })
  return { offsite, errors }
}

for (const [path, text] of pages) {
  test(`/demo/${path} renders from fixtures`, async ({ page }) => {
    const { offsite, errors } = watch(page)
    await page.goto(`/demo/${path}`)
    await expect(page.getByText('Live demo: a fictional, seeded engagement.')).toBeVisible()
    await expect(page.getByText(text).first()).toBeVisible()
    await expect(page.getByText(/Failed to load|Something went wrong/)).toHaveCount(0)
    await page.waitForTimeout(300)
    expect(offsite).toEqual([])
    expect(errors).toEqual([])
  })
}

test('sidebar navigation stays inside /demo', async ({ page }) => {
  const { offsite } = watch(page)
  await page.goto('/demo/')
  await page.locator('aside nav').getByText('Findings', { exact: true }).click()
  await expect(page).toHaveURL(/\/demo\/findings$/)
  await page.getByText('sqli').first().click()
  await expect(page.getByText('proof').first()).toBeVisible()
  await page.keyboard.press('Escape')
  await page.locator('aside nav').getByText('Traffic', { exact: true }).click()
  await page.getByText('/api/v2/search').first().click()
  await expect(page.getByText('pq: syntax error').first()).toBeVisible()
  expect(offsite).toEqual([])
})

test('chat replays a captured conversation', async ({ page }) => {
  await page.goto('/demo/chat')
  await page.getByText('Recon acme.example').click()
  await expect(page.getByText('Recon summary')).toBeVisible()
})

test('mutations report read-only', async ({ page }) => {
  await page.goto('/demo/tasks')
  await expect(page.getByText('Live demo: a fictional, seeded engagement.')).toBeVisible()
  const res = await page.evaluate(async () => {
    const r = await fetch('/api/hints', { method: 'POST', body: '{}' })
    return { status: r.status, body: await r.json() }
  })
  expect(res).toEqual({ status: 403, body: { error: 'Demo is read-only' } })
})
