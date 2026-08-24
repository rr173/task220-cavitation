package service

import (
	"encoding/json"
	"fmt"
	"time"

	"task220-cavitation/internal/acq"
	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
)

// SegmentService 声纹片段接收、去重与通道校准服务。
type SegmentService struct {
	store      *store.SegmentStore
	features   *store.FeatureStore
	calibrator *acq.Calibrator
}

// NewSegmentService 构造片段服务。
func NewSegmentService(s *store.SegmentStore, f *store.FeatureStore, c *acq.Calibrator) *SegmentService {
	return &SegmentService{store: s, features: f, calibrator: c}
}

// IngestResult 片段接收结果统计。
type IngestResult struct {
	Inserted  int `json:"inserted"`
	Duplicate int `json:"duplicate"`
}

// Ingest 接收一段声纹：校验、幂等去重并入库。
// 约束：试验处于采集态；采样率与既有片段一致；同通道时间单调递增。
func (s *SegmentService) Ingest(trial *model.Trial, channel int, sampleRate float64, startMs int64, samples []float64) (IngestResult, error) {
	var res IngestResult
	if trial.Status != model.TrialAcquiring {
		return res, fmt.Errorf("%w: trial %s is %s, not acquiring", model.ErrInvalidState, trial.ID, trial.Status)
	}
	if len(samples) == 0 {
		return res, fmt.Errorf("%w: samples must not be empty", model.ErrInvalidInput)
	}
	seg := &model.AcousticSegment{
		ID:           fmt.Sprintf("seg-%s-%d-%d", trial.ID, channel, startMs),
		TrialID:      trial.ID,
		ChannelIndex: channel,
		SampleRateHz: sampleRate,
		StartTimeMs:  startMs,
		DurationMs:   int64(float64(len(samples)) / sampleRate * 1000),
	}
	if err := acq.Validate(seg); err != nil {
		return res, err
	}
	seg.Samples = mustJSON(samples)
	seg.PeakAmplitude, seg.RMS = acq.ComputePeakRMS(samples)
	seg.Fingerprint = acq.Fingerprint(trial.ID, channel, startMs)
	seg.Status = model.SegmentPendingCalibration
	seg.CreatedAt = time.Now().UTC()

	// 采样率一致性、幂等去重与时间单调性校验（基于既有片段）。
	existing, err := s.store.ListByTrial(trial.ID)
	if err != nil {
		return res, err
	}
	var channelMaxStart int64 = -1
	for _, e := range existing {
		if e.ChannelIndex == channel {
			if e.SampleRateHz != sampleRate {
				return res, fmt.Errorf("%w: sample rate %.0f mismatch channel baseline %.0f",
					model.ErrInvalidInput, sampleRate, e.SampleRateHz)
			}
			if e.StartTimeMs == startMs {
				// 精确重复：幂等跳过，不视为时间倒退。
				res.Inserted++
				return res, nil
			}
			if e.StartTimeMs > channelMaxStart {
				channelMaxStart = e.StartTimeMs
			}
		}
	}
	if channelMaxStart >= 0 && startMs < channelMaxStart {
		return res, fmt.Errorf("%w: time going backwards on channel %d (last %d, got %d)",
			model.ErrInvalidInput, channel, channelMaxStart, startMs)
	}

	if err := s.store.Insert(seg); err != nil {
		if err == model.ErrDuplicate {
			res.Duplicate++
			return res, nil
		}
		return res, err
	}
	res.Inserted++
	return res, nil
}

// MarkNoisy 把片段标记为机械噪声（不删除，仅降权）。
func (s *SegmentService) MarkNoisy(segmentID string) error {
	seg, err := s.store.Get(segmentID)
	if err != nil {
		return err
	}
	if seg.Status == model.SegmentDuplicate {
		return fmt.Errorf("%w: duplicate segment cannot be marked noisy", model.ErrInvalidState)
	}
	return s.store.UpdateStatus(segmentID, model.SegmentNoisy)
}

// CalibrateChannels 以参考通道为基准，对其它通道做互相关延迟校准。
// 校准结果写入 channel_delays，返回校准结果列表。
func (s *SegmentService) CalibrateChannels(trial *model.Trial) ([]model.ChannelDelay, error) {
	segments, err := s.store.ListByTrial(trial.ID)
	if err != nil {
		return nil, err
	}
	// 按通道聚合抽样（时间升序拼接）。
	byChannel := map[int][]float64{}
	var sampleRate float64
	for _, seg := range segments {
		if seg.Status == model.SegmentDuplicate || seg.Status == model.SegmentMissing {
			continue
		}
		var samples []float64
		if err := json.Unmarshal([]byte(seg.Samples), &samples); err != nil {
			return nil, fmt.Errorf("decode samples: %w", err)
		}
		byChannel[seg.ChannelIndex] = append(byChannel[seg.ChannelIndex], samples...)
		if sampleRate == 0 {
			sampleRate = seg.SampleRateHz
		}
	}
	if sampleRate == 0 {
		return nil, model.ErrInsufficientData
	}
	ref := byChannel[trial.ReferenceChannel]
	if len(ref) == 0 {
		return nil, fmt.Errorf("%w: reference channel %d has no samples", model.ErrInsufficientData, trial.ReferenceChannel)
	}

	var out []model.ChannelDelay
	for ch, signal := range byChannel {
		if ch == trial.ReferenceChannel {
			// 参考通道自身延迟为 0。
			d := model.ChannelDelay{
				TrialID: trial.ID, ChannelIndex: ch, DelayMs: 0, CorrelationScore: 1.0,
				Status: model.DelayLocked, CreatedAt: time.Now().UTC(),
			}
			if err := s.features.UpsertChannelDelay(&d); err != nil {
				return nil, err
			}
			out = append(out, d)
			continue
		}
		d, err := s.calibrator.Calibrate(trial.ID, ch, ref, signal, sampleRate)
		if err != nil {
			return nil, err
		}
		d.CreatedAt = time.Now().UTC()
		if err := s.features.UpsertChannelDelay(d); err != nil {
			return nil, err
		}
		out = append(out, *d)
	}

	// 校准完成后，把待校准片段置为有效（后续可参与特征提取）。
	for _, seg := range segments {
		if seg.Status == model.SegmentPendingCalibration {
			if err := s.store.UpdateStatus(seg.ID, model.SegmentValid); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// ListSegments 返回试验全部片段。
func (s *SegmentService) ListSegments(trialID string) ([]model.AcousticSegment, error) {
	return s.store.ListByTrial(trialID)
}

// ListDelays 返回试验全部通道延迟校准结果。
func (s *SegmentService) ListDelays(trialID string) ([]model.ChannelDelay, error) {
	return s.features.ListChannelDelays(trialID)
}

// CountNoisyChannels 返回被标记噪声的片段所属通道集合大小（置信度降权用）。
func (s *SegmentService) CountNoisyChannels(trialID string) (int, error) {
	segments, err := s.store.ListByTrial(trialID)
	if err != nil {
		return 0, err
	}
	noisy := map[int]bool{}
	for _, seg := range segments {
		if seg.Status == model.SegmentNoisy {
			noisy[seg.ChannelIndex] = true
		}
	}
	return len(noisy), nil
}

func mustJSON(v []float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
