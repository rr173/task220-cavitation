package service

import (
	"testing"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
)

// TestSegmentMarkNoisyKeepsStatusAndWeight 锁定机械噪声标记语义：
// 标记只改变片段状态为 noisy 并据此降权，不得把片段改成有效片段，
// 也不得在持久化后丢失 noisy 状态（噪声片段不参与特征提取、计入噪声通道）。
func TestSegmentMarkNoisyKeepsStatusAndWeight(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	app, err := New(db)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	trial, err := app.Trials.Create("noisy mark", "status guard", 1500, 200, 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := app.Trials.StartAcquisition(trial.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	trial, err = app.Trials.Get(trial.ID)
	if err != nil {
		t.Fatalf("get after start: %v", err)
	}
	if trial.Status != model.TrialAcquiring {
		t.Fatalf("trial status = %s, want acquiring", trial.Status)
	}

	// 两个通道各一个片段；通道 1 将被标记为机械噪声。
	ingest := func(ch int, startMs int64) {
		samples := make([]float64, 20)
		if _, err := app.Segments.Ingest(trial, ch, 2000, startMs, samples); err != nil {
			t.Fatalf("ingest ch=%d: %v", ch, err)
		}
	}
	ingest(0, 0)
	ingest(1, 0)

	segs, err := app.Segments.ListSegments(trial.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var noisyID string
	for _, seg := range segs {
		if seg.ChannelIndex == 1 {
			noisyID = seg.ID
		}
	}
	if noisyID == "" {
		t.Fatalf("no segment on channel 1 to mark")
	}

	if err := app.Segments.MarkNoisy(noisyID); err != nil {
		t.Fatalf("mark noisy: %v", err)
	}

	marked, err := app.Segments.store.Get(noisyID)
	if err != nil {
		t.Fatalf("get marked: %v", err)
	}
	if marked.Status != model.SegmentNoisy {
		t.Fatalf("marked segment status = %q, want %q (must not become valid)", marked.Status, model.SegmentNoisy)
	}

	// 噪声通道计数驱动置信度降权：标记后应为 1。
	noisyChannels, err := app.Segments.CountNoisyChannels(trial.ID)
	if err != nil {
		t.Fatalf("count noisy: %v", err)
	}
	if noisyChannels != 1 {
		t.Fatalf("noisy channels = %d, want 1 (noisy status lost on persist)", noisyChannels)
	}

	// 噪声片段不得进入有效片段集合（特征提取应排除噪声通道）。
	valid, err := app.Segments.store.ListValidByTrial(trial.ID)
	if err != nil {
		t.Fatalf("list valid: %v", err)
	}
	for _, v := range valid {
		if v.ID == noisyID {
			t.Fatalf("noisy segment must not appear in valid segments")
		}
	}

	// 重复标记应幂等保持 noisy，不得翻转为其它状态。
	if err := app.Segments.MarkNoisy(noisyID); err != nil {
		t.Fatalf("re-mark noisy: %v", err)
	}
	marked, err = app.Segments.store.Get(noisyID)
	if err != nil {
		t.Fatalf("get re-marked: %v", err)
	}
	if marked.Status != model.SegmentNoisy {
		t.Fatalf("re-marked status = %q, want %q", marked.Status, model.SegmentNoisy)
	}
}
