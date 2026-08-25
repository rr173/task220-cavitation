package store

import (
	"encoding/json"
	"testing"
	"time"

	"task220-cavitation/internal/model"
)

// TestEventStorePersistsEvidence 验证事件落库时保留完整证据片段快照，
// 重读后内容与写入时一致（不回退为空集）。
func TestEventStorePersistsEvidence(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := NewEventStore(db)
	now := time.Now().UTC()
	encoded, _ := json.Marshal([]string{"seg-a", "seg-b", "seg-c"})
	want := &model.CavitationEvent{
		ID:               "evt-persist-evidence",
		TrialID:          "trial-x",
		Stage:            model.EventDecay,
		OnsetMs:          200,
		SustainedMs:      400,
		DecayMs:          600,
		Confidence:       0.82,
		EvidenceSegments: string(encoded),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.Insert(want); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	got, err := s.Get(want.ID)
	if err != nil {
		t.Fatalf("get event: %v", err)
	}
	if got.EvidenceSegments != want.EvidenceSegments {
		t.Fatalf("evidence round trip mismatch:\n got %q\nwant %q", got.EvidenceSegments, want.EvidenceSegments)
	}
	var ids []string
	if err := json.Unmarshal([]byte(got.EvidenceSegments), &ids); err != nil {
		t.Fatalf("decode evidence: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("evidence count = %d, want 3", len(ids))
	}
}

// TestPackageStorePersistsEventsJSON 验证发布结论包冻结的事件快照完整落库，
// 重读后内容与发布时一致（不回退为空集），满足审计可回溯。
func TestPackageStorePersistsEventsJSON(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// 先建试验，满足外键约束。
	ts := NewTrialStore(db)
	now := time.Now().UTC()
	if err := ts.Create(&model.Trial{
		ID: "trial-pkg", Name: "pkg snapshot", Description: "",
		ShaftSpeedRPM: 1500, InflowPressureKPa: 200, ReferenceChannel: 0,
		Status: model.TrialSealed, Fingerprint: "fp-pkg",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create trial: %v", err)
	}

	events := []model.CavitationEvent{
		{ID: "evt-1", TrialID: "trial-pkg", Stage: model.EventDecay,
			OnsetMs: 200, SustainedMs: 400, DecayMs: 600, Confidence: 0.82,
			EvidenceSegments: `["seg-a","seg-b","seg-c"]`, CreatedAt: now, UpdatedAt: now},
		{ID: "evt-2", TrialID: "trial-pkg", Stage: model.EventRejected, RejectReason: "机械噪声",
			OnsetMs: 100, Confidence: 0.1, CreatedAt: now, UpdatedAt: now},
	}
	eventsJSON, _ := json.Marshal(events)

	pkg := &model.ConclusionPackage{
		ID:               "pkg-snapshot-1",
		TrialID:          "trial-pkg",
		Version:          1,
		Status:           model.PackagePublished,
		ThresholdVersion: 1,
		EventsJSON:       string(eventsJSON),
		Summary:          "试验 trial-pkg：识别 1 个空化事件",
		Confidence:       0.82,
		CreatedAt:        now,
		PublishedAt:      &now,
	}
	ps := NewPackageStore(db)
	if err := ps.InsertPackage(pkg); err != nil {
		t.Fatalf("insert package: %v", err)
	}

	got, err := ps.GetPackage(pkg.ID)
	if err != nil {
		t.Fatalf("get package: %v", err)
	}
	if got.EventsJSON != pkg.EventsJSON {
		t.Fatalf("events_json round trip mismatch:\n got %q\nwant %q", got.EventsJSON, pkg.EventsJSON)
	}
	var gotEvents []model.CavitationEvent
	if err := json.Unmarshal([]byte(got.EventsJSON), &gotEvents); err != nil {
		t.Fatalf("decode events_json: %v", err)
	}
	if len(gotEvents) != 2 {
		t.Fatalf("event count = %d, want 2 (snapshot must be complete)", len(gotEvents))
	}
	if gotEvents[0].EvidenceSegments != `["seg-a","seg-b","seg-c"]` {
		t.Fatalf("snapshot event evidence lost: %q", gotEvents[0].EvidenceSegments)
	}
}
