package report

import "time"

type ShiftWindow struct {
	SiteID string    `json:"site_id"`
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
}

func (w ShiftWindow) Valid() bool {
	return w.SiteID != "" && w.Start.Before(w.End) && w.End.Sub(w.Start) <= 24*time.Hour
}

type ShiftStatistics struct {
	SiteID             string        `json:"site_id"`
	Start              time.Time     `json:"start"`
	End                time.Time     `json:"end"`
	CraneCount         int           `json:"crane_count"`
	TelemetryCount     int64         `json:"telemetry_count"`
	DegradedCount      int64         `json:"degraded_count"`
	WarningCount       int64         `json:"warning_count"`
	AlarmCount         int64         `json:"alarm_count"`
	InterlockCount     int64         `json:"interlock_count"`
	AcknowledgementP95 time.Duration `json:"acknowledgement_p95"`
	OperatingDuration  time.Duration `json:"operating_duration"`
}

type RegulatoryReport struct {
	ID           string          `json:"id"`
	SiteID       string          `json:"site_id"`
	Window       ShiftWindow     `json:"window"`
	Statistics   ShiftStatistics `json:"statistics"`
	EvidenceIDs  []string        `json:"evidence_ids"`
	GeneratedAt  time.Time       `json:"generated_at"`
	GeneratedBy  string          `json:"generated_by"`
	ContentHash  string          `json:"content_hash"`
	RetentionEnd time.Time       `json:"retention_end"`
}

type TimelineEvent struct {
	ID       string         `json:"id"`
	CraneID  string         `json:"crane_id"`
	Type     string         `json:"type"`
	At       time.Time      `json:"at"`
	Summary  string         `json:"summary"`
	Evidence map[string]any `json:"evidence"`
}
