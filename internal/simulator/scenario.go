package simulator

import (
	"fmt"
	"time"

	"github.com/wyw14/cry-086/internal/domain/sensor"
	"github.com/wyw14/cry-086/internal/domain/telemetry"
)

type Scenario struct {
	CraneID string
	NodeID  string
	Start   time.Time
	Step    time.Duration
	Samples int
}

func (s Scenario) Readings() []telemetry.RawReading {
	if s.Samples < 1 {
		s.Samples = 12
	}
	if s.Step <= 0 {
		s.Step = time.Second
	}
	readings := make([]telemetry.RawReading, 0, s.Samples*6)
	for index := 0; index < s.Samples; index++ {
		at := s.Start.Add(time.Duration(index) * s.Step)
		values := []struct {
			kind  sensor.Kind
			value int64
			unit  string
			limit bool
		}{
			{sensor.Load, 2_000 + int64(index*80), "kg", false},
			{sensor.Radius, 35 + int64(index/6), "m", false},
			{sensor.Height, 52, "m", false},
			{sensor.SlewAngle, int64(index * 4), "deg", false},
			{sensor.WindSpeed, 8 + int64(index/8), "m/s", false},
			{sensor.LimitSwitch, 0, "bool", false},
		}
		for offset, value := range values {
			sequence := int64(index*len(values) + offset + 1)
			readings = append(readings, telemetry.RawReading{EventID: fmt.Sprintf("%s-%06d", s.CraneID, sequence), CraneID: s.CraneID, SensorID: s.CraneID + "-" + string(value.kind), Kind: value.kind, Value: value.value, Unit: value.unit, Sequence: sequence, ObservedAt: at, LimitEngaged: value.limit, SimulatorNode: s.NodeID})
		}
	}
	return readings
}
