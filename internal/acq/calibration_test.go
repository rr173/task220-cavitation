package acq

import (
	"math"
	"math/rand"
	"testing"

	"task220-cavitation/internal/model"
)

func TestCrossCorrelateFindsDelay(t *testing.T) {
	// 生成参考信号，并制造一个滞后 10 样本的副本。
	rng := rand.New(rand.NewSource(7))
	n := 400
	ref := make([]float64, n)
	for i := range ref {
		ref[i] = math.Sin(2*math.Pi*float64(i)/20) + rng.NormFloat64()*0.05
	}
	lagWant := 10
	other := make([]float64, n+lagWant)
	for i := range other {
		idx := i - lagWant
		if idx >= 0 && idx < n {
			other[i] = ref[idx]
		} else {
			other[i] = rng.NormFloat64() * 0.05
		}
	}
	lag, peak := CrossCorrelate(ref, other)
	if lag != lagWant {
		t.Fatalf("best lag = %d, want %d", lag, lagWant)
	}
	if peak < 0.8 {
		t.Fatalf("correlation peak too low: %.3f", peak)
	}
}

func TestCalibratorDelayMs(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	ref := make([]float64, 400)
	for i := range ref {
		ref[i] = math.Sin(2*math.Pi*float64(i)/20) + rng.NormFloat64()*0.05
	}
	// 延迟 13 样本 @ 2000 Hz ≈ 6.5 ms（非整周期，避免正弦周期歧义）。
	other := make([]float64, 413)
	for i := range other {
		if i >= 13 {
			other[i] = ref[i-13]
		} else {
			other[i] = rng.NormFloat64() * 0.05
		}
	}
	c := NewCalibrator()
	d, err := c.Calibrate("t1", 1, ref, other, 2000)
	if err != nil {
		t.Fatalf("calibrate: %v", err)
	}
	if d.Status != model.DelayLocked {
		t.Fatalf("status = %s, want locked", d.Status)
	}
	if math.Abs(d.DelayMs-6.5) > 0.5 {
		t.Fatalf("delay = %.2f ms, want ~6.5 ms", d.DelayMs)
	}
}

func TestCalibrateScoreNeverNegative(t *testing.T) {
	// 反相信号：互相关峰值呈强负值（最佳对齐为反极性），
	// 但质量分数必须保持非负且归一化。
	rng := rand.New(rand.NewSource(11))
	n := 400
	ref := make([]float64, n)
	for i := range ref {
		ref[i] = math.Sin(2*math.Pi*float64(i)/20) + rng.NormFloat64()*0.05
	}
	other := make([]float64, n)
	for i := range other {
		other[i] = -ref[i]
	}
	c := NewCalibrator()
	d, err := c.Calibrate("t-inv", 2, ref, other, 2000)
	if err != nil {
		t.Fatalf("calibrate: %v", err)
	}
	if d.CorrelationScore < 0 || d.CorrelationScore > 1 {
		t.Fatalf("correlation score = %.3f, must be in [0,1]", d.CorrelationScore)
	}
	if d.CorrelationScore < 0.8 {
		t.Fatalf("strong anti-phase alignment should yield high score, got %.3f", d.CorrelationScore)
	}
	if d.Status != model.DelayLocked {
		t.Fatalf("status = %s, want locked for strong anti-phase alignment", d.Status)
	}
}

func TestValidateRejectsInvalid(t *testing.T) {
	seg := &model.AcousticSegment{SampleRateHz: 0, DurationMs: 100, ChannelIndex: 0}
	if err := Validate(seg); err == nil {
		t.Fatalf("zero sample rate should be rejected")
	}
	seg2 := &model.AcousticSegment{SampleRateHz: 2000, DurationMs: 100, ChannelIndex: -1}
	if err := Validate(seg2); err == nil {
		t.Fatalf("negative channel should be rejected")
	}
}
