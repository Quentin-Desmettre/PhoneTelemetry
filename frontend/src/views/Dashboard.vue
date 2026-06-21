<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
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

// Aggregate stats (min/max/avg) over the current temperature sensors.
const tempStats = computed(() => {
  const list = tempList.value
  if (!list.length) return null
  let max = list[0]
  let min = list[0]
  let sum = 0
  for (const t of list) {
    if (t.value > max.value) max = t
    if (t.value < min.value) min = t
    sum += t.value
  }
  return {
    count: list.length,
    max,
    min,
    avg: sum / list.length,
  }
})

// Whether the full per-sensor detail (cards + chart) is expanded.
const showAllTemps = ref(false)

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

// Min/max/avg across all sensors, computed per shared timestamp.
const tempAggSeries = computed(() => {
  const h = metrics.history
  if (!h || !h.temps) return []
  const buckets = new Map() // ts -> [values]
  for (const points of Object.values(h.temps)) {
    for (const pt of points || []) {
      if (!buckets.has(pt.ts)) buckets.set(pt.ts, [])
      buckets.get(pt.ts).push(pt.value)
    }
  }
  if (!buckets.size) return []
  const maxPts = []
  const minPts = []
  const avgPts = []
  for (const [ts, vals] of buckets) {
    const max = Math.max(...vals)
    const min = Math.min(...vals)
    const avg = vals.reduce((a, b) => a + b, 0) / vals.length
    maxPts.push({ ts, value: +max.toFixed(1) })
    minPts.push({ ts, value: +min.toFixed(1) })
    avgPts.push({ ts, value: +avg.toFixed(1) })
  }
  return [
    { label: 'Max', color: '#ef4444', points: maxPts },
    { label: 'Moyenne', color: '#6366f1', points: avgPts },
    { label: 'Min', color: '#06b6d4', points: minPts },
  ]
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

      <!-- Temperature summary (min / max / avg) -->
      <section v-if="tempStats" class="mt-4">
        <div class="mb-3 flex items-center justify-between">
          <h2 class="text-sm font-medium text-slate-400">
            🌡️ Températures
            <span class="text-slate-600">· {{ tempStats.count }} capteurs</span>
          </h2>
          <button class="btn-ghost text-xs" @click="showAllTemps = !showAllTemps">
            {{ showAllTemps ? 'Masquer les capteurs' : 'Afficher tous les capteurs' }}
          </button>
        </div>

        <div class="grid grid-cols-3 gap-4">
          <MetricCard
            label="Temp. max"
            icon="🔥"
            :value="tempStats.max.value.toFixed(1)"
            unit="°C"
            :sub="tempStats.max.name"
          />
          <MetricCard
            label="Temp. moyenne"
            icon="🌡️"
            :value="tempStats.avg.toFixed(1)"
            unit="°C"
          />
          <MetricCard
            label="Temp. min"
            icon="❄️"
            :value="tempStats.min.value.toFixed(1)"
            unit="°C"
            :sub="tempStats.min.name"
          />
        </div>

        <!-- Detailed per-sensor cards (collapsed by default) -->
        <div v-if="showAllTemps" class="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          <MetricCard
            v-for="t in tempList"
            :key="t.name"
            :label="t.name"
            icon="🌡️"
            :value="t.value.toFixed(1)"
            unit="°C"
          />
        </div>
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
        <MetricChart
          v-if="tempAggSeries.length"
          title="Températures (min / moy / max)"
          unit="°C"
          :series="tempAggSeries"
          :range="metrics.range"
          class="xl:col-span-2"
        />
        <MetricChart
          v-if="showAllTemps && tempSeries.length"
          title="Températures (tous les capteurs)"
          unit="°C"
          :series="tempSeries"
          :range="metrics.range"
          class="xl:col-span-2"
        />
      </section>
    </template>
  </div>
</template>
