import { defineStore } from 'pinia'
import { api } from '../api/client'

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    pollIntervalSeconds: 5,
    retentionDays: 7,
    loaded: false,
  }),
  actions: {
    async fetch() {
      const s = await api.getSettings()
      this.pollIntervalSeconds = s.poll_interval_seconds
      this.retentionDays = s.retention_days
      this.loaded = true
    },
    async save(pollIntervalSeconds, retentionDays) {
      const s = await api.putSettings({
        poll_interval_seconds: pollIntervalSeconds,
        retention_days: retentionDays,
      })
      this.pollIntervalSeconds = s.poll_interval_seconds
      this.retentionDays = s.retention_days
    },
  },
})
