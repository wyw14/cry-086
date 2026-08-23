import { request } from './http'
import type { DashboardView, TimelineEvent } from '../types'

export function loadDashboard(siteID: string, sort = 'risk'): Promise<DashboardView> {
  return request(`/sites/${encodeURIComponent(siteID)}/dashboard?limit=50&sort=${encodeURIComponent(sort)}`)
}

export function loadReplay(siteID: string, start: Date, end: Date): Promise<TimelineEvent[]> {
  const query = new URLSearchParams({ start: start.toISOString(), end: end.toISOString(), limit: '500' })
  return request(`/sites/${encodeURIComponent(siteID)}/replay?${query}`)
}

export function simulateFleet(siteID: string, velocities: Record<string, [number, number]>) {
  return request(`/sites/${encodeURIComponent(siteID)}/fleet/simulate`, { method: 'POST', body: JSON.stringify({ velocities }) })
}
