import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import { createAppRouter } from './router'
import { setTokenProvider, setUnauthorizedHandler } from './api/http'
import { useAuthStore } from './stores/auth'
import { useToastStore } from './stores/toast'
import './assets/main.css'

const app = createApp(App)
const pinia = createPinia()
const router = createAppRouter()

app.use(pinia)
app.use(router)

// Wire the HTTP client to the session once, in one place.
const auth = useAuthStore()
const toast = useToastStore()
setTokenProvider(() => auth.token)
setUnauthorizedHandler(() => {
  if (!auth.token) return
  auth.logout()
  toast.info('Your session has expired. Please sign in again.')
  router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
})

app.mount('#app')
