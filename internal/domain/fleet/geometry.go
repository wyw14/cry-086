package fleet

import (
	"math"
	"time"
)

type Pose struct {
	CraneID       string    `json:"crane_id"`
	PivotXMM      int64     `json:"pivot_x_mm"`
	PivotYMM      int64     `json:"pivot_y_mm"`
	JibRadiusMM   int64     `json:"jib_radius_mm"`
	AngleMilliDeg int64     `json:"angle_millideg"`
	HookHeightMM  int64     `json:"hook_height_mm"`
	ObservedAt    time.Time `json:"observed_at"`
}

type Relation struct {
	CraneAID          string `json:"crane_a_id"`
	CraneBID          string `json:"crane_b_id"`
	HorizontalClearMM int64  `json:"horizontal_clearance_mm"`
	VerticalClearMM   int64  `json:"vertical_clearance_mm"`
	Enabled           bool   `json:"enabled"`
	Version           int64  `json:"version"`
}

type CollisionRisk struct {
	CraneAID      string    `json:"crane_a_id"`
	CraneBID      string    `json:"crane_b_id"`
	DistanceMM    int64     `json:"distance_mm"`
	VerticalGapMM int64     `json:"vertical_gap_mm"`
	PredictedAt   time.Time `json:"predicted_at"`
	Critical      bool      `json:"critical"`
}

func HookPosition(p Pose) (float64, float64) {
	angle := float64(p.AngleMilliDeg) / 1000 * math.Pi / 180
	x := float64(p.PivotXMM) + float64(p.JibRadiusMM)*math.Cos(angle)
	y := float64(p.PivotYMM) + float64(p.JibRadiusMM)*math.Sin(angle)
	return x, y
}

func EvaluateRelation(relation Relation, a, b Pose, at time.Time) CollisionRisk {
	ax, ay := HookPosition(a)
	bx, by := HookPosition(b)
	distance := int64(math.Round(math.Hypot(ax-bx, ay-by)))
	vertical := abs(a.HookHeightMM - b.HookHeightMM)
	return CollisionRisk{CraneAID: a.CraneID, CraneBID: b.CraneID, DistanceMM: distance, VerticalGapMM: vertical, PredictedAt: at.UTC(), Critical: relation.Enabled && distance < relation.HorizontalClearMM && vertical < relation.VerticalClearMM}
}

func Project(p Pose, angleVelocityMilliDeg, hoistVelocityMM int64, step time.Duration) Pose {
	seconds := step.Seconds()
	p.AngleMilliDeg += int64(float64(angleVelocityMilliDeg) * seconds)
	p.HookHeightMM += int64(float64(hoistVelocityMM) * seconds)
	p.ObservedAt = p.ObservedAt.Add(step)
	return p
}

func abs(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
