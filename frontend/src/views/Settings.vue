<script setup>
import { onMounted, ref } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useSettingsStore } from '../stores/settings'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const settings = useSettingsStore()
const auth = useAuthStore()

const poll = ref(5)
const retention = ref(7)
const error = ref('')
const saved = ref(false)
const busy = ref(false)

onMounted(async () => {
  try {
    await settings.fetch()
    poll.value = settings.pollIntervalSeconds
    retention.value = settings.retentionDays
  } catch (e) {
    if (e && e.status === 401) router.push({ name: 'login' })
  }
})

async function save() {
  error.value = ''
  saved.value = false
  busy.value = true
  try {
    await settings.save(Number(poll.value), Number(retention.value))
    saved.value = true
  } catch (e) {
    error.value = e.message || 'Échec de l’enregistrement.'
    if (e && e.status === 401) router.push({ name: 'login' })
  } finally {
    busy.value = false
  }
}

async function logout() {
  await auth.logout().catch(() => {})
  router.push({ name: 'login' })
}
</script>

<template>
  <div class="mx-auto max-w-2xl p-4 sm:p-6">
    <header class="mb-6 flex items-center justify-between">
      <h1 class="text-xl font-semibold text-white">⚙️ Réglages</h1>
      <RouterLink :to="{ name: 'dashboard' }" class="btn-ghost">← Retour</RouterLink>
    </header>

    <div class="card space-y-6 p-6">
      <div>
        <label class="label">Fréquence de collecte (secondes)</label>
        <input v-model.number="poll" type="number" min="1" max="3600" class="input" />
        <p class="mt-1.5 text-xs text-slate-500">
          Intervalle entre deux mesures. Par défaut 5 s. Appliqué immédiatement.
        </p>
      </div>

      <div>
        <label class="label">Rétention des données (jours)</label>
        <input v-model.number="retention" type="number" min="1" max="365" class="input" />
        <p class="mt-1.5 text-xs text-slate-500">
          Les mesures plus anciennes sont automatiquement supprimées. Par défaut 7 jours.
        </p>
      </div>

      <p v-if="error" class="text-sm text-rose-400">{{ error }}</p>
      <p v-if="saved" class="text-sm text-emerald-400">Réglages enregistrés ✓</p>

      <div class="flex items-center justify-between">
        <button class="btn-primary" :disabled="busy" @click="save">
          {{ busy ? 'Enregistrement…' : 'Enregistrer' }}
        </button>
        <button class="btn-ghost" @click="logout">Déconnexion</button>
      </div>
    </div>
  </div>
</template>
