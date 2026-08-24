package service_test

import (
	"encoding/json"
	"testing"
	"time"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/service"
	"task220-cavitation/internal/store"
)

func TestBug07PublishedPackagePreservesEvidenceSnapshot(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := service.New(db)
	if err != nil {
		t.Fatal(err)
	}
	trial, err := app.Trials.Create("evidence probe", "", 1500, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	trial, err = app.Trials.StartAcquisition(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	trial, err = app.Trials.FinishAcquisition(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	trial, err = app.Trials.Confirm(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ev := &model.CavitationEvent{ID: "evt-evidence", TrialID: trial.ID, Stage: model.EventDecay, OnsetMs: 100, DecayMs: 300, EvidenceSegments: `["seg-1","seg-2"]`, CreatedAt: now, UpdatedAt: now}
	if err := store.NewEventStore(db).Insert(ev); err != nil {
		t.Fatal(err)
	}
	pkg, err := app.Packages.Publish(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	var events []model.CavitationEvent
	if err := json.Unmarshal([]byte(pkg.EventsJSON), &events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EvidenceSegments != `["seg-1","seg-2"]` {
		t.Fatalf("in-memory snapshot evidence = %+v", events)
	}
	stored, err := app.Packages.GetPackage(pkg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.EventsJSON != pkg.EventsJSON {
		t.Fatalf("stored package snapshot = %s, want %s", stored.EventsJSON, pkg.EventsJSON)
	}
}
