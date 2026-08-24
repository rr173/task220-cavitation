package service_test

import (
	"errors"
	"testing"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/service"
	"task220-cavitation/internal/store"
)

func TestBug05SegmentTimesCannotMoveBackwards(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := service.New(db)
	if err != nil {
		t.Fatal(err)
	}
	trial, err := app.Trials.Create("time probe", "", 1500, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	trial, err = app.Trials.StartAcquisition(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = app.Segments.Ingest(trial, 0, 2000, 100, []float64{0.1, 0.2}); err != nil {
		t.Fatal(err)
	}
	_, err = app.Segments.Ingest(trial, 0, 2000, 50, []float64{0.1, 0.2})
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("backward segment error = %v, want ErrInvalidInput", err)
	}
}
