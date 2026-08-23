<script setup lang="ts">
import type { RiskLevel } from '../types'
defineProps<{ level: RiskLevel }>()
const labels: Record<RiskLevel, string> = { normal: '正常', degraded: '降级', warning: '预警', alarm: '报警', critical: '高危' }
</script>

<template><span class="risk-badge" :class="`risk-${level}`"><i />{{ labels[level] }}</span></template>
