package acq

import (
	"math/rand"
	"testing"
)

func TestBug10CorrelationScoreIsAlwaysNonNegative(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	ref := make([]float64, 128)
	other := make([]float64, len(ref))
	for i := range ref {
		ref[i] = rng.NormFloat64()
		other[i] = -ref[i]
	}
	d, err := NewCalibrator().Calibrate("score-probe", 1, ref, other, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if d.CorrelationScore < 0 || d.CorrelationScore > 1 {
		t.Fatalf("correlation score = %.3f, want a normalized non-negative score", d.CorrelationScore)
	}
}
