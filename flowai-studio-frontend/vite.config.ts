import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backendTarget = process.env.FLOWAI_BACKEND_TARGET === 'go'
  ? 'http://127.0.0.1:3001'
  : 'http://127.0.0.1:3000'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': '/src',
      '@components': '/src/components',
      '@pages': '/src/pages',
      '@store': '/src/store',
      '@hooks': '/src/hooks',
      '@utils': '/src/utils',
      '@types': '/src/types',
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: backendTarget,
        changeOrigin: true,
      },
    },
  },
})
