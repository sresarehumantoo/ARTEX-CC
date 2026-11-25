import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e/demo',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:4173',
    headless: true,
  },
  webServer: {
    command: 'npm run build:demo && npm run preview:demo -- --port 4173 --strictPort',
    port: 4173,
    reuseExistingServer: true,
    timeout: 180_000,
  },
})
