<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: { type: String, required: true },
  icon: { type: String, default: '' },
  value: { type: [String, Number], default: '—' },
  unit: { type: String, default: '' },
  sub: { type: String, default: '' },
  // percent (0-100) drives the progress bar; null hides it.
  percent: { type: Number, default: null },
  color: { type: String, default: 'bg-accent' },
})

const barWidth = computed(() => {
  if (props.percent == null) return '0%'
  return Math.max(0, Math.min(100, props.percent)) + '%'
})
</script>

<template>
  <div class="card p-5">
    <div class="flex items-start justify-between">
      <div class="flex items-center gap-2 text-sm text-slate-400">
        <span v-if="icon" class="text-base">{{ icon }}</span>
        <span>{{ label }}</span>
      </div>
      <slot name="badge" />
    </div>
    <div class="mt-3 flex items-baseline gap-1">
      <span class="text-3xl font-semibold tabular-nums text-white">{{ value }}</span>
      <span v-if="unit" class="text-sm text-slate-400">{{ unit }}</span>
    </div>
    <div v-if="percent != null" class="mt-4 h-2 w-full overflow-hidden rounded-full bg-ink-900">
      <div class="h-full rounded-full transition-all duration-500" :class="color" :style="{ width: barWidth }" />
    </div>
    <p v-if="sub" class="mt-3 text-xs text-slate-500">{{ sub }}</p>
  </div>
</template>
