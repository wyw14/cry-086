package maintenance

import (
	"testing"
	"time"
)

func TestCalibrationWorkRequiresEvidence(t *testing.T) {
	order := WorkOrder{ID: "work", Type: Calibration, Status: WorkInProgress, Version: 1}
	if err := order.Complete("calibrated", "", time.Now()); err == nil {
		t.Fatal("calibration completed without evidence")
	}
	if err := order.Complete("calibrated", "file-1", time.Now()); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
}

func TestStopRecordResumesOnce(t *testing.T) {
	record := StopRecord{ID: "stop", CraneID: "crane"}
	if err := record.Resume("officer", false, time.Now()); err == nil {
		t.Fatal("resume without completed work succeeded")
	}
	if err := record.Resume("officer", true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := record.Resume("officer", true, time.Now()); err == nil {
		t.Fatal("second resume succeeded")
	}
}
