package service_test

import (
	"testing"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/service"
	"task220-cavitation/internal/store"
)

func TestBug03MarkedNoiseChannelIsCounted(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := service.New(db)
	if err != nil {
		t.Fatal(err)
	}
	trial, err := app.Trials.Create("noise probe", "", 1500, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	trial, err = app.Trials.StartAcquisition(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.Segments.Ingest(trial, 2, 2000, 0, []float64{0.1, 0.2})
	if err != nil || res.Inserted != 1 {
		t.Fatalf("ingest = %+v err=%v", res, err)
	}
	segments, err := app.Segments.ListSegments(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Segments.MarkNoisy(segments[0].ID); err != nil {
		t.Fatal(err)
	}
	if segments[0].Status == model.SegmentNoisy {
		t.Fatal("list snapshot should not be mutated in place")
	}
	count, err := app.Segments.CountNoisyChannels(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("noisy channel count = %d, want 1", count)
	}
}
