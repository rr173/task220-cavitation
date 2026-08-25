package threshold

import (
	"errors"
	"sync"
	"testing"

	"task220-cavitation/internal/model"
)

// TestNewVersionStrictlyIncreasing 断言连续两次新增得到不同且递增的版本号，
// 且默认版本不被覆盖（版本号从初始值之上 +1 起步）。
func TestNewVersionStrictlyIncreasing(t *testing.T) {
	m, err := NewManager(&model.ThresholdConfig{
		Version: 1, GapRatioThreshold: 0.15, EnergyFloor: 0.001, ConfirmWindows: 3,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	v1, err := m.NewVersion(0.2, 0.002, 3)
	if err != nil {
		t.Fatalf("first NewVersion: %v", err)
	}
	if v1.Version != 2 {
		t.Fatalf("first new version = %d, want 2 (must exceed default 1)", v1.Version)
	}

	v2, err := m.NewVersion(0.25, 0.002, 4)
	if err != nil {
		t.Fatalf("second NewVersion: %v", err)
	}
	if v2.Version != 3 {
		t.Fatalf("second new version = %d, want 3", v2.Version)
	}
	if v2.Version == v1.Version {
		t.Fatalf("two consecutive new versions must not be equal, both = %d", v1.Version)
	}
	if v2.Version <= v1.Version {
		t.Fatalf("version must strictly increase: v1=%d v2=%d", v1.Version, v2.Version)
	}

	// Current 随版本推进而更新。
	cur := m.Current()
	if cur.Version != 3 {
		t.Fatalf("current version = %d, want 3", cur.Version)
	}
	// Current 返回快照拷贝，修改不影响管理器内部状态。
	cur.Version = 999
	if got := m.Current().Version; got != 3 {
		t.Fatalf("Current snapshot leaked: got %d, want 3", got)
	}
}

// TestNewVersionValidationFailureDoesNotAdvance 校验失败时不推进当前版本号。
func TestNewVersionValidationFailureDoesNotAdvance(t *testing.T) {
	m, err := NewManager(&model.ThresholdConfig{
		Version: 1, GapRatioThreshold: 0.15, EnergyFloor: 0.001, ConfirmWindows: 3,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	// 非法缺口比阈值。
	if _, err := m.NewVersion(0, 0.002, 3); !errors.Is(err, model.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	// 当前版本未被推进，下一次合法新增应得到 2。
	v, err := m.NewVersion(0.2, 0.002, 3)
	if err != nil {
		t.Fatalf("NewVersion after validation failure: %v", err)
	}
	if v.Version != 2 {
		t.Fatalf("version after failed attempt = %d, want 2", v.Version)
	}
}

// TestNewVersionConcurrentAllDistinct 验证并发新增时不会有两个请求
// 派生出相同的版本号（线程安全的严格递增）。
func TestNewVersionConcurrentAllDistinct(t *testing.T) {
	m, err := NewManager(&model.ThresholdConfig{
		Version: 1, GapRatioThreshold: 0.15, EnergyFloor: 0.001, ConfirmWindows: 3,
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	const n = 64
	var wg sync.WaitGroup
	versions := make([]int, n)
	errs := make([]error, n)
	start := make(chan struct{})
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			cfg, err := m.NewVersion(0.1+float64(i%5)*0.01, 0.002, 3+(i%3))
			versions[i] = -1
			if err != nil {
				errs[i] = err
				return
			}
			versions[i] = cfg.Version
		}(i)
	}
	close(start)
	wg.Wait()

	seen := make(map[int]bool, n+1)
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d error: %v", i, errs[i])
		}
		if versions[i] <= 1 {
			t.Fatalf("goroutine %d produced non-increasing version %d", i, versions[i])
		}
		if seen[versions[i]] {
			t.Fatalf("duplicate version %d produced by concurrent NewVersion", versions[i])
		}
		seen[versions[i]] = true
	}
	// 当前版本应为初始 1 + n 次新增。
	if got := m.Current().Version; got != 1+n {
		t.Fatalf("final current version = %d, want %d", got, 1+n)
	}
}
