// Package service 编排层：组装各业务模块并暴露给 httpapi 与 main。
package service

import (
	"sync"

	"task220-cavitation/internal/acq"
	"task220-cavitation/internal/cavitation"
	"task220-cavitation/internal/conclusion"
	"task220-cavitation/internal/harmonic"
	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
	"task220-cavitation/internal/threshold"
)

// App 应用编排根：聚合全部模块服务。
type App struct {
	db *store.DB

	Trials    *TrialService
	Segments  *SegmentService
	Events    *EventService
	Packages  *PackageService

	stats     *store.StatsStore
	threshold *threshold.Manager

	mu sync.Mutex // 试验状态流转与结论发布串行
}

// New 组装应用服务。
func New(db *store.DB) (*App, error) {
	trialStore := store.NewTrialStore(db)
	segmentStore := store.NewSegmentStore(db)
	featureStore := store.NewFeatureStore(db)
	eventStore := store.NewEventStore(db)
	packageStore := store.NewPackageStore(db)
	stats := store.NewStatsStore(db)

	latestThreshold, err := packageStore.LatestThreshold()
	if err != nil {
		return nil, err
	}
	thrMgr, err := threshold.NewManager(latestThreshold)
	if err != nil {
		return nil, err
	}

	app := &App{
		db:        db,
		stats:     stats,
		threshold: thrMgr,
	}
	app.Trials = NewTrialService(trialStore)
	app.Segments = NewSegmentService(segmentStore, featureStore, acq.NewCalibrator())
	app.Events = NewEventService(eventStore, featureStore, segmentStore, harmonic.NewGapAnalyzer(), cavitation.NewClassifier())
	app.Packages = NewPackageService(packageStore, eventStore, trialStore, conclusion.NewBuilder(), thrMgr)
	return app, nil
}

// DB 暴露底层连接（自检用）。
func (a *App) DB() *store.DB { return a.db }

// Stats 返回全局统计快照。
func (a *App) Stats() (*model.StatSummary, error) { return a.stats.Global() }

// Threshold 返回当前阈值管理器。
func (a *App) Threshold() *threshold.Manager { return a.threshold }
