import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    host: 'localhost',
    origin: 'http://localhost:5173',
    cors: true,
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
