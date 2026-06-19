<script setup>
defineProps({ charging: { type: Boolean, default: false } })
</script>

<template>
  <span
    class="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-medium"
    :class="charging ? 'bg-emerald-500/15 text-emerald-300' : 'bg-slate-500/15 text-slate-400'"
  >
    <span>{{ charging ? '⚡' : '🔋' }}</span>
    {{ charging ? 'En charge' : 'Sur batterie' }}
  </span>
</template>
