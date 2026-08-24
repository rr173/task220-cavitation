package service_test

import (
	"encoding/json"
	"testing"
	"time"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/service"
	"task220-cavitation/internal/store"
)

func TestBug01PublishedEventsKeepChronologicalOrder(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := service.New(db)
	if err != nil {
		t.Fatal(err)
	}
	trial, err := app.Trials.Create("order probe", "", 1500, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.Trials.StartAcquisition(trial.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = app.Trials.FinishAcquisition(trial.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = app.Trials.Confirm(trial.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	es := store.NewEventStore(db)
	for _, ev := range []model.CavitationEvent{
		{ID: "evt-late", TrialID: trial.ID, Stage: model.EventDecay, OnsetMs: 200, Confidence: 0.8, CreatedAt: now, UpdatedAt: now},
		{ID: "evt-early", TrialID: trial.ID, Stage: model.EventInception, OnsetMs: 100, Confidence: 0.7, CreatedAt: now, UpdatedAt: now},
	} {
		if err := es.Insert(&ev); err != nil {
			t.Fatal(err)
		}
	}
	pkg, err := app.Packages.Publish(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	var events []model.CavitationEvent
	if err := json.Unmarshal([]byte(pkg.EventsJSON), &events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].OnsetMs != 100 || events[1].OnsetMs != 200 {
		t.Fatalf("published event snapshot order = %+v, want onset 100 then 200", events)
	}
}
