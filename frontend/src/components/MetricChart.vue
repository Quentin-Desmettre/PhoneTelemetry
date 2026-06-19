<script setup>
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { Chart } from 'chart.js'

const props = defineProps({
  title: { type: String, required: true },
  unit: { type: String, default: '' },
  // [{ label, color, points: [{ ts, value }] }]
  series: { type: Array, default: () => [] },
  range: { type: String, default: '24h' },
  yMax: { type: Number, default: null },
})

const canvas = ref(null)
let chart = null

function fmt(ts) {
  const d = new Date(ts * 1000)
  const p = (n) => String(n).padStart(2, '0')
  if (props.range === 'live') return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
  if (props.range === '7d') return `${p(d.getDate())}/${p(d.getMonth() + 1)} ${p(d.getHours())}:${p(d.getMinutes())}`
  return `${p(d.getHours())}:${p(d.getMinutes())}`
}

// Align all series onto a shared, sorted timestamp axis (missing -> null).
function buildData() {
  const tsSet = new Set()
  for (const s of props.series) for (const pt of s.points || []) tsSet.add(pt.ts)
  const axis = [...tsSet].sort((a, b) => a - b)

  const datasets = props.series.map((s) => {
    const map = new Map((s.points || []).map((p) => [p.ts, p.value]))
    return {
      label: s.label,
      data: axis.map((t) => (map.has(t) ? map.get(t) : null)),
      borderColor: s.color,
      backgroundColor: s.color + '22',
      borderWidth: 2,
      pointRadius: 0,
      tension: 0.35,
      fill: props.series.length === 1,
      spanGaps: true,
    }
  })
  return { labels: axis.map(fmt), datasets }
}

function render() {
  const data = buildData()
  if (chart) {
    chart.data = data
    chart.options.scales.y.max = props.yMax ?? undefined
    chart.update('none')
    return
  }
  chart = new Chart(canvas.value, {
    type: 'line',
    data,
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: {
          display: props.series.length > 1,
          labels: { color: '#94a3b8', boxWidth: 12, usePointStyle: true },
        },
        tooltip: {
          callbacks: {
            label: (c) => `${c.dataset.label}: ${c.parsed.y}${props.unit}`,
          },
        },
      },
      scales: {
        x: {
          ticks: { color: '#64748b', maxTicksLimit: 6, maxRotation: 0 },
          grid: { color: 'rgba(148,163,184,0.08)' },
        },
        y: {
          min: 0,
          max: props.yMax ?? undefined,
          ticks: { color: '#64748b', callback: (v) => `${v}${props.unit}` },
          grid: { color: 'rgba(148,163,184,0.08)' },
        },
      },
    },
  })
}

onMounted(render)
watch(() => [props.series, props.range, props.yMax], render, { deep: true })
onBeforeUnmount(() => chart && chart.destroy())
</script>

<template>
  <div class="card p-5">
    <h3 class="mb-4 text-sm font-medium text-slate-300">{{ title }}</h3>
    <div class="h-60">
      <canvas ref="canvas" />
    </div>
  </div>
</template>
