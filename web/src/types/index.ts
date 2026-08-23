export type RiskLevel = 'normal' | 'degraded' | 'warning' | 'alarm' | 'critical'

export interface Reading {
  event_id: string
  kind: string
  value: number
  canonical_unit: string
  observed_at: string
  quality: string
}

export interface Crane {
  id: string
  serial_number: string
  model_id: string
  status: string
  current_driver_id?: string
  responsible_unit_id: string
}

export interface Decision {
  level: RiskLevel
  rule_codes: string[]
  interlock: boolean
  message: string
  evaluated_at: string
}

export interface CraneCard {
  crane: Crane
  snapshot: { at: string; overall_quality: string; readings: Record<string, Reading> }
  decision: Decision
  open_alarms: number
  data_age: number
}

export interface DashboardView {
  site_id: string
  generated_at: string
  cards: CraneCard[]
  next_cursor?: string
}

export interface TimelineEvent {
  id: string
  crane_id: string
  type: string
  at: string
  summary: string
}
