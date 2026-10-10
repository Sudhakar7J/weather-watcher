import path from 'node:path'
import { fileURLToPath } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

const rootDir = path.dirname(fileURLToPath(import.meta.url))

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(rootDir, './src'),
    },
  },
  server: {
    port: 5173,
    open: true,
  },
  build: {
    outDir: 'dist',
    // Ship source maps so a production stack trace is actually debuggable.
    sourcemap: true,
    // Fail the build if a chunk grows unexpectedly rather than silently
    // shipping a much larger bundle than last time.
    chunkSizeWarningLimit: 700,
  },
})
