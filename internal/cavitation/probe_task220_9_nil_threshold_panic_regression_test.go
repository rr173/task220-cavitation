package cavitation

import (
	"errors"
	"testing"

	"task220-cavitation/internal/model"
)

func TestBug09NilThresholdConfigReturnsInvalidInput(t *testing.T) {
	features := []model.HarmonicFeatures{{GapRatio: 2.0, BroadbandEnergy: 1.0}}
	_, err := NewClassifier().DetectEvents(features, nil, 0)
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}
