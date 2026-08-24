package store

import (
	"testing"
	"time"

	"task220-cavitation/internal/model"
)

func TestTrialStoreRoundTripAndStatusGuard(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := NewTrialStore(db)
	now := time.Now().UTC()
	want := &model.Trial{
		ID: "trial-store-test", Name: "store round trip", Description: "test",
		ShaftSpeedRPM: 1500, InflowPressureKPa: 200, ReferenceChannel: 0,
		Status: model.TrialPreparing, Fingerprint: "fingerprint-store-test",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Create(want); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Get(want.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != want.Name || got.Status != model.TrialPreparing || got.ReferenceChannel != 0 {
		t.Fatalf("round trip mismatch: got %+v", got)
	}

	updated, err := s.UpdateStatus(want.ID, model.TrialPreparing, model.TrialAcquiring)
	if err != nil || !updated {
		t.Fatalf("expected guarded status update, updated=%v err=%v", updated, err)
	}
	updated, err = s.UpdateStatus(want.ID, model.TrialPreparing, model.TrialAnalyzing)
	if err != nil || updated {
		t.Fatalf("stale status update should be rejected, updated=%v err=%v", updated, err)
	}
}
