import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import './styles/base.css'
import { useTheme } from './utils/theme'

const { initTheme } = useTheme()
initTheme()

const app = createApp(App)
app.use(router)
app.mount('#app')
