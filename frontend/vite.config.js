import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// In development the Vite server proxies /api to the Go backend, so the
// browser only ever talks to one origin and no CORS setup is needed.
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: process.env.VITE_PROXY_TARGET || 'http://localhost:8080', changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    include: ['tests/**/*.test.js'],
    setupFiles: ['tests/setup.js'],
  },
})
