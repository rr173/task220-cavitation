package threshold

import "task220-cavitation/internal/model"

// History 阈值版本历史：维护已创建的全部版本，供结论回溯与审计。
type History struct {
	versions []model.ThresholdConfig
}

// NewHistory 以初始版本构造历史。
func NewHistory(initial model.ThresholdConfig) *History {
	return &History{versions: []model.ThresholdConfig{initial}}
}

// Add 追加新版本（去重：相同版本号不重复加入）。
func (h *History) Add(cfg model.ThresholdConfig) error {
	if err := Validate(&cfg); err != nil {
		return err
	}
	for _, v := range h.versions {
		if v.Version == cfg.Version {
			return nil
		}
	}
	h.versions = append(h.versions, cfg)
	return nil
}

// Get 按版本号取阈值，不存在返回 ErrNotFound。
func (h *History) Get(version int) (*model.ThresholdConfig, error) {
	for i := range h.versions {
		if h.versions[i].Version == version {
			v := h.versions[i]
			return &v, nil
		}
	}
	return nil, model.ErrNotFound
}

// Latest 返回最新版本。
func (h *History) Latest() *model.ThresholdConfig {
	if len(h.versions) == 0 {
		return nil
	}
	v := h.versions[len(h.versions)-1]
	return &v
}

// All 返回全部版本（按版本号升序）。
func (h *History) All() []model.ThresholdConfig {
	out := make([]model.ThresholdConfig, len(h.versions))
	copy(out, h.versions)
	return out
}
