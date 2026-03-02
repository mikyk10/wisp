import { createVuetify } from 'vuetify'
import 'vuetify/styles'
import { aliases, mdi } from 'vuetify/iconsets/mdi'
import '@mdi/font/css/materialdesignicons.css'

export default createVuetify({
  theme: {
    defaultTheme: 'dark',
    themes: {
      dark: {
        dark: true,
        colors: {
          primary: '#00d2a8',
          'on-primary': '#0f1117',
          secondary: '#1a1d27',
          background: '#0f1117',
          surface: '#1a1d27',
          'surface-variant': '#252836',
          error: '#ff5370',
          info: '#82aaff',
          success: '#c3e88d',
          warning: '#ffcb6b',
        },
      },
    },
  },
  icons: {
    defaultSet: 'mdi',
    aliases,
    sets: {
      mdi,
    },
  },
})
