<script setup>
const props = defineProps({
  modelValue: { type: String, required: true },
})
const emit = defineEmits(['update:modelValue'])

const options = [
  { value: 'live', label: 'Live' },
  { value: '1h', label: '1h' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7 jours' },
]
</script>

<template>
  <div class="inline-flex rounded-xl bg-ink-800 p-1 ring-1 ring-white/5">
    <button
      v-for="opt in options"
      :key="opt.value"
      class="rounded-lg px-3 py-1.5 text-sm font-medium transition-colors"
      :class="modelValue === opt.value ? 'bg-accent text-white' : 'text-slate-400 hover:text-slate-200'"
      @click="emit('update:modelValue', opt.value)"
    >
      {{ opt.label }}
    </button>
  </div>
</template>
