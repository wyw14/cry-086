import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'CRANE_')
  const backend = env.CRANE_API_TARGET || 'http://localhost:8080'
  return {
    appType: 'spa',
    plugins: [vue()],
    build: {
      sourcemap: false,
      rollupOptions: {
        output: { manualChunks: { element: ['element-plus'], state: ['pinia', 'vue-router'] } }
      }
    },
    server: {
      port: 5173,
      strictPort: true,
      proxy: {
        '/api': { target: backend, changeOrigin: false },
        '/healthz': { target: backend, changeOrigin: false }
      }
    },
    test: { environment: 'jsdom', globals: true }
  }
})
