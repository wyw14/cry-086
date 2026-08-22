package safety

import (
	"errors"
	"sort"
	"time"

	"github.com/wyw14/cry-086/internal/domain/sensor"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
)

type RiskLevel string

const (
	RiskNormal   RiskLevel = "normal"
	RiskDegraded RiskLevel = "degraded"
	RiskWarning  RiskLevel = "warning"
	RiskAlarm    RiskLevel = "alarm"
	RiskCritical RiskLevel = "critical"
)

type LoadCurvePoint struct {
	RadiusMM int64 `json:"radius_mm"`
	MaxLoadG int64 `json:"max_load_g"`
}

type ExclusionZone struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	PolygonMM []Point `json:"polygon_mm"`
	MinZMM    int64   `json:"min_z_mm"`
	MaxZMM    int64   `json:"max_z_mm"`
}

type Point struct {
	X int64 `json:"x"`
	Y int64 `json:"y"`
}

type Config struct {
	ID                  string           `json:"id"`
	CraneModelID        string           `json:"crane_model_id"`
	Version             int64            `json:"version"`
	EffectiveAt         time.Time        `json:"effective_at"`
	LoadCurve           []LoadCurvePoint `json:"load_curve"`
	ExclusionZones      []ExclusionZone  `json:"exclusion_zones"`
	WarningWindMilliMPS int64            `json:"warning_wind_milli_mps"`
	StopWindMilliMPS    int64            `json:"stop_wind_milli_mps"`
	RecoveryHold        time.Duration    `json:"recovery_hold"`
	CombinationMargin   int64            `json:"combination_margin_percent"`
}

func (c Config) Validate() error {
	if c.ID == "" || c.CraneModelID == "" || c.Version < 1 || len(c.LoadCurve) < 2 {
		return errors.New("safety configuration is incomplete")
	}
	if c.WarningWindMilliMPS <= 0 || c.StopWindMilliMPS <= c.WarningWindMilliMPS {
		return errors.New("wind thresholds are invalid")
	}
	previousRadius := int64(-1)
	for _, point := range c.LoadCurve {
		if point.RadiusMM <= previousRadius || point.MaxLoadG <= 0 {
			return errors.New("load curve must be strictly ordered")
		}
		previousRadius = point.RadiusMM
	}
	return nil
}

func (c Config) RatedLoad(radiusMM int64) (int64, bool) {
	points := append([]LoadCurvePoint(nil), c.LoadCurve...)
	sort.Slice(points, func(i, j int) bool { return points[i].RadiusMM < points[j].RadiusMM })
	for _, point := range points {
		if radiusMM <= point.RadiusMM {
			return point.MaxLoadG, true
		}
	}
	return 0, false
}

type Decision struct {
	Level          RiskLevel `json:"level"`
	RuleCodes      []string  `json:"rule_codes"`
	Interlock      bool      `json:"interlock"`
	Message        string    `json:"message"`
	ConfigID       string    `json:"config_id"`
	ConfigVersion  int64     `json:"config_version"`
	EvaluatedAt    time.Time `json:"evaluated_at"`
	EvidenceEvents []string  `json:"evidence_events"`
}

func Evaluate(config Config, snapshot telemetry.Snapshot, now time.Time) Decision {
	decision := Decision{Level: RiskNormal, ConfigID: config.ID, ConfigVersion: config.Version, EvaluatedAt: now.UTC()}
	if snapshot.OverallQuality != telemetry.QualityGood {
		decision.Level = RiskDegraded
		decision.RuleCodes = append(decision.RuleCodes, "DATA_QUALITY_UNCERTAIN")
		decision.Message = "telemetry quality cannot be confirmed"
		return decision
	}
	for _, reading := range snapshot.Readings {
		decision.EvidenceEvents = append(decision.EvidenceEvents, reading.EventID)
	}
	load := snapshot.Readings[sensor.Load].Value
	radius := snapshot.Readings[sensor.Radius].Value
	wind := snapshot.Readings[sensor.WindSpeed].Value
	limit := snapshot.Readings[sensor.LimitSwitch].LimitEngaged
	rated, ok := config.RatedLoad(radius)
	if !ok {
		decision.Level = RiskCritical
		decision.Interlock = true
		decision.RuleCodes = append(decision.RuleCodes, "RADIUS_OUTSIDE_CURVE")
	}
	if ok && load*100 >= rated*100 {
		decision.Level = RiskCritical
		decision.Interlock = true
		decision.RuleCodes = append(decision.RuleCodes, "RATED_LOAD_EXCEEDED")
	} else if ok && load*100 >= rated*(100-config.CombinationMargin) {
		decision.Level = RiskWarning
		decision.RuleCodes = append(decision.RuleCodes, "LOAD_MARGIN_LOW")
	}
	if wind >= config.StopWindMilliMPS {
		decision.Level = RiskCritical
		decision.Interlock = true
		decision.RuleCodes = append(decision.RuleCodes, "WIND_STOP_THRESHOLD")
	} else if wind >= config.WarningWindMilliMPS && decision.Level == RiskNormal {
		decision.Level = RiskWarning
		decision.RuleCodes = append(decision.RuleCodes, "WIND_WARNING_THRESHOLD")
	}
	if limit {
		decision.Level = RiskCritical
		decision.Interlock = true
		decision.RuleCodes = append(decision.RuleCodes, "LIMIT_SWITCH_ENGAGED")
	}
	if decision.Message == "" {
		decision.Message = "safety rules evaluated"
	}
	return decision
}
