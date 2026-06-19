<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await auth.login(username.value, password.value)
    router.push({ name: 'dashboard' })
  } catch (e) {
    error.value = e.message || 'Identifiants invalides.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex min-h-full items-center justify-center p-6">
    <div class="card w-full max-w-md p-8">
      <div class="mb-6 text-center">
        <h1 class="text-2xl font-semibold text-white">Phone Dashboard</h1>
        <p class="mt-2 text-sm text-slate-400">Connectez-vous pour accéder aux métriques.</p>
      </div>
      <form class="space-y-4" @submit.prevent="submit">
        <div>
          <label class="label">Nom d'utilisateur</label>
          <input v-model="username" class="input" autocomplete="username" required />
        </div>
        <div>
          <label class="label">Mot de passe</label>
          <input v-model="password" type="password" class="input" autocomplete="current-password" required />
        </div>
        <p v-if="error" class="text-sm text-rose-400">{{ error }}</p>
        <button class="btn-primary w-full" :disabled="busy">
          {{ busy ? 'Connexion…' : 'Se connecter' }}
        </button>
      </form>
    </div>
  </div>
</template>
