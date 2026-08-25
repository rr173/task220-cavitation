package store

import (
	"testing"
	"time"

	"task220-cavitation/internal/model"
)

// TestSegmentStoreNoisyStatusPersisted 锁定机械噪声标记的持久化语义：
// 标记 noisy 不得把片段改成有效片段，也不得在写入后丢失 noisy 状态。
func TestSegmentStoreNoisyStatusPersisted(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := NewSegmentStore(db)
	now := time.Now().UTC()
	seg := &model.AcousticSegment{
		ID: "seg-noisy-rt", TrialID: "trial-noisy", ChannelIndex: 2,
		SampleRateHz: 2000, StartTimeMs: 0, DurationMs: 200,
		Samples: "[0.1]", PeakAmplitude: 0.5, RMS: 0.3, Fingerprint: "fp-noisy",
		Status: model.SegmentPendingCalibration, CreatedAt: now,
	}
	if err := s.Insert(seg); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// 标记为机械噪声：状态应写入 noisy 本身，而非被改写成 valid。
	if err := s.UpdateStatus(seg.ID, model.SegmentNoisy); err != nil {
		t.Fatalf("mark noisy: %v", err)
	}
	got, err := s.Get(seg.ID)
	if err != nil {
		t.Fatalf("get after noisy: %v", err)
	}
	if got.Status != model.SegmentNoisy {
		t.Fatalf("status after mark noisy = %q, want %q (must not be rewritten to valid)", got.Status, model.SegmentNoisy)
	}

	// noisy 片段不得进入有效片段集合（特征提取应排除噪声通道）。
	valid, err := s.ListValidByTrial(seg.TrialID)
	if err != nil {
		t.Fatalf("list valid: %v", err)
	}
	for _, v := range valid {
		if v.ID == seg.ID {
			t.Fatalf("noisy segment %s must not appear in valid segments", seg.ID)
		}
	}

	// 标记 noisy 后再切回有效（例如撤回噪声标记）应原样写入 valid，
	// 验证 UpdateStatus 不再对 noisy 做单向映射。
	if err := s.UpdateStatus(seg.ID, model.SegmentValid); err != nil {
		t.Fatalf("re-validate: %v", err)
	}
	got, err = s.Get(seg.ID)
	if err != nil {
		t.Fatalf("get after re-validate: %v", err)
	}
	if got.Status != model.SegmentValid {
		t.Fatalf("status after re-validate = %q, want %q", got.Status, model.SegmentValid)
	}
}
