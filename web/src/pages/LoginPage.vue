<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useSessionStore } from '../stores/session'
const username = ref('safety')
const password = ref('CraneGuard!2026')
const pending = ref(false)
const error = ref('')
const session = useSessionStore()
const router = useRouter()
async function submit() {
  pending.value = true; error.value = ''
  try { await session.login(username.value, password.value); await router.push('/') }
  catch (reason) { error.value = reason instanceof Error ? reason.message : '登录失败' }
  finally { pending.value = false }
}
</script>

<template>
  <main class="login-page">
    <section class="login-shell">
      <div class="brand-mark"><span class="mast" /><span class="jib" /></div>
      <h1>塔安智控</h1><p>智慧工地塔机运行安全监测</p>
      <el-form @submit.prevent="submit">
        <el-form-item><el-input v-model="username" aria-label="用户名" placeholder="用户名" /></el-form-item>
        <el-form-item><el-input v-model="password" aria-label="密码" type="password" show-password placeholder="密码" /></el-form-item>
        <el-alert v-if="error" :title="error" type="error" :closable="false" />
        <el-button native-type="submit" type="primary" :loading="pending">进入监控台</el-button>
      </el-form>
    </section>
  </main>
</template>
