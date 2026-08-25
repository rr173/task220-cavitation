package service

import (
	"encoding/json"
	"testing"

	"task220-cavitation/internal/model"
)

// evidenceFor 的包级边界测试：起始与消退边界窗口本身必须保留为证据，
// 不能因严格不等而丢失；未检测到消退时取到序列末尾。
func TestEvidenceForIncludesBoundaryWindows(t *testing.T) {
	windows := []model.HarmonicFeatures{
		{ID: "w-before", WindowStartMs: 0},    // 起始之前
		{ID: "w-onset", WindowStartMs: 200},  // 起始边界
		{ID: "w-mid", WindowStartMs: 400},    // 区间中段
		{ID: "w-decay", WindowStartMs: 600},  // 消退边界
		{ID: "w-after", WindowStartMs: 800},  // 消退之后
	}

	t.Run("with decay includes onset and decay boundaries", func(t *testing.T) {
		ev := model.CavitationEvent{OnsetMs: 200, DecayMs: 600}
		got := evidenceFor(windows, ev)
		var ids []string
		if err := json.Unmarshal([]byte(got), &ids); err != nil {
			t.Fatalf("decode evidence: %v", err)
		}
		want := []string{"w-onset", "w-mid", "w-decay"}
		if !equalSlice(ids, want) {
			t.Fatalf("evidence = %v, want %v (boundaries must be retained)", ids, want)
		}
	})

	t.Run("without decay extends to last window", func(t *testing.T) {
		ev := model.CavitationEvent{OnsetMs: 200, DecayMs: 0}
		got := evidenceFor(windows, ev)
		var ids []string
		if err := json.Unmarshal([]byte(got), &ids); err != nil {
			t.Fatalf("decode evidence: %v", err)
		}
		want := []string{"w-onset", "w-mid", "w-decay", "w-after"}
		if !equalSlice(ids, want) {
			t.Fatalf("evidence = %v, want %v (no decay should keep tail)", ids, want)
		}
	})

	t.Run("onset at zero keeps onset window", func(t *testing.T) {
		ev := model.CavitationEvent{OnsetMs: 0, DecayMs: 200}
		got := evidenceFor(windows, ev)
		var ids []string
		if err := json.Unmarshal([]byte(got), &ids); err != nil {
			t.Fatalf("decode evidence: %v", err)
		}
		// 起始在 0 时，w-before（ms==0）即起始窗口，必须保留。
		want := []string{"w-before", "w-onset"}
		if !equalSlice(ids, want) {
			t.Fatalf("evidence = %v, want %v (onset==0 must keep onset window)", ids, want)
		}
	})
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
