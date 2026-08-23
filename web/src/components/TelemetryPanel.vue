<script setup lang="ts">
import type { CraneCard } from '../types'
defineProps<{ card: CraneCard }>()
const units: Record<string, string> = { load: 'kg', radius: 'm', height: 'm', slew_angle: '°', wind_speed: 'm/s', limit_switch: '' }
const display = (kind: string, value?: number) => {
  if (value === undefined) return '--'
  if (kind === 'load') return `${(value / 1_000).toFixed(0)} ${units[kind]}`
  if (['radius', 'height', 'wind_speed'].includes(kind)) return `${(value / 1_000).toFixed(1)} ${units[kind]}`
  if (kind === 'slew_angle') return `${(value / 1_000).toFixed(1)} ${units[kind]}`
  return value ? '触发' : '正常'
}
</script>

<template>
  <div class="telemetry-grid">
    <div v-for="kind in ['load','radius','height','slew_angle','wind_speed','limit_switch']" :key="kind">
      <span>{{ { load:'载重',radius:'幅度',height:'高度',slew_angle:'回转角',wind_speed:'风速',limit_switch:'限位' }[kind] }}</span>
      <strong>{{ display(kind, card.snapshot.readings?.[kind]?.value) }}</strong>
    </div>
  </div>
</template>
