import { fileURLToPath } from 'node:url'
import { defineConfig, configDefaults } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import vuetify from 'vite-plugin-vuetify'

export default defineConfig({
  plugins: [vue(), vuetify({ autoImport: true })],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      // Stub out @vue/devtools-api so that @vue/devtools-kit (its dependency) is
      // never loaded. @vue/devtools-kit calls localStorage at module-init time,
      // before jsdom has been set up, causing "localStorage.getItem is not a function".
      '@vue/devtools-api': fileURLToPath(
        new URL('./src/test/__stubs__/devtools-api.ts', import.meta.url)
      ),
    },
  },
  test: {
    environment: 'jsdom',
    environmentOptions: {
      jsdom: { url: 'http://localhost/' },
    },
    exclude: [...configDefaults.exclude, 'e2e/**'],
    root: fileURLToPath(new URL('./', import.meta.url)),
    server: {
      deps: {
        // Inline packages so Vite processes them and resolve.alias is applied.
        // Without inlining, pinia is loaded natively by Node.js and the alias
        // for @vue/devtools-api is not respected.
        inline: ['vuetify', 'pinia', '@vue/devtools-api'],
      },
    },
    setupFiles: ['./src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,vue}'],
      exclude: [
        'src/main.ts',
        'src/plugins/**',
        'src/test/**',
        'src/**/__tests__/**',
        'src/types/**',
        'src/env.d.ts',
      ],
      reporter: ['text', 'lcov'],
      thresholds: {
        lines: 70,
        functions: 70,
        branches: 60,
        statements: 70,
      },
    },
  },
})
