package maintenance

import (
	"errors"
	"time"
)

type WorkType string

const (
	Inspection  WorkType = "inspection"
	Maintenance WorkType = "maintenance"
	Calibration WorkType = "calibration"
	FaultRepair WorkType = "fault_repair"
)

type WorkStatus string

const (
	WorkPlanned    WorkStatus = "planned"
	WorkInProgress WorkStatus = "in_progress"
	WorkCompleted  WorkStatus = "completed"
	WorkFailed     WorkStatus = "failed"
)

type WorkOrder struct {
	ID             string     `json:"id"`
	SiteID         string     `json:"site_id"`
	CraneID        string     `json:"crane_id"`
	Type           WorkType   `json:"type"`
	Status         WorkStatus `json:"status"`
	Reason         string     `json:"reason"`
	AssignedUnitID string     `json:"assigned_unit_id"`
	DueAt          time.Time  `json:"due_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Result         string     `json:"result,omitempty"`
	EvidenceFileID string     `json:"evidence_file_id,omitempty"`
	Version        int64      `json:"version"`
}

func (w *WorkOrder) Start(actorUnitID string, at time.Time) error {
	if w.Status != WorkPlanned || actorUnitID != w.AssignedUnitID {
		return errors.New("work order cannot be started")
	}
	t := at.UTC()
	w.Status = WorkInProgress
	w.StartedAt = &t
	w.Version++
	return nil
}

func (w *WorkOrder) Complete(result, evidenceFileID string, at time.Time) error {
	if w.Status != WorkInProgress || result == "" {
		return errors.New("work order cannot be completed")
	}
	t := at.UTC()
	w.Status = WorkCompleted
	w.Result = result
	w.EvidenceFileID = evidenceFileID
	w.CompletedAt = &t
	w.Version++
	return nil
}

type StopRecord struct {
	ID          string     `json:"id"`
	CraneID     string     `json:"crane_id"`
	Reason      string     `json:"reason"`
	StoppedAt   time.Time  `json:"stopped_at"`
	StoppedBy   string     `json:"stopped_by"`
	ResumedAt   *time.Time `json:"resumed_at,omitempty"`
	ResumedBy   string     `json:"resumed_by,omitempty"`
	WorkOrderID string     `json:"work_order_id,omitempty"`
}

func (r *StopRecord) Resume(actor string, workCompleted bool, at time.Time) error {
	if r.ResumedAt != nil || actor == "" || !workCompleted {
		return errors.New("crane cannot resume")
	}
	t := at.UTC()
	r.ResumedAt = &t
	r.ResumedBy = actor
	return nil
}
