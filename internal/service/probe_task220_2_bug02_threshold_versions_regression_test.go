package service_test

import (
	"testing"

	"task220-cavitation/internal/service"
	"task220-cavitation/internal/store"
)

func TestBug02ThresholdVersionsRemainMonotonic(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app, err := service.New(db)
	if err != nil {
		t.Fatal(err)
	}
	first, err := app.Packages.AddThreshold(0.20, 0.002, 3)
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.Packages.AddThreshold(0.25, 0.003, 4)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 2 || second.Version != 3 {
		t.Fatalf("threshold versions = %d, %d, want 2, 3", first.Version, second.Version)
	}
	all, err := app.Packages.ListThresholds()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Version != 1 || all[1].Version != 2 || all[2].Version != 3 {
		t.Fatalf("persisted threshold versions = %+v, want [1 2 3]", all)
	}
}
