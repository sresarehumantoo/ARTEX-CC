import { test, expect } from '@playwright/test'

test.describe('smoke tests', () => {
  test('dashboard loads', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveTitle(/ARTEX/)
    await expect(page.locator('h1')).toContainText('Dashboard')
  })

  test('sidebar navigation works', async ({ page }) => {
    await page.goto('/')
    const sidebar = page.locator('aside')
    await expect(sidebar).toBeVisible()
  })

  test('findings page loads', async ({ page }) => {
    await page.goto('/findings')
    await expect(page.locator('aside')).toBeVisible()
  })

  test('graph page loads', async ({ page }) => {
    await page.goto('/graph')
    await expect(page.locator('aside')).toBeVisible()
  })

  test('404 page shows for unknown routes', async ({ page }) => {
    await page.goto('/nonexistent')
    await expect(page.locator('text=404')).toBeVisible()
  })

  test('command palette opens with Ctrl+K', async ({ page }) => {
    await page.goto('/')
    await expect(page.locator('aside')).toBeVisible()
    await page.keyboard.press('Control+k')
    await expect(page.locator('[role="dialog"]')).toBeVisible({ timeout: 5_000 })
  })

  test('dark mode toggle works', async ({ page }) => {
    await page.goto('/')
    const html = page.locator('html')
    const initialDark = await html.evaluate(el => el.classList.contains('dark'))

    const themeButton = page.locator('button[aria-label*="Switch to"]')
    await themeButton.click()

    const afterDark = await html.evaluate(el => el.classList.contains('dark'))
    expect(afterDark).not.toBe(initialDark)
  })
})
