package service_test

import (
	"testing"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/service"
	"task220-cavitation/internal/store"
)

func TestBug06NoiseAnnotationPersistsWithoutChangingValidity(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := service.New(db)
	if err != nil {
		t.Fatal(err)
	}
	trial, err := app.Trials.Create("noise state probe", "", 1500, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	trial, err = app.Trials.StartAcquisition(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Segments.Ingest(trial, 0, 2000, 0, []float64{0.1, 0.2}); err != nil {
		t.Fatal(err)
	}
	segments, err := app.Segments.ListSegments(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Segments.MarkNoisy(segments[0].ID); err != nil {
		t.Fatal(err)
	}
	stored, err := app.Segments.ListSegments(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored[0].Status != model.SegmentNoisy {
		t.Fatalf("stored segment status = %s, want noisy", stored[0].Status)
	}
}
