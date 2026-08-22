package fleet

import (
	"testing"
	"time"
)

func TestEvaluateRelationDetectsVerticalAndHorizontalConflict(t *testing.T) {
	relation := Relation{CraneAID: "a", CraneBID: "b", HorizontalClearMM: 15_000, VerticalClearMM: 5_000, Enabled: true}
	a := Pose{CraneID: "a", PivotXMM: 0, JibRadiusMM: 40_000, HookHeightMM: 50_000}
	b := Pose{CraneID: "b", PivotXMM: 70_000, JibRadiusMM: 40_000, AngleMilliDeg: 180_000, HookHeightMM: 52_000}
	risk := EvaluateRelation(relation, a, b, time.Now())
	if !risk.Critical {
		t.Fatalf("risk = %#v, want critical", risk)
	}
	b.HookHeightMM = 80_000
	if EvaluateRelation(relation, a, b, time.Now()).Critical {
		t.Fatal("large vertical separation remained critical")
	}
}
