import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
  },
  server: {

    proxy: {
      '/status': 'http://127.0.0.1:8080',
      '/files': 'http://127.0.0.1:8080',
      '/history': 'http://127.0.0.1:8080',
      '/commit': 'http://127.0.0.1:8080',
      '/remote': 'http://127.0.0.1:8080',
      '/push': 'http://127.0.0.1:8080',
      '/rollback': 'http://127.0.0.1:8080',
      '/gc': 'http://127.0.0.1:8080',
      '/setup': 'http://127.0.0.1:8080',
      '/admin': 'http://127.0.0.1:8080',
      '/languages': 'http://127.0.0.1:8080',
      '/me': 'http://127.0.0.1:8080',
    },
  },
})
