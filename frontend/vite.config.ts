import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { chunkSizeWarningLimit: 2000 }, // assets load locally from the app bundle
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
})
