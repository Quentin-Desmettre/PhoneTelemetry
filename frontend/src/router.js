import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from './stores/auth'

import Setup from './views/Setup.vue'
import Login from './views/Login.vue'
import Dashboard from './views/Dashboard.vue'
import Settings from './views/Settings.vue'

const routes = [
  { path: '/setup', name: 'setup', component: Setup },
  { path: '/login', name: 'login', component: Login },
  { path: '/', name: 'dashboard', component: Dashboard, meta: { requiresAuth: true } },
  { path: '/settings', name: 'settings', component: Settings, meta: { requiresAuth: true } },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Central access control: enforce first-run setup, then authentication.
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (auth.setupComplete === null) {
    try {
      await auth.checkStatus()
    } catch {
      // Backend unreachable — let the target view surface the error.
    }
  }

  // Before any admin exists, the only reachable page is the setup panel.
  if (auth.setupComplete === false) {
    return to.name === 'setup' ? true : { name: 'setup' }
  }

  // Once configured, the setup panel must never appear again.
  if (to.name === 'setup') {
    return { name: auth.authenticated ? 'dashboard' : 'login' }
  }
  if (to.meta.requiresAuth && !auth.authenticated) {
    return { name: 'login' }
  }
  if (to.name === 'login' && auth.authenticated) {
    return { name: 'dashboard' }
  }
  return true
})

export default router
