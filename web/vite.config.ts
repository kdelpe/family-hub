/// <reference types="vitest/config" />
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react()],
  // The Go server serves the built app and the API from one origin; in dev, Vite stands in front of it.
  server: { proxy: { '/api': 'http://localhost:8080' } },
  build: { outDir: '../internal/webui/dist', emptyOutDir: true },
  test: { environment: 'jsdom', setupFiles: ['./src/test/setup.ts'] },
})
