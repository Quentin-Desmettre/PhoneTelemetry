import { defineStore } from 'pinia'
import { api } from '../api/client'

export const useMetricsStore = defineStore('metrics', {
  state: () => ({
    current: null,
    history: null,
    range: '24h',
  }),
  actions: {
    async fetchCurrent() {
      this.current = await api.current()
    },
    async fetchHistory() {
      this.history = await api.history(this.range)
    },
    async setRange(range) {
      this.range = range
      await this.fetchHistory()
    },
  },
})
