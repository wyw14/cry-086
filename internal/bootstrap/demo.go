package bootstrap

import (
	"context"
	"strings"
	"time"

	authapp "github.com/wyw14/cry-086/internal/application/auth"
	"github.com/wyw14/cry-086/internal/domain/fleet"
	"github.com/wyw14/cry-086/internal/domain/identity"
	"github.com/wyw14/cry-086/internal/domain/maintenance"
	"github.com/wyw14/cry-086/internal/domain/safety"
	"github.com/wyw14/cry-086/internal/domain/sensor"
	"github.com/wyw14/cry-086/internal/domain/site"
)

type demoRepository interface {
	CreateSite(context.Context, site.Site) error
	CreateCrane(context.Context, site.TowerCrane) error
	SeedSensor(sensor.Sensor)
	SeedCalibration(sensor.Calibration)
	SeedConfig(safety.Config)
	SeedUser(identity.User)
	SeedRelation(string, fleet.Relation)
	SeedPose(fleet.Pose)
	SeedWorkOrder(maintenance.WorkOrder)
}

func seedDemo(ctx context.Context, repository demoRepository) error {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	demoSite, _ := site.NewSite("site-demo", "Riverside Transit Hub", "Asia/Shanghai", "unit-builder", 365, now)
	if err := repository.CreateSite(ctx, demoSite); err != nil && !strings.Contains(err.Error(), "already") && !strings.Contains(err.Error(), "duplicate") {
		return err
	}
	craneA, _ := site.NewCrane("crane-a", demoSite.ID, "TC-A-001", "model-zt6015", "config-zt6015", "unit-lift", 3, site.Position{XMillimeters: 0, YMillimeters: 0, BaseHeightMM: 0}, now)
	craneB, _ := site.NewCrane("crane-b", demoSite.ID, "TC-B-002", "model-zt6015", "config-zt6015", "unit-lift", 3, site.Position{XMillimeters: 70_000, YMillimeters: 0, BaseHeightMM: 0}, now)
	_ = craneA.Transition(site.CraneRunning, now)
	_ = craneB.Transition(site.CraneRunning, now)
	for _, crane := range []site.TowerCrane{craneA, craneB} {
		if err := repository.CreateCrane(ctx, crane); err != nil && !strings.Contains(err.Error(), "already") && !strings.Contains(err.Error(), "duplicate") {
			return err
		}
	}
	config := safety.Config{ID: "config-zt6015", CraneModelID: "model-zt6015", Version: 3, EffectiveAt: now, WarningWindMilliMPS: 12_000, StopWindMilliMPS: 15_000, RecoveryHold: 30 * time.Second, CombinationMargin: 10, LoadCurve: []safety.LoadCurvePoint{{RadiusMM: 20_000, MaxLoadG: 6_000_000}, {RadiusMM: 40_000, MaxLoadG: 3_000_000}, {RadiusMM: 60_000, MaxLoadG: 1_500_000}}}
	repository.SeedConfig(config)
	kinds := []struct {
		kind sensor.Kind
		unit string
		min  int64
		max  int64
	}{{sensor.Load, "g", 0, 8_000_000}, {sensor.Radius, "mm", 0, 70_000}, {sensor.Height, "mm", 0, 100_000}, {sensor.SlewAngle, "millideg", -360_000, 360_000}, {sensor.WindSpeed, "mm/s", 0, 60_000}, {sensor.LimitSwitch, "bool", 0, 1}}
	for _, crane := range []site.TowerCrane{craneA, craneB} {
		for _, spec := range kinds {
			id := crane.ID + "-" + string(spec.kind)
			calibrationID := "cal-" + id
			repository.SeedSensor(sensor.Sensor{ID: id, CraneID: crane.ID, Kind: spec.kind, PointCode: string(spec.kind), NativeUnit: spec.unit, CanonicalUnit: spec.unit, MinValue: spec.min, MaxValue: spec.max, CalibrationID: calibrationID, CalibrationVer: 1, Communication: sensor.CommunicationOnline, LastSeenAt: now, Version: 1})
			repository.SeedCalibration(sensor.Calibration{ID: calibrationID, SensorID: id, Version: 1, ScaleMicros: 1_000_000, CertifiedBy: "local-calibration-lab", EffectiveAt: now.AddDate(-1, 0, 0), ExpiresAt: now.AddDate(2, 0, 0)})
		}
	}
	passwordHash, err := authapp.HashPassword("CraneGuard!2026")
	if err != nil {
		return err
	}
	repository.SeedUser(identity.User{ID: "user-safety", SiteIDs: []string{demoSite.ID}, Name: "Demo Safety Officer", Username: "safety", PasswordHash: passwordHash, Roles: []identity.Role{identity.RoleSafetyOfficer, identity.RoleDispatcher, identity.RoleMaintainer, identity.RoleRegulator}, Active: true, CreatedAt: now, Version: 1})
	repository.SeedRelation(demoSite.ID, fleet.Relation{CraneAID: craneA.ID, CraneBID: craneB.ID, HorizontalClearMM: 12_000, VerticalClearMM: 8_000, Enabled: true, Version: 1})
	repository.SeedPose(fleet.Pose{CraneID: craneA.ID, PivotXMM: 0, PivotYMM: 0, JibRadiusMM: 40_000, AngleMilliDeg: 0, HookHeightMM: 50_000, ObservedAt: now})
	repository.SeedPose(fleet.Pose{CraneID: craneB.ID, PivotXMM: 70_000, PivotYMM: 0, JibRadiusMM: 40_000, AngleMilliDeg: 180_000, HookHeightMM: 52_000, ObservedAt: now})
	repository.SeedWorkOrder(maintenance.WorkOrder{ID: "work-demo", SiteID: demoSite.ID, CraneID: craneA.ID, Type: maintenance.Inspection, Status: maintenance.WorkInProgress, Reason: "scheduled shift inspection", AssignedUnitID: "unit-lift", DueAt: now.Add(8 * time.Hour), Version: 1})
	return nil
}
