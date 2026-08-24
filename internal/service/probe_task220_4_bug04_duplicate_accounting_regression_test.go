package service_test

import (
	"testing"

	"task220-cavitation/internal/service"
	"task220-cavitation/internal/store"
)

func TestBug04DuplicateSegmentIsNotCountedAsInserted(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := service.New(db)
	if err != nil {
		t.Fatal(err)
	}
	trial, err := app.Trials.Create("duplicate probe", "", 1500, 200, 0)
	if err != nil {
		t.Fatal(err)
	}
	trial, err = app.Trials.StartAcquisition(trial.ID)
	if err != nil {
		t.Fatal(err)
	}
	samples := []float64{0.1, -0.2, 0.3, -0.1}
	first, err := app.Segments.Ingest(trial, 0, 2000, 0, samples)
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.Segments.Ingest(trial, 0, 2000, 0, samples)
	if err != nil {
		t.Fatal(err)
	}
	if first.Inserted != 1 || first.Duplicate != 0 || second.Inserted != 0 || second.Duplicate != 1 {
		t.Fatalf("ingest results = first %+v second %+v, want inserted=1 then duplicate=1", first, second)
	}
}
