import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// During development run the Go server (go run ./cmd/auswertung -no-browser)
// and `npm run dev`; API requests are proxied to it.
export default defineConfig({
  plugins: [svelte()],
  server: {
    proxy: { '/api': 'http://127.0.0.1:8080' },
  },
  build: {
    emptyOutDir: true,
  },
})
