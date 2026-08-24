package service

import (
	"errors"
	"testing"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
)

func TestTrialServiceLifecycleAndValidation(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	app, err := New(db)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if _, err := app.Trials.Create("", "", 1500, 200, 0); !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("empty trial name error = %v, want ErrInvalidInput", err)
	}

	trial, err := app.Trials.Create("lifecycle", "state machine", 1500, 200, 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, step := range []func(string) (*model.Trial, error){
		app.Trials.StartAcquisition,
		app.Trials.FinishAcquisition,
		app.Trials.Confirm,
		app.Trials.Seal,
	} {
		trial, err = step(trial.ID)
		if err != nil {
			t.Fatalf("lifecycle step: %v", err)
		}
	}
	if trial.Status != model.TrialSealed {
		t.Fatalf("final status = %s, want sealed", trial.Status)
	}
	if _, err := app.Trials.StartAcquisition(trial.ID); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("sealed transition error = %v, want ErrSealed", err)
	}
}
