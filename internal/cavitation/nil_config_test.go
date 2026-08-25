package cavitation

import (
	"errors"
	"testing"

	"task220-cavitation/internal/model"
)

// TestDetectEventsNilConfigReturnsInvalidInput 阈值配置缺失时空化事件分析
// 必须返回 invalid input 错误，而非解引用 nil 配置触发 panic。
func TestDetectEventsNilConfigReturnsInvalidInput(t *testing.T) {
	features := []model.HarmonicFeatures{
		{WindowStartMs: 0, GapRatio: 0.5, BroadbandEnergy: 0.5},
		{WindowStartMs: 200, GapRatio: 0.5, BroadbandEnergy: 0.5},
		{WindowStartMs: 400, GapRatio: 0.5, BroadbandEnergy: 0.5},
	}
	c := NewClassifier()

	// recover 兜底：若实现错误地解引用 nil，panic 会让本测试失败而非通过。
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DetectEvents panicked on nil config: %v", r)
		}
	}()

	_, err := c.DetectEvents(features, nil, 0)
	if !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
}

// TestJudgeWindowsNilConfigReturnsInvalidInput 直接覆盖底层越界判定器，
// 防止绕过分类器调用 JudgeWindows/Detect 时再次 panic。
func TestJudgeWindowsNilConfigReturnsInvalidInput(t *testing.T) {
	d := NewDetector()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("JudgeWindows panicked on nil config: %v", r)
		}
	}()
	flags := d.JudgeWindows(
		[]model.HarmonicFeatures{{WindowStartMs: 0, GapRatio: 0.5, BroadbandEnergy: 0.5}},
		nil,
	)
	if len(flags) != 0 {
		t.Fatalf("expected no flags for nil config, got %d", len(flags))
	}
}
