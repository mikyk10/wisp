import '@fontsource/poppins/400.css'
import '@fontsource/poppins/700.css'
import './assets/main.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import vuetify from './plugins/vuetify'

const app = createApp(App)

app.config.errorHandler = (err, _vm, info) => {
  console.error('[Vue errorHandler]', err, info)
}

app.use(createPinia())
app.use(vuetify)

app.mount('#app')
