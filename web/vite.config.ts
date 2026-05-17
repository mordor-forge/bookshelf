import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

// outputs directly to internal/web/dist for the go:embed directive in
// internal/web/embed.go — no copy step.
export default defineConfig({
  plugins: [vue()],
  base: '/',
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir: path.resolve(__dirname, '../internal/web/dist'),
    emptyOutDir: true,
  },
  server: {
    port: 19321,
    proxy: {
      '/api': 'http://localhost:19320',
      '/books': 'http://localhost:19320',
      '/healthz': 'http://localhost:19320',
    },
  },
})
