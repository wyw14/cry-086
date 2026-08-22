import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

const safetyConsoleRoutes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: () => import('../pages/LoginPage.vue'), meta: { anonymous: true } },
  { path: '/', name: 'live-safety', component: () => import('../pages/DashboardPage.vue') },
  { path: '/replay', name: 'evidence-replay', component: () => import('../pages/ReplayPage.vue') },
  { path: '/maintenance', name: 'equipment-assurance', component: () => import('../pages/MaintenancePage.vue') },
  { path: '/reports', name: 'regulatory-reports', component: () => import('../pages/ReportsPage.vue') }
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: safetyConsoleRoutes,
  scrollBehavior: () => ({ top: 0 })
})

router.beforeEach(destination => {
  const signedIn = Boolean(sessionStorage.getItem('access_token'))
  if (!destination.meta.anonymous && !signedIn) return { name: 'login', query: { redirect: destination.fullPath } }
  if (destination.name === 'login' && signedIn) return { name: 'live-safety' }
  return true
})
export default router
