import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// proxyTarget builds the dev-proxy config for the Go API, logging upstream
// failures explicitly. Without an error handler, a backend that's down or
// resets the connection surfaces in the browser as an opaque 502 with nothing
// in the Vite terminal explaining why -- so connection refused (backend not
// running) is the first thing to check, and now it's printed.
function proxyTarget() {
  return {
    target: 'http://localhost:8081',
    changeOrigin: true,
    configure: (proxy) => {
      proxy.on('error', (err, req) => {
        console.error(`[proxy] ${req.method} ${req.url} -> ${err.code || err.message} (is the Go API running on :8081?)`)
      })
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  base: process.env.GITHUB_PAGES ? '/ChatVault/' : '/',
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': proxyTarget(),
      '/auth': proxyTarget(),
    },
  },
})
