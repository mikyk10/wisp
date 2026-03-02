import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vuetify from 'vite-plugin-vuetify'

// https://vite.dev/config/
// vite-plugin-vue-devtools is loaded only in development:
// @vue/devtools-kit (a transitive dep) calls localStorage.getItem at module
// init time, crashing Node.js during `vite build` where localStorage does not
// exist. A dynamic import ensures the module is never evaluated in production.
export default defineConfig(async ({ mode }) => {
  const devPlugins = []
  if (mode === 'development') {
    const { default: vueDevTools } = await import('vite-plugin-vue-devtools')
    devPlugins.push(vueDevTools())
  }

  return {
    plugins: [
      vue(),
      ...devPlugins,
      vuetify({ autoImport: true }),
    ],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url))
      },
    },
    server: {
      watch: {
        // macOS → Docker volume mount does not propagate inotify events.
        // Polling ensures HMR works when running inside a container.
        usePolling: true,
      },
    },
    preview: {
      host: '0.0.0.0',
      allowedHosts: true,
    },
  }
})
