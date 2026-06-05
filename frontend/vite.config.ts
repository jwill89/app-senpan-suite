import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    // In dev, proxy API + WebSocket to the Go backend on :8080 so the
    // frontend behaves exactly as it does behind Apache in production
    // (Apache serves the SPA, proxies /api to Go). Start the backend with:
    //   cd src; go run . -addr :8080 -db ../data/bingo.sqlite
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    // Emits index.html + hashed assets/. Cutover = copy dist/* into the
    // Apache web root that currently holds index.html + assets/.
    outDir: 'dist',
    emptyOutDir: true,
  },
})
