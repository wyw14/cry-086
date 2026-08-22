<script setup lang="ts">
import { ref } from 'vue'
import { loadReplay } from '../services/dashboard'
import { useSessionStore } from '../stores/session'
import type { TimelineEvent } from '../types'
const session = useSessionStore()
const events = ref<TimelineEvent[]>([])
const loading = ref(false)
async function load() { loading.value = true; const end = new Date(); const start = new Date(end.getTime() - 8*60*60*1000); try { events.value = await loadReplay(session.siteID, start, end) } finally { loading.value = false } }
load()
</script>

<template><div class="content-page"><header class="page-header"><div><h2>事件回放</h2><p>按不可变证据时间线查看安全处置过程</p></div><el-button @click="load" :loading="loading">重新加载</el-button></header><el-timeline><el-timeline-item v-for="event in events" :key="event.id" :timestamp="new Date(event.at).toLocaleString()" placement="top"><strong>{{ event.type }}</strong><p>{{ event.crane_id }} · {{ event.summary }}</p></el-timeline-item></el-timeline></div></template>
