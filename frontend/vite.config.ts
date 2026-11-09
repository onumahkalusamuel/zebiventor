import path from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'

// In `wails dev`, paths Vite doesn't know must 404 so Wails hands them to
// the app's Go handler (product pictures at /img/...), not index.html.
const appRoutes: Plugin = {
  name: 'zebiventor-app-routes',
  configureServer(server) {
    server.middlewares.use((req, res, next) => {
      if (req.url?.startsWith('/img/')) {
        res.statusCode = 404
        res.end()
        return
      }
      next()
    })
  },
}

export default defineConfig({
  plugins: [appRoutes, react(), tailwindcss()],
  build: { chunkSizeWarningLimit: 2000 }, // assets load locally from the app bundle
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
})
