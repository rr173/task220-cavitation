package cavitation

import (
	"testing"

	"task220-cavitation/internal/model"
)

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{model.EventCandidate, model.EventInception, true},
		{model.EventInception, model.EventSustained, true},
		{model.EventSustained, model.EventDecay, true},
		{model.EventCandidate, model.EventRejected, true},
		{model.EventDecay, model.EventRejected, true},
		{model.EventDecay, model.EventSustained, false}, // 不可回退
		{model.EventRejected, model.EventCandidate, false},
		{model.EventSustained, model.EventInception, false},
	}
	for _, c := range cases {
		if got := CanTransition(c.from, c.to); got != c.want {
			t.Errorf("CanTransition(%s,%s)=%v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestTransition(t *testing.T) {
	ev := &model.CavitationEvent{Stage: model.EventCandidate}
	if err := Transition(ev, model.EventInception); err != nil {
		t.Fatalf("legal transition failed: %v", err)
	}
	if ev.Stage != model.EventInception {
		t.Fatalf("stage = %s, want inception", ev.Stage)
	}
	if err := Transition(ev, model.EventSustained); err != nil {
		t.Fatalf("legal transition failed: %v", err)
	}
	if err := Transition(ev, model.EventInception); err == nil {
		t.Fatalf("backward transition should fail")
	}
}

func TestDetectFindsOnsetAndDecay(t *testing.T) {
	// 构造 20 个窗口：前 5 个正常，中 10 个空化，后 5 个消退。
	features := make([]model.HarmonicFeatures, 20)
	for i := range features {
		gap := 0.01
		if i >= 5 && i < 15 {
			gap = 0.5
		}
		features[i] = model.HarmonicFeatures{
			WindowStartMs: int64(i * 200),
			GapRatio:      gap,
			BroadbandEnergy: gap,
		}
	}
	cfg := &model.ThresholdConfig{GapRatioThreshold: 0.15, EnergyFloor: 0.001, ConfirmWindows: 3}
	d := NewDetector()
	onset, sustained, decay, found := d.Detect(features, cfg)
	if !found {
		t.Fatalf("event not found")
	}
	// 平滑窗口（半径 2）会使边界前移/后移至多 2 个窗口，用区间断言而非精确值。
	if onset < 3 || onset > 5 {
		t.Fatalf("onset = %d, want in [3,5]", onset)
	}
	if sustained <= onset {
		t.Fatalf("sustained = %d should exceed onset %d", sustained, onset)
	}
	if decay <= sustained {
		t.Fatalf("decay = %d should exceed sustained %d", decay, sustained)
	}
}
