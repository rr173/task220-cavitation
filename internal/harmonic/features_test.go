package harmonic

import (
	"math"
	"math/rand"
	"testing"
)

// genTestWindow 生成一个窗口的螺旋桨声纹抽样（与冒烟测试同源）。
func genTestWindow(rng *rand.Rand, cavitating bool) []float64 {
	const sampleRate = 2000.0
	const windowMs = 200
	const f0 = 100.0
	n := int(sampleRate * windowMs / 1000.0)
	samples := make([]float64, n)
	noiseScale := 0.04
	if cavitating {
		noiseScale = 0.45
	}
	for i := 0; i < n; i++ {
		t := float64(i) / sampleRate
		v := math.Sin(2*math.Pi*f0*t)
		for k := 2; k <= 6; k++ {
			v += (0.35 / float64(k)) * math.Sin(2*math.Pi*f0*float64(k)*t)
		}
		v += rng.NormFloat64() * noiseScale
		samples[i] = v
	}
	return samples
}

func TestEstimateFundamental(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	samples := genTestWindow(rng, false)
	f0 := EstimateFundamental(samples, 2000)
	if f0 < 90 || f0 > 110 {
		t.Fatalf("fundamental estimate = %.1f, want near 100 Hz", f0)
	}
}

func TestComputeFeaturesDistinguishesCavitation(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	normal, _ := ComputeFeatures(genTestWindow(rng, false), 2000)
	cav, _ := ComputeFeatures(genTestWindow(rng, true), 2000)
	if normal.GapRatio >= cav.GapRatio {
		t.Fatalf("cavitation gap (%.4f) should exceed normal gap (%.4f)", cav.GapRatio, normal.GapRatio)
	}
	if normal.GapRatio > 0.05 {
		t.Fatalf("normal gap too high: %.4f", normal.GapRatio)
	}
	if cav.GapRatio < 0.15 {
		t.Fatalf("cavitation gap too low: %.4f", cav.GapRatio)
	}
}

func TestAutocorrelationZeroLag(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	samples := genTestWindow(rng, false)
	r0 := Autocorrelation(samples, 0)
	if math.Abs(r0-1.0) > 1e-9 {
		t.Fatalf("R(0) should be 1.0, got %.6f", r0)
	}
}
