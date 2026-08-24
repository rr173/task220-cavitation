package service

import (
	"encoding/json"
	"fmt"
	"time"

	"task220-cavitation/internal/conclusion"
	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
	"task220-cavitation/internal/threshold"
)

// PackageService 结论包发布与阈值版本服务。
type PackageService struct {
	store     *store.PackageStore
	events    *store.EventStore
	trials    *store.TrialStore
	builder   *conclusion.Builder
	threshold *threshold.Manager
}

// NewPackageService 构造结论包服务。
func NewPackageService(ps *store.PackageStore, es *store.EventStore, ts *store.TrialStore, b *conclusion.Builder, tm *threshold.Manager) *PackageService {
	return &PackageService{store: ps, events: es, trials: ts, builder: b, threshold: tm}
}

// Publish 发布结论包并封存试验：
//  1. 试验须为 confirmed；
//  2. 聚合事件、计算置信度、生成摘要；
//  3. 旧已发布包置为 superseded，新包发布；
//  4. 试验封存（confirmed -> sealed）。
func (s *PackageService) Publish(trialID string) (*model.ConclusionPackage, error) {
	trial, err := s.trials.Get(trialID)
	if err != nil {
		return nil, err
	}
	if trial.Status != model.TrialConfirmed {
		if trial.Status == model.TrialSealed {
			return nil, model.ErrSealed
		}
		return nil, fmt.Errorf("%w: trial %s is %s, not confirmed", model.ErrInvalidState, trial.ID, trial.Status)
	}
	events, err := s.events.ListByTrial(trialID)
	if err != nil {
		return nil, err
	}
	conclusion.SortEvents(events)
	eventsJSON, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("marshal events: %w", err)
	}

	cfg := s.threshold.Current()
	pkg := s.builder.Build(trial, events, cfg.Version, string(eventsJSON))
	pkg.Version = s.nextVersion(trialID)
	now := time.Now().UTC()
	pkg.ID = fmt.Sprintf("pkg-%s-%d", trialID, pkg.Version)
	pkg.Status = model.PackagePublished
	pkg.CreatedAt = now
	pkg.PublishedAt = &now

	if err := s.store.InsertPackage(pkg); err != nil {
		return nil, err
	}
	// 旧已发布包 -> 替代。
	if err := s.supersedeOlder(trialID, pkg.Version); err != nil {
		return nil, err
	}
	// 封存试验。
	if _, err := s.trials.UpdateStatus(trialID, model.TrialConfirmed, model.TrialSealed); err != nil {
		return nil, err
	}
	return pkg, nil
}

// nextVersion 计算该试验下一个结论包版本号。
func (s *PackageService) nextVersion(trialID string) int {
	packages, err := s.store.ListPackagesByTrial(trialID)
	if err != nil {
		return 1
	}
	maxV := 0
	for _, p := range packages {
		if p.Version > maxV {
			maxV = p.Version
		}
	}
	return maxV + 1
}

// supersedeOlder 把同试验其它已发布包置为替代。
func (s *PackageService) supersedeOlder(trialID string, exceptVersion int) error {
	packages, err := s.store.ListPackagesByTrial(trialID)
	if err != nil {
		return err
	}
	for _, p := range packages {
		if p.Version == exceptVersion {
			continue
		}
		if p.Status == model.PackagePublished {
			if err := s.store.UpdateStatus(p.ID, model.PackageSuperseded, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListPackages 返回试验全部结论包。
func (s *PackageService) ListPackages(trialID string) ([]model.ConclusionPackage, error) {
	return s.store.ListPackagesByTrial(trialID)
}

// GetPackage 读取结论包。
func (s *PackageService) GetPackage(id string) (*model.ConclusionPackage, error) {
	return s.store.GetPackage(id)
}

// ListThresholds 返回全部阈值版本。
func (s *PackageService) ListThresholds() ([]model.ThresholdConfig, error) {
	return s.store.ListThresholds()
}

// AddThreshold 创建新阈值版本（持久化并同步内存管理器）。
func (s *PackageService) AddThreshold(gapRatio, energyFloor float64, confirmWindows int) (*model.ThresholdConfig, error) {
	next, err := s.threshold.NewVersion(gapRatio, energyFloor, confirmWindows)
	if err != nil {
		return nil, err
	}
	next.CreatedAt = time.Now().UTC()
	if err := s.store.InsertThreshold(next); err != nil {
		return nil, err
	}
	return next, nil
}
