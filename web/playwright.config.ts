import { defineConfig } from '@playwright/test'

// E2E_BASE_URL targets a running server (see scripts/web-e2e/run.sh) instead of the Vite dev server.
const baseURL = process.env.E2E_BASE_URL

export default defineConfig({
  testDir: './e2e',
  testIgnore: 'demo/**',
  timeout: 30_000,
  retries: 0,
  globalSetup: baseURL ? './e2e/login.setup.ts' : undefined,
  use: {
    baseURL: baseURL ?? 'http://localhost:5173',
    headless: true,
    storageState: baseURL ? 'playwright/.auth/state.json' : undefined,
  },
  webServer: baseURL
    ? undefined
    : {
        command: 'npm run dev',
        port: 5173,
        reuseExistingServer: true,
      },
})
