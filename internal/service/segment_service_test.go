package service

import (
	"errors"
	"testing"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
)

// TestSegmentIngestDuplicateIsReportedNotInserted 验证：重复提交同一试验/通道/时间窗的
// 声纹片段时，接口必须报告 duplicate（而非 inserted），且数据库中只保留一份片段。
func TestSegmentIngestDuplicateIsReportedNotInserted(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	trial, err := app.Trials.Create("dup trial", "duplicate ingest", 1500, 200, 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	trial, err = app.Trials.StartAcquisition(trial.ID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	samples := []float64{0.1, 0.2, 0.3, 0.4}
	const channel = 1
	const startMs = int64(500)

	// 首次提交：新增一份。
	first, err := app.Segments.Ingest(trial, channel, 2000, startMs, samples)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.Inserted != 1 || first.Duplicate != 0 {
		t.Fatalf("first ingest = inserted=%d duplicate=%d, want 1/0", first.Inserted, first.Duplicate)
	}

	// 重复提交同一试验/通道/时间窗：必须报告 duplicate，而非新增。
	dup, err := app.Segments.Ingest(trial, channel, 2000, startMs, samples)
	if err != nil {
		t.Fatalf("duplicate ingest: %v", err)
	}
	if dup.Inserted != 0 || dup.Duplicate != 1 {
		t.Fatalf("duplicate ingest = inserted=%d duplicate=%d, want 0/1", dup.Inserted, dup.Duplicate)
	}

	// 数据库中只应保留一份该 (trial, channel, start) 的片段。
	segs, err := app.Segments.ListSegments(trial.ID)
	if err != nil {
		t.Fatalf("list segments: %v", err)
	}
	var count int
	for _, s := range segs {
		if s.ChannelIndex == channel && s.StartTimeMs == startMs {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 persisted segment for (channel=%d, start=%d), got %d", channel, startMs, count)
	}

	// 去重仅针对三元组：不同通道、不同时间窗仍可正常新增。
	other, err := app.Segments.Ingest(trial, channel+1, 2000, startMs, samples)
	if err != nil {
		t.Fatalf("other-channel ingest: %v", err)
	}
	if other.Inserted != 1 || other.Duplicate != 0 {
		t.Fatalf("other-channel ingest = inserted=%d duplicate=%d, want 1/0", other.Inserted, other.Duplicate)
	}
	later, err := app.Segments.Ingest(trial, channel, 2000, startMs+200, samples)
	if err != nil {
		t.Fatalf("later-window ingest: %v", err)
	}
	if later.Inserted != 1 || later.Duplicate != 0 {
		t.Fatalf("later-window ingest = inserted=%d duplicate=%d, want 1/0", later.Inserted, later.Duplicate)
	}

	// 上述新增均应真正落库，合计三份。
	segs, err = app.Segments.ListSegments(trial.ID)
	if err != nil {
		t.Fatalf("list segments 2: %v", err)
	}
	if len(segs) != 3 {
		t.Fatalf("expected 3 persisted segments total, got %d", len(segs))
	}

	// 防回归：时间倒退（非重复）仍应被 ErrInvalidInput 拒绝。
	if _, err := app.Segments.Ingest(trial, channel, 2000, startMs-100, samples); !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("backwards time should be ErrInvalidInput, got %v", err)
	}
}

// TestSegmentStoreInsertDuplicateGuardsConcurrency 验证存储层并发路径：跳过 service
// 预检查直接重复插入时，Insert 返回 ErrDuplicate（非静默成功），确保即使先读后写
// 存在窗口，数据库仍只保留一份。
func TestSegmentStoreInsertDuplicateGuardsConcurrency(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	ts := store.NewTrialStore(db)
	if err := ts.Create(&model.Trial{
		ID: "t-dup", Name: "dup", ShaftSpeedRPM: 1, InflowPressureKPa: 1,
		Status: model.TrialAcquiring, Fingerprint: "fp-dup",
	}); err != nil {
		t.Fatalf("create trial: %v", err)
	}

	ss := store.NewSegmentStore(db)
	seg := &model.AcousticSegment{
		ID: "seg-dup-0-100", TrialID: "t-dup", ChannelIndex: 0, SampleRateHz: 2000,
		StartTimeMs: 100, DurationMs: 50, Samples: "[0.1]", Status: model.SegmentPendingCalibration,
	}
	if err := ss.Insert(seg); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	// 同一三元组再次 Insert：必须返回 ErrDuplicate，而非静默成功。
	if err := ss.Insert(seg); !errors.Is(err, model.ErrDuplicate) {
		t.Fatalf("second insert err = %v, want ErrDuplicate", err)
	}
	// 即便 ID 不同但三元组相同，也应判定为重复。
	seg2 := *seg
	seg2.ID = "seg-dup-other-id"
	if err := ss.Insert(&seg2); !errors.Is(err, model.ErrDuplicate) {
		t.Fatalf("third insert (diff id) err = %v, want ErrDuplicate", err)
	}
	list, err := ss.ListByTrial("t-dup")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 persisted row, got %d", len(list))
	}
}
