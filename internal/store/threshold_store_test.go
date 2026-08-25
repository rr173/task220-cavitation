package store

import (
	"errors"
	"testing"
	"time"

	"task220-cavitation/internal/model"
)

// TestInsertThresholdDoesNotOverwriteDefault 验证插入默认版本号（1）冲突时
// 返回 ErrDuplicate 而非覆盖默认阈值：默认版本必须保留完整。
func TestInsertThresholdDoesNotOverwriteDefault(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := NewPackageStore(db)

	// 默认版本 1 由迁移种子植入；尝试以相同版本号再插一条应被拒绝。
	dup := &model.ThresholdConfig{
		Version: 1, GapRatioThreshold: 0.99, EnergyFloor: 0.5, ConfirmWindows: 9,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.InsertThreshold(dup); !errors.Is(err, model.ErrDuplicate) {
		t.Fatalf("insert duplicate version 1: want ErrDuplicate, got %v", err)
	}

	// 默认版本内容未被覆盖。
	got, err := s.GetThreshold(1)
	if err != nil {
		t.Fatalf("get default threshold: %v", err)
	}
	if got.GapRatioThreshold != 0.15 || got.EnergyFloor != 0.001 || got.ConfirmWindows != 3 {
		t.Fatalf("default threshold was overwritten: %+v", got)
	}
}

// TestInsertThresholdPersistsAcrossReopen 验证新增阈值版本号在关闭并重开
// 数据库后仍完整：版本号持续递增且历史全部可读。
func TestInsertThresholdPersistsAcrossReopen(t *testing.T) {
	path := t.TempDir() + "/thr_reopen.db"
	if err := openAndSeedTwoVersions(path); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// 重开数据库，验证历史完整且最大版本恢复正确。
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer db2.Close()
	s2 := NewPackageStore(db2)

	all, err := s2.ListThresholds()
	if err != nil {
		t.Fatalf("list thresholds after reopen: %v", err)
	}
	if len(all) != 3 { // 默认 1 + 新增 2、3
		t.Fatalf("threshold count after reopen = %d, want 3 (default must survive): %+v", len(all), all)
	}
	// 版本号升序且连续递增：1,2,3。
	for i, v := range all {
		if v.Version != i+1 {
			t.Fatalf("threshold[%d].Version = %d, want %d (must be contiguous & increasing)", i, v.Version, i+1)
		}
	}
	latest, err := s2.LatestThreshold()
	if err != nil {
		t.Fatalf("latest threshold after reopen: %v", err)
	}
	if latest.Version != 3 {
		t.Fatalf("latest threshold version after reopen = %d, want 3", latest.Version)
	}
}

func openAndSeedTwoVersions(path string) error {
	db, err := Open(path)
	if err != nil {
		return err
	}
	defer db.Close()
	s := NewPackageStore(db)
	for _, c := range []model.ThresholdConfig{
		{Version: 2, GapRatioThreshold: 0.2, EnergyFloor: 0.002, ConfirmWindows: 3, CreatedAt: time.Now().UTC()},
		{Version: 3, GapRatioThreshold: 0.25, EnergyFloor: 0.002, ConfirmWindows: 4, CreatedAt: time.Now().UTC()},
	} {
		if err := s.InsertThreshold(&c); err != nil {
			return err
		}
	}
	return nil
}
