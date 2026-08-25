package service

import (
	"encoding/json"
	"fmt"
	"time"

	"task220-cavitation/internal/cavitation"
	"task220-cavitation/internal/harmonic"
	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
)

// EventService 空化事件服务：特征提取、事件检测与人工否决。
type EventService struct {
	store     *store.EventStore
	features  *store.FeatureStore
	segments  *store.SegmentStore
	analyzer  *harmonic.GapAnalyzer
	classifier *cavitation.Classifier
}

// NewEventService 构造事件服务。
func NewEventService(es *store.EventStore, fs *store.FeatureStore, ss *store.SegmentStore, ga *harmonic.GapAnalyzer, cl *cavitation.Classifier) *EventService {
	return &EventService{store: es, features: fs, segments: ss, analyzer: ga, classifier: cl}
}

// AnalyzeResult 分析结果统计。
type AnalyzeResult struct {
	FeatureCount int `json:"feature_count"`
	EventCount   int `json:"event_count"`
}

// Analyze 对试验执行特征提取与空化事件检测。
// 试验须处于 analyzing 态；对全部 valid 片段逐窗提取谐波特征后检测事件。
func (s *EventService) Analyze(trial *model.Trial, cfg *model.ThresholdConfig, noisyChannels int) (AnalyzeResult, error) {
	var res AnalyzeResult
	if trial.Status != model.TrialAnalyzing {
		return res, fmt.Errorf("%w: trial %s is %s, not analyzing", model.ErrInvalidState, trial.ID, trial.Status)
	}
	// 阈值配置缺失时不得进入检测链路：JudgeWindows 会解引用 cfg.GapRatioThreshold
	// 等字段，nil 配置将触发 panic。提前以非法输入终止，避免特征提取副作用。
	if cfg == nil {
		return res, fmt.Errorf("%w: threshold config is missing", model.ErrInvalidInput)
	}
	segments, err := s.segments.ListValidByTrial(trial.ID)
	if err != nil {
		return res, err
	}
	if len(segments) == 0 {
		return res, fmt.Errorf("%w: no valid segments to analyze", model.ErrInsufficientData)
	}

	var windows []model.HarmonicFeatures
	for i, seg := range segments {
		var samples []float64
		if err := json.Unmarshal([]byte(seg.Samples), &samples); err != nil {
			return res, fmt.Errorf("decode samples: %w", err)
		}
		f, err := harmonic.ComputeFeatures(samples, seg.SampleRateHz)
		if err != nil {
			// 单窗特征失败不阻断整体分析，记录并跳过。
			continue
		}
		f.ID = fmt.Sprintf("feat-%s-%d", trial.ID, i)
		f.TrialID = trial.ID
		f.WindowStartMs = seg.StartTimeMs
		f.WindowEndMs = seg.StartTimeMs + seg.DurationMs
		f.CreatedAt = time.Now().UTC()
		if err := s.features.InsertFeature(f); err != nil {
			if err == model.ErrDuplicate {
				continue
			}
			return res, err
		}
		windows = append(windows, *f)
		res.FeatureCount++
	}

	events, err := s.classifier.DetectEvents(windows, cfg, noisyChannels)
	if err != nil {
		return res, err
	}
	for _, ev := range events {
		now := time.Now().UTC()
		ev.ID = fmt.Sprintf("evt-%s-%d", trial.ID, now.UnixNano())
		ev.TrialID = trial.ID
		ev.EvidenceSegments = evidenceFor(windows, ev)
		ev.CreatedAt = now
		ev.UpdatedAt = now
		if err := s.store.Insert(&ev); err != nil {
			return res, err
		}
		res.EventCount++
	}
	return res, nil
}

// evidenceFor 生成事件的证据片段引用（窗口时间落在事件区间内的窗口）。
func evidenceFor(windows []model.HarmonicFeatures, ev model.CavitationEvent) string {
	var ids []string
	for _, w := range windows {
		if w.WindowStartMs >= ev.OnsetMs && (ev.DecayMs == 0 || w.WindowStartMs <= ev.DecayMs) {
			ids = append(ids, w.ID)
		}
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

// ListEvents 返回试验全部事件。
func (s *EventService) ListEvents(trialID string) ([]model.CavitationEvent, error) {
	return s.store.ListByTrial(trialID)
}

// GetEvent 读取单个事件。
func (s *EventService) GetEvent(id string) (*model.CavitationEvent, error) {
	return s.store.Get(id)
}

// Reject 否决一个事件（标注原因，状态 -> rejected）。
func (s *EventService) Reject(trial *model.Trial, eventID, reason string) error {
	if trial.Status == model.TrialSealed {
		return model.ErrSealed
	}
	ev, err := s.store.Get(eventID)
	if err != nil {
		return err
	}
	if ev.TrialID != trial.ID {
		return fmt.Errorf("%w: event %s not in trial %s", model.ErrInvalidInput, eventID, trial.ID)
	}
	if ev.Stage == model.EventRejected {
		return fmt.Errorf("%w: event already rejected", model.ErrInvalidState)
	}
	ev.Stage = model.EventRejected
	ev.RejectReason = reason
	ev.UpdatedAt = time.Now().UTC()
	return s.store.Update(ev)
}

// Advance 手动推进事件阶段（如候选 -> 起始），非法流转返回错误。
func (s *EventService) Advance(trial *model.Trial, eventID, toStage string) error {
	if trial.Status == model.TrialSealed {
		return model.ErrSealed
	}
	ev, err := s.store.Get(eventID)
	if err != nil {
		return err
	}
	if ev.TrialID != trial.ID {
		return fmt.Errorf("%w: event %s not in trial %s", model.ErrInvalidInput, eventID, trial.ID)
	}
	if err := cavitation.Transition(ev, toStage); err != nil {
		return err
	}
	ev.UpdatedAt = time.Now().UTC()
	return s.store.Update(ev)
}

// ListFeatures 返回试验全部谐波特征。
func (s *EventService) ListFeatures(trialID string) ([]model.HarmonicFeatures, error) {
	return s.features.ListFeaturesByTrial(trialID)
}
