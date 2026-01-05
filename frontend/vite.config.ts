import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src')
    }
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:3007',
        changeOrigin: true,
        ws: true
      }
    }
  },
  build: {
    outDir: '../backend/static',
    emptyOutDir: true
  }
})
