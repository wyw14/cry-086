import { defineStore } from 'pinia'
import { loadDashboard } from '../services/dashboard'
import type { DashboardView } from '../types'

export const useDashboardStore = defineStore('dashboard', {
  state: () => ({ view: null as DashboardView | null, loading: false, error: '' }),
  actions: {
    async refresh(siteID: string) {
      this.loading = true
      this.error = ''
      try { this.view = await loadDashboard(siteID) }
      catch (error) { this.error = error instanceof Error ? error.message : '驾驶舱加载失败' }
      finally { this.loading = false }
    }
  }
})
