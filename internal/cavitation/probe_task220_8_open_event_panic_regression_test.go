package cavitation

import (
	"testing"

	"task220-cavitation/internal/model"
)

func TestBug08OpenEndedEventDoesNotPanic(t *testing.T) {
	features := make([]model.HarmonicFeatures, 6)
	for i := range features {
		features[i] = model.HarmonicFeatures{
			WindowStartMs:   int64(i * 100),
			GapRatio:        2.0,
			BroadbandEnergy: 1.0,
		}
	}
	cfg := &model.ThresholdConfig{GapRatioThreshold: 0.5, EnergyFloor: 0.1, ConfirmWindows: 2}
	events, err := NewClassifier().DetectEvents(features, cfg, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want one open-ended event", len(events))
	}
	if events[0].DecayMs != 0 {
		t.Fatalf("decay_ms = %d, want zero for an event without a decay window", events[0].DecayMs)
	}
}
