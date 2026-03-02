import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import vueTsEslintConfig from '@vue/eslint-config-typescript'

export default [
  js.configs.recommended,
  ...pluginVue.configs['flat/recommended'],
  ...vueTsEslintConfig(),
  {
    rules: {
      // App.vue / single-word component names を許可
      'vue/multi-word-component-names': 'off',
      // any 禁止（段階的に error へ引き上げる）
      '@typescript-eslint/no-explicit-any': 'warn',
      // console.log は warn（console.error/warn は許可）
      'no-console': ['warn', { allow: ['warn', 'error'] }],
    },
  },
  {
    // テスト・設定ファイルはルールを緩和
    files: ['**/*.spec.ts', '**/*.config.ts', 'e2e/**', 'cypress/**'],
    rules: {
      'no-console': 'off',
      '@typescript-eslint/no-explicit-any': 'off',
    },
  },
  {
    ignores: ['dist/', 'node_modules/', 'coverage/'],
  },
]
