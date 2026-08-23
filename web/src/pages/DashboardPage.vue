<script setup lang="ts">
import { computed } from 'vue'
import { useDashboardStore } from '../stores/dashboard'
import { useSessionStore } from '../stores/session'
import { usePolling } from '../composables/usePolling'
import RiskBadge from '../components/RiskBadge.vue'
import CraneMap from '../components/CraneMap.vue'
import TelemetryPanel from '../components/TelemetryPanel.vue'
const session = useSessionStore()
const dashboard = useDashboardStore()
usePolling(() => dashboard.refresh(session.siteID), 5000)
const cards = computed(() => dashboard.view?.cards ?? [])
const totals = computed(() => ({ cranes: cards.value.length, high: cards.value.filter(card => ['alarm','critical'].includes(card.decision.level)).length, degraded: cards.value.filter(card => card.decision.level === 'degraded').length, alarms: cards.value.reduce((sum, card) => sum + card.open_alarms, 0) }))
</script>

<template>
  <div class="dashboard-page" v-loading="dashboard.loading && !dashboard.view">
    <header class="page-header"><div><h2>实时驾驶舱</h2><p>{{ session.siteID }} · {{ dashboard.view?.generated_at ? new Date(dashboard.view.generated_at).toLocaleString() : '等待数据' }}</p></div><el-button :loading="dashboard.loading" @click="dashboard.refresh(session.siteID)">刷新</el-button></header>
    <el-alert v-if="dashboard.error" :title="dashboard.error" type="error" show-icon />
    <section class="summary-strip"><div><span>在册塔机</span><strong>{{ totals.cranes }}</strong></div><div><span>高危设备</span><strong class="danger">{{ totals.high }}</strong></div><div><span>降级监测</span><strong class="warning">{{ totals.degraded }}</strong></div><div><span>待处置告警</span><strong>{{ totals.alarms }}</strong></div></section>
    <CraneMap :cards="cards" />
    <section class="crane-list">
      <article v-for="card in cards" :key="card.crane.id" class="crane-card">
        <header><div><h3>{{ card.crane.serial_number }}</h3><p>{{ card.crane.model_id }} · {{ card.crane.status }}</p></div><RiskBadge :level="card.decision.level" /></header>
        <TelemetryPanel :card="card" />
        <footer><span>{{ card.decision.message }}</span><strong v-if="card.decision.interlock">联锁建议已触发</strong></footer>
      </article>
    </section>
  </div>
</template>
