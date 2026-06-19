<script setup>
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useRouter, RouterLink } from 'vue-router'
import { useMetricsStore } from '../stores/metrics'
import { useSettingsStore } from '../stores/settings'
import { useAuthStore } from '../stores/auth'
import MetricCard from '../components/MetricCard.vue'
import MetricChart from '../components/MetricChart.vue'
import RangeSelector from '../components/RangeSelector.vue'
import ChargingBadge from '../components/ChargingBadge.vue'

const router = useRouter()
const metrics = useMetricsStore()
const settings = useSettingsStore()
const auth = useAuthStore()

let currentTimer = null
let historyTimer = null

function handleError(e) {
  if (e && e.status === 401) {
    auth.authenticated = false
    router.push({ name: 'login' })
  }
}

async function pollCurrent() {
  try {
    await metrics.fetchCurrent()
  } catch (e) {
    handleError(e)
  }
  currentTimer = setTimeout(pollCurrent, Math.max(1, settings.pollIntervalSeconds) * 1000)
}

onMounted(async () => {
  try {
    await settings.fetch()
    await metrics.fetchHistory()
  } catch (e) {
    handleError(e)
  }
  pollCurrent()
  historyTimer = setInterval(() => metrics.fetchHistory().catch(handleError), 30000)
})

onBeforeUnmount(() => {
  clearTimeout(currentTimer)
  clearInterval(historyTimer)
})

async function logout() {
  await auth.logout().catch(() => {})
  router.push({ name: 'login' })
}

// --- formatting helpers ---
function fmtBytes(n) {
  if (!n && n !== 0) return '—'
  const u = ['o', 'Ko', 'Mo', 'Go', 'To']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 || i === 0 ? 0 : 1)} ${u[i]}`
}
const pct = (v) => (v == null ? '—' : v.toFixed(1))

function barColor(p) {
  if (p == null) return 'bg-accent'
  if (p >= 90) return 'bg-rose-500'
  if (p >= 75) return 'bg-amber-500'
  return 'bg-emerald-500'
}
function batteryColor(p) {
  if (p == null) return 'bg-accent'
  if (p <= 15) return 'bg-rose-500'
  if (p <= 35) return 'bg-amber-500'
  return 'bg-emerald-500'
}

const cur = computed(() => metrics.current)
const hasBattery = computed(() => cur.value && cur.value.battery_pct != null)
const tempList = computed(() => {
  const t = (cur.value && cur.value.temps) || {}
  return Object.entries(t).map(([name, value]) => ({ name, value }))
})

const lastUpdated = computed(() => {
  if (!cur.value) return ''
  return new Date(cur.value.ts * 1000).toLocaleTimeString('fr-FR')
})

// --- chart series ---
const palette = ['#6366f1', '#10b981', '#f59e0b', '#ec4899', '#06b6d4', '#a78bfa', '#ef4444', '#84cc16']

const systemSeries = computed(() => {
  const h = metrics.history
  if (!h) return []
  return [
    { label: 'CPU', color: '#6366f1', points: h.cpu || [] },
    { label: 'RAM', color: '#10b981', points: h.mem || [] },
    { label: 'Disque', color: '#f59e0b', points: h.disk || [] },
  ]
})
const batterySeries = computed(() => {
  const h = metrics.history
  if (!h || !h.battery || !h.battery.length) return []
  return [{ label: 'Batterie', color: '#10b981', points: h.battery }]
})
const tempSeries = computed(() => {
  const h = metrics.history
  if (!h || !h.temps) return []
  return Object.entries(h.temps).map(([name, points], i) => ({
    label: name,
    color: palette[i % palette.length],
    points,
  }))
})

function setRange(r) {
  metrics.setRange(r).catch(handleError)
}
</script>

<template>
  <div class="mx-auto max-w-6xl p-4 sm:p-6">
    <!-- Header -->
    <header class="mb-6 flex flex-wrap items-center justify-between gap-4">
      <div>
        <h1 class="text-xl font-semibold text-white">📱 Phone Dashboard</h1>
        <p class="text-xs text-slate-500">
          <span v-if="lastUpdated">MAJ {{ lastUpdated }} · </span>rafraîchissement {{ settings.pollIntervalSeconds }}s
        </p>
      </div>
      <div class="flex items-center gap-2">
        <RouterLink :to="{ name: 'settings' }" class="btn-ghost">⚙️ Réglages</RouterLink>
        <button class="btn-ghost" @click="logout">Déconnexion</button>
      </div>
    </header>

    <div v-if="!cur" class="card p-10 text-center text-slate-400">
      Collecte des premières mesures…
    </div>

    <template v-else>
      <!-- Current value cards -->
      <section class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <MetricCard label="CPU" icon="🧠" :value="pct(cur.cpu_pct)" unit="%" :percent="cur.cpu_pct" :color="barColor(cur.cpu_pct)" />
        <MetricCard
          label="Mémoire"
          icon="📊"
          :value="pct(cur.mem_pct)"
          unit="%"
          :percent="cur.mem_pct"
          :color="barColor(cur.mem_pct)"
          :sub="`${fmtBytes(cur.mem_used)} / ${fmtBytes(cur.mem_total)}`"
        />
        <MetricCard
          label="Disque"
          icon="💾"
          :value="pct(cur.disk_pct)"
          unit="%"
          :percent="cur.disk_pct"
          :color="barColor(cur.disk_pct)"
          :sub="`${fmtBytes(cur.disk_used)} / ${fmtBytes(cur.disk_total)} · ${fmtBytes(cur.disk_total - cur.disk_used)} libre`"
        />
        <MetricCard
          v-if="hasBattery"
          label="Batterie"
          icon="🔋"
          :value="pct(cur.battery_pct)"
          unit="%"
          :percent="cur.battery_pct"
          :color="batteryColor(cur.battery_pct)"
        >
          <template #badge>
            <ChargingBadge :charging="!!cur.battery_charging" />
          </template>
        </MetricCard>
      </section>

      <!-- Temperature cards -->
      <section v-if="tempList.length" class="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
        <MetricCard
          v-for="t in tempList"
          :key="t.name"
          :label="t.name"
          icon="🌡️"
          :value="t.value.toFixed(1)"
          unit="°C"
        />
      </section>

      <!-- Range selector -->
      <div class="mt-8 mb-4 flex items-center justify-between">
        <h2 class="text-lg font-semibold text-white">Historique</h2>
        <RangeSelector :model-value="metrics.range" @update:model-value="setRange" />
      </div>

      <!-- Charts -->
      <section class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <MetricChart title="Système (CPU / RAM / Disque)" unit="%" :y-max="100" :series="systemSeries" :range="metrics.range" />
        <MetricChart v-if="batterySeries.length" title="Batterie" unit="%" :y-max="100" :series="batterySeries" :range="metrics.range" />
        <MetricChart v-if="tempSeries.length" title="Températures" unit="°C" :series="tempSeries" :range="metrics.range" class="xl:col-span-2" />
      </section>
    </template>
  </div>
</template>
