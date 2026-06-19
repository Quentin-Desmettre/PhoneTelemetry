import { defineStore } from 'pinia'
import { api } from '../api/client'

// Tracks whether first-run setup is done and whether we hold a valid session.
export const useAuthStore = defineStore('auth', {
  state: () => ({
    setupComplete: null, // null = unknown (not yet fetched)
    authenticated: false,
  }),
  actions: {
    async checkStatus() {
      const s = await api.status()
      this.setupComplete = s.setup_complete
      this.authenticated = s.authenticated
    },
    async setup(username, password) {
      await api.setup(username, password)
      this.setupComplete = true
      this.authenticated = true
    },
    async login(username, password) {
      await api.login(username, password)
      this.authenticated = true
    },
    async logout() {
      await api.logout()
      this.authenticated = false
    },
  },
})
