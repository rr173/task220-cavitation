package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
)

// newSealedTrialWithEvents 建立一个已 confirmed 的试验，并按给定事件列表原样写入
// 数据库（保留调用方给出的写入顺序），供发布顺序测试使用。
func newSealedTrialWithEvents(t *testing.T, db *store.DB, events []model.CavitationEvent) *model.Trial {
	t.Helper()
	app, err := New(db)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	trial, err := app.Trials.Create("publish-order", "snapshot ordering", 1500, 200, 0)
	if err != nil {
		t.Fatalf("create trial: %v", err)
	}
	for _, step := range []func(string) (*model.Trial, error){
		app.Trials.StartAcquisition,
		app.Trials.FinishAcquisition,
		app.Trials.Confirm,
	} {
		if _, err := step(trial.ID); err != nil {
			t.Fatalf("lifecycle step: %v", err)
		}
	}
	es := store.NewEventStore(db)
	now := time.Now().UTC()
	for i := range events {
		events[i].ID = "evt-order-" + trial.ID + "-" + itoa(i)
		events[i].TrialID = trial.ID
		if events[i].CreatedAt.IsZero() {
			events[i].CreatedAt = now
		}
		events[i].UpdatedAt = now
		if err := es.Insert(&events[i]); err != nil {
			t.Fatalf("insert event %d: %v", i, err)
		}
	}
	tr, err := app.Trials.Get(trial.ID)
	if err != nil {
		t.Fatalf("get trial: %v", err)
	}
	return tr
}

func itoa(i int) string {
	// 不依赖 strconv 以减少导入；试验规模小，手写即可。
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

// TestPublishPackageFreezesEventsEarliestFirst 验证：发布结论包时事件证据按起始
// 时间从早到晚冻结在快照中，即便事件写入数据库的顺序与之相反也不倒序、不丢失。
func TestPublishPackageFreezesEventsEarliestFirst(t *testing.T) {
	// 按“从晚到早”的倒序写入数据库，模拟写入顺序与时间顺序不一致。
	reverse := []model.CavitationEvent{
		{Stage: model.EventInception, OnsetMs: 9000, Confidence: 0.7},
		{Stage: model.EventInception, OnsetMs: 5000, Confidence: 0.6},
		{Stage: model.EventInception, OnsetMs: 1000, Confidence: 0.5},
	}
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	trial := newSealedTrialWithEvents(t, db, reverse)
	app, err := New(db)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	pkg, err := app.Packages.Publish(trial.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if pkg.Status != model.PackagePublished {
		t.Fatalf("status = %s, want published", pkg.Status)
	}

	var got []model.CavitationEvent
	if err := json.Unmarshal([]byte(pkg.EventsJSON), &got); err != nil {
		t.Fatalf("unmarshal events snapshot: %v", err)
	}
	if len(got) != len(reverse) {
		t.Fatalf("event count = %d, want %d (events must not be lost)", len(got), len(reverse))
	}
	wantOnsets := []int64{1000, 5000, 9000}
	for i, e := range got {
		if e.OnsetMs != wantOnsets[i] {
			t.Fatalf("snapshot event %d onset = %d, want %d (must be earliest-first)",
				i, e.OnsetMs, wantOnsets[i])
		}
	}
}

// TestPublishPackageEmptyEventsSnapshot 验证无事件时快照为空数组而非丢失。
func TestPublishPackageEmptyEventsSnapshot(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	trial := newSealedTrialWithEvents(t, db, nil)

	app, err := New(db)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	pkg, err := app.Packages.Publish(trial.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if pkg.EventsJSON != "[]" && pkg.EventsJSON != "null" {
		t.Fatalf("empty snapshot = %q, want [] or null", pkg.EventsJSON)
	}
	if pkg.Confidence != 0 {
		t.Fatalf("confidence = %v, want 0 for no events", pkg.Confidence)
	}
}

// TestSealedTrialRejectsPublish 验证已封存试验不可再次发布（幂等护栏）。
func TestSealedTrialRejectsPublish(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	trial := newSealedTrialWithEvents(t, db, []model.CavitationEvent{
		{Stage: model.EventInception, OnsetMs: 1000, Confidence: 0.5},
	})
	app, err := New(db)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	if _, err := app.Packages.Publish(trial.ID); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	if _, err := app.Packages.Publish(trial.ID); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("second publish error = %v, want ErrSealed", err)
	}
}
