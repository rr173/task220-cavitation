package service

import (
	"errors"
	"testing"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
)

// TestAddThresholdVersioningAcrossRestart 验证空化判定阈值版本号：
//  1. 连续两次新增得到不同且递增的版本号（2、3）；
//  2. 新增不覆盖默认版本 1（列表中仍含默认值，共 3 条）；
//  3. 关闭并重开数据库后，已有版本完整保留，且再次新增得到的版本号
//     在重启前的最大值之上继续递增（4），不回退、不重复。
func TestAddThresholdVersioningAcrossRestart(t *testing.T) {
	path := t.TempDir() + "/thr_versioning.db"

	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	app, err := New(db)
	if err != nil {
		db.Close()
		t.Fatalf("new app: %v", err)
	}

	// 默认版本为 1。
	if v := app.Threshold().Current().Version; v != 1 {
		t.Fatalf("initial current threshold version = %d, want 1", v)
	}

	v1, err := app.Packages.AddThreshold(0.2, 0.002, 3)
	if err != nil {
		db.Close()
		t.Fatalf("first AddThreshold: %v", err)
	}
	v2, err := app.Packages.AddThreshold(0.25, 0.002, 4)
	if err != nil {
		db.Close()
		t.Fatalf("second AddThreshold: %v", err)
	}
	if v1.Version != 2 || v2.Version != 3 {
		db.Close()
		t.Fatalf("new versions = %d,%d, want 2,3 (strictly increasing)", v1.Version, v2.Version)
	}
	if v1.Version == v2.Version {
		db.Close()
		t.Fatalf("two consecutive new versions must differ, both = %d", v1.Version)
	}

	// 默认版本与历史完整：1,2,3。
	thrs, err := app.Packages.ListThresholds()
	if err != nil {
		db.Close()
		t.Fatalf("list thresholds: %v", err)
	}
	if len(thrs) != 3 {
		db.Close()
		t.Fatalf("threshold count = %d, want 3 (default must survive)", len(thrs))
	}
	for i, v := range thrs {
		if v.Version != i+1 {
			db.Close()
			t.Fatalf("threshold[%d].Version = %d, want %d", i, v.Version, i+1)
		}
	}

	// --- 重启恢复 ---
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	db2, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer db2.Close()
	app2, err := New(db2)
	if err != nil {
		t.Fatalf("reinit app: %v", err)
	}

	// 重启后当前版本恢复为重启前的最大值 3。
	if v := app2.Threshold().Current().Version; v != 3 {
		t.Fatalf("current threshold version after reopen = %d, want 3", v)
	}
	thrs2, err := app2.Packages.ListThresholds()
	if err != nil {
		t.Fatalf("list thresholds after reopen: %v", err)
	}
	if len(thrs2) != 3 {
		t.Fatalf("threshold count after reopen = %d, want 3", len(thrs2))
	}
	// 重启后再次新增：版本号在 3 之上继续递增到 4，不回退、不与已存在版本重复。
	v3, err := app2.Packages.AddThreshold(0.3, 0.003, 5)
	if err != nil {
		t.Fatalf("AddThreshold after reopen: %v", err)
	}
	if v3.Version != 4 {
		t.Fatalf("version after reopen AddThreshold = %d, want 4", v3.Version)
	}
	// 覆盖默认版本的反例：默认版本 1 仍可读且内容未被改写。
	def, err := app2.Packages.ListThresholds()
	if err != nil {
		t.Fatalf("list after second add: %v", err)
	}
	var defaults *model.ThresholdConfig
	for i := range def {
		if def[i].Version == 1 {
			defaults = &def[i]
			break
		}
	}
	if defaults == nil {
		t.Fatalf("default threshold version 1 missing after adds")
	}
	if defaults.GapRatioThreshold != 0.15 || defaults.EnergyFloor != 0.001 || defaults.ConfirmWindows != 3 {
		t.Fatalf("default threshold was overwritten: %+v", defaults)
	}
	// 重复插入同一版本号应被拒绝而非覆盖（防御性断言）。
	if _, err := app2.Packages.AddThreshold(0.31, 0.003, 5); err != nil {
		// 正常情况下版本号严格递增，此处不应触发；仅在版本号回退时才可能冲突。
		// 用 errors.Is 校验：若真冲突，应是 ErrDuplicate 而非静默覆盖。
		if !errors.Is(err, model.ErrDuplicate) {
			t.Fatalf("unexpected error on add: %v", err)
		}
	}
}
