<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { DataAnalysis, Files, Histogram, Monitor, SwitchButton, Tools } from '@element-plus/icons-vue'
import { useSessionStore } from './stores/session'
const route = useRoute(); const router = useRouter(); const session = useSessionStore(); const standalone = computed(() => route.path === '/login')
function logout() { session.logout(); router.push('/login') }
</script>

<template>
  <router-view v-if="standalone" />
  <div v-else class="app-shell">
    <aside class="app-sidebar"><div class="app-brand"><span class="brand-symbol"><i /></span><div><strong>塔安智控</strong><small>CRANE GUARD</small></div></div><nav><router-link to="/"><el-icon><Monitor/></el-icon>实时驾驶舱</router-link><router-link to="/replay"><el-icon><DataAnalysis/></el-icon>事件回放</router-link><router-link to="/maintenance"><el-icon><Tools/></el-icon>设备保障</router-link><router-link to="/reports"><el-icon><Files/></el-icon>监管报告</router-link></nav><button class="logout" @click="logout"><el-icon><SwitchButton/></el-icon>退出登录</button></aside>
    <main class="app-main"><router-view /></main>
  </div>
</template>
