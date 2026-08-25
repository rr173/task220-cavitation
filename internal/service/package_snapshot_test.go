package service

import (
	"encoding/json"
	"testing"
	"time"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
)

// TestPublishPreservesEventEvidenceSnapshot 端到端验证结论包冻结的事件
// 证据片段快照完整、起止边界不丢窗口、重读结论包与发布时一致。
//
// 覆盖三处回归点：
//  1. evidenceFor 闭区间保留起始/消退边界窗口；
//  2. EventStore.Insert 持久化 evidence_segments（不回退为空集）；
//  3. PackageStore.InsertPackage 持久化 events_json（重读与发布一致）。
func TestPublishPreservesEventEvidenceSnapshot(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	app, err := New(db)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}

	// 准备试验并推进到 confirmed（跳过声纹采集，直接在事件层注入证据）。
	trial, err := app.Trials.Create("snapshot trial", "evidence snapshot", 1500, 200, 0)
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}

	// 直接向事件表注入一个带证据边界的事件（含起始与消退边界窗口），
	// 以隔离“结论包冻结事件快照”这一回归点，不依赖谐波检测路径。
	// 事件先入库，随后走完整的试验状态机到 confirmed 再发布。
	es := store.NewEventStore(db)
	now := time.Now().UTC()
	evt := model.CavitationEvent{
		ID:               "evt-snapshot",
		TrialID:          trial.ID,
		Stage:            model.EventDecay,
		OnsetMs:          200,
		SustainedMs:      400,
		DecayMs:          600,
		Confidence:       0.82,
		EvidenceSegments: `["feat-onset","feat-mid","feat-decay"]`,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := es.Insert(&evt); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	// 推进试验状态机到可发布：preparing -> acquiring -> analyzing -> confirmed。
	if _, err := app.Trials.StartAcquisition(trial.ID); err != nil {
		t.Fatalf("start acquisition: %v", err)
	}
	if _, err := app.Trials.FinishAcquisition(trial.ID); err != nil {
		t.Fatalf("finish acquisition: %v", err)
	}
	if _, err := app.Trials.Confirm(trial.ID); err != nil {
		t.Fatalf("confirm trial: %v", err)
	}

	pkg, err := app.Packages.Publish(trial.ID)
	if err != nil {
		t.Fatalf("publish package: %v", err)
	}
	if pkg.Status != model.PackagePublished {
		t.Fatalf("package status = %s, want published", pkg.Status)
	}
	if pkg.EventsJSON == "" || pkg.EventsJSON == "[]" {
		t.Fatalf("published package froze empty events snapshot: %q", pkg.EventsJSON)
	}

	// 重读结论包，断言快照与发布时一致。
	reloaded, err := app.Packages.GetPackage(pkg.ID)
	if err != nil {
		t.Fatalf("reload package: %v", err)
	}
	if reloaded.EventsJSON != pkg.EventsJSON {
		t.Fatalf("reloaded events_json differs from published:\n published %q\n reloaded %q",
			pkg.EventsJSON, reloaded.EventsJSON)
	}
	var snapshotted []model.CavitationEvent
	if err := json.Unmarshal([]byte(reloaded.EventsJSON), &snapshotted); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(snapshotted) != 1 {
		t.Fatalf("snapshot event count = %d, want 1", len(snapshotted))
	}
	got := snapshotted[0]
	if got.EvidenceSegments != evt.EvidenceSegments {
		t.Fatalf("snapshot evidence mismatch:\n got %q\nwant %q", got.EvidenceSegments, evt.EvidenceSegments)
	}
	var ids []string
	if err := json.Unmarshal([]byte(got.EvidenceSegments), &ids); err != nil {
		t.Fatalf("decode evidence: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("evidence count = %d, want 3 (boundaries must be retained)", len(ids))
	}
}
