// Package threshold 阈值模块：空化判定的阈值配置版本管理。
// 阈值（缺口比、能量下限、确认窗口数）决定空化起始/消退的判定边界，
// 版本化保证已发布结论可回溯到当时使用的阈值。
package threshold

import (
	"fmt"

	"task220-cavitation/internal/model"
)

// Validate 校验阈值配置合法性：缺口比阈值 > 0，能量下限 >= 0，确认窗口 >= 1。
func Validate(cfg *model.ThresholdConfig) error {
	if cfg.GapRatioThreshold <= 0 {
		return fmt.Errorf("%w: gap ratio threshold must be positive", model.ErrInvalidInput)
	}
	if cfg.EnergyFloor < 0 {
		return fmt.Errorf("%w: energy floor must be non-negative", model.ErrInvalidInput)
	}
	if cfg.ConfirmWindows < 1 {
		return fmt.Errorf("%w: confirm windows must be at least 1", model.ErrInvalidInput)
	}
	return nil
}

// Manager 阈值管理器：维护当前生效阈值版本并派生新版本。
type Manager struct {
	current *model.ThresholdConfig
}

// NewManager 以初始阈值构造管理器。
func NewManager(current *model.ThresholdConfig) (*Manager, error) {
	if err := Validate(current); err != nil {
		return nil, err
	}
	return &Manager{current: current}, nil
}

// Current 返回当前生效阈值。
func (m *Manager) Current() *model.ThresholdConfig { return m.current }

// NewVersion 基于当前阈值派生新版本（版本号 +1）。
func (m *Manager) NewVersion(gapRatio, energyFloor float64, confirmWindows int) (*model.ThresholdConfig, error) {
	next := &model.ThresholdConfig{
		Version:           m.current.Version,
		GapRatioThreshold: gapRatio,
		EnergyFloor:       energyFloor,
		ConfirmWindows:    confirmWindows,
	}
	if err := Validate(next); err != nil {
		return nil, err
	}
	m.current = next
	return next, nil
}
