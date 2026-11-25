import { copyFileSync, mkdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// GitHub Pages has no SPA rewrites, so every app route gets its own copy of
// index.html, plus 404.html for anything else.
function staticRoutes(): Plugin {
  return {
    name: 'demo-static-routes',
    apply: 'build',
    writeBundle({ dir }) {
      if (!dir) return
      const app = readFileSync(new URL('./src/App.tsx', import.meta.url), 'utf8')
      const routes = [...app.matchAll(/path="\/([\w-]+)"/g)].map(m => m[1])
      const index = join(dir, 'index.html')
      copyFileSync(index, join(dir, '404.html'))
      for (const route of routes) {
        mkdirSync(join(dir, route), { recursive: true })
        copyFileSync(index, join(dir, route, 'index.html'))
      }
    },
  }
}

export default defineConfig(({ mode }) => {
  const demo = mode === 'demo'
  return {
    base: demo ? '/demo/' : '/',
    plugins: [react(), tailwindcss(), demo && staticRoutes()],
    server: demo ? {} : {
      proxy: {
        '/api': 'http://localhost:9400',
      },
    },
    build: {
      outDir: demo ? 'dist-demo' : 'dist',
    },
  }
})
