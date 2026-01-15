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
    port: Number(process.env.ACA_FRONTEND_PORT || '34001'),
    host: '0.0.0.0',
    allowedHosts: 'all',
    proxy: {
      '/api': {
        target: `http://${process.env.ACA_BACKEND_HOST || 'localhost'}:${process.env.ACA_BACKEND_PORT || '34007'}`,
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
