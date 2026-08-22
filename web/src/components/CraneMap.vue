<script setup lang="ts">
import { computed } from 'vue'
import type { CraneCard } from '../types'
const props = defineProps<{ cards: CraneCard[] }>()
const points = computed(() => props.cards.map((card, index) => ({ card, x: 18 + (index % 4) * 25, y: 28 + Math.floor(index / 4) * 38 })))
</script>

<template>
  <section class="crane-map" aria-label="群塔空间态势">
    <div class="site-grid" />
    <div v-for="point in points" :key="point.card.crane.id" class="crane-point" :class="`point-${point.card.decision.level}`" :style="{ left: `${point.x}%`, top: `${point.y}%` }">
      <span class="crane-mast" /><span class="crane-jib" />
      <strong>{{ point.card.crane.serial_number }}</strong>
    </div>
    <div class="map-legend"><span>本地坐标系</span><span>实时风险叠加</span></div>
  </section>
</template>
