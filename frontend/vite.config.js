import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// In dev, proxy API calls to the Go backend on :8080. In production the Go
// binary serves the built assets and the API from the same origin.
export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
