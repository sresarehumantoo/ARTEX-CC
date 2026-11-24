// Usage: node capture.cjs <screenshot-dir> <video-dir>
// Resolve playwright from web/node_modules (see run.sh).
const { chromium } = require('playwright')

const [shotDir, videoDir] = process.argv.slice(2)
const BASE = 'http://127.0.0.1:19400'
const PASSWORD = 'artex-demo-pass'

async function login(browser) {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, colorScheme: 'dark' })
  const page = await ctx.newPage()
  await page.goto(BASE + '/login')
  await page.fill('input[type=password]', PASSWORD)
  await page.keyboard.press('Enter')
  await page.waitForURL((url) => !url.pathname.startsWith('/login'))
  const state = await ctx.storageState()
  await ctx.close()
  return state
}

async function screenshots(browser, state) {
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1.5, colorScheme: 'dark' })
  const page = await ctx.newPage()
  // Never wait for networkidle: the SSE stream keeps the network busy.
  const settle = (ms) => page.waitForTimeout(ms)

  await page.goto(BASE + '/login')
  await settle(800)
  await page.screenshot({ path: `${shotDir}/login.png` })
  await ctx.addCookies(state.cookies)

  for (const name of ['dashboard', 'graph', 'findings', 'coverage', 'assets', 'report']) {
    await page.goto(BASE + (name === 'dashboard' ? '/' : '/' + name))
    await settle(2000)
    await page.screenshot({ path: `${shotDir}/${name}.png` })
  }

  await page.goto(BASE + '/traffic')
  await settle(1500)
  await page.getByText('/api/v2/search').first().click()
  await settle(1000)
  await page.screenshot({ path: `${shotDir}/traffic.png` })

  await page.goto(BASE + '/findings')
  await settle(1500)
  await page.getByText('sqli').first().click()
  await settle(1200)
  await page.screenshot({ path: `${shotDir}/finding-detail.png` })
  await ctx.close()
}

async function walkthrough(browser, state) {
  const viewport = { width: 1280, height: 800 }
  const ctx = await browser.newContext({ viewport, colorScheme: 'dark', storageState: state, recordVideo: { dir: videoDir, size: viewport } })
  const page = await ctx.newPage()
  // Navigate through the sidebar, not page.goto: a full reload drops the SSE
  // connection and the recording catches the reconnect banner and skeletons.
  const nav = async (label, ms) => {
    await page.locator('aside nav').getByText(label, { exact: true }).click()
    await page.waitForTimeout(ms)
  }

  await page.goto(BASE + '/')
  await page.waitForTimeout(3500)
  await nav('Graph', 3500)
  await nav('Findings', 1500)
  await page.getByText('sqli').first().click()
  await page.waitForTimeout(2800)
  await page.keyboard.press('Escape')
  await page.waitForTimeout(400)
  await nav('Traffic', 1300)
  await page.getByText('/api/v2/search').first().click()
  await page.waitForTimeout(2800)
  await nav('Coverage', 2800)
  await nav('Report', 1500)
  await page.mouse.move(800, 500)
  await page.mouse.wheel(0, 600)
  await page.waitForTimeout(1800)
  await ctx.close()
}

;(async () => {
  const browser = await chromium.launch()
  const state = await login(browser)
  await screenshots(browser, state)
  await walkthrough(browser, state)
  await browser.close()
})().catch((err) => {
  console.error(err)
  process.exit(1)
})
