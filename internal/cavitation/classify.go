package cavitation

import (
	"math"

	"task220-cavitation/internal/model"
)

// Classifier 事件分类器：在判定器之上补充置信度评估与多事件切分。
type Classifier struct {
	detector *Detector
}

// NewClassifier 构造分类器。
func NewClassifier() *Classifier {
	return &Classifier{detector: NewDetector()}
}

// DetectEvents 从特征窗口序列中识别空化事件（含置信度）。
// noisyChannels 为该试验被标记为机械噪声的通道数，用于置信度降权。
func (c *Classifier) DetectEvents(features []model.HarmonicFeatures, cfg *model.ThresholdConfig, noisyChannels int) ([]model.CavitationEvent, error) {
	if len(features) == 0 {
		return nil, model.ErrInsufficientData
	}
	onset, sustained, decay, found := c.detector.Detect(features, cfg)
	if !found {
		return nil, nil // 无空化事件，返回空集而非错误
	}

	ev := model.CavitationEvent{
		Stage:       model.EventCandidate,
		OnsetMs:     features[onset].WindowStartMs,
		SustainedMs: features[sustained].WindowStartMs,
		Confidence:  c.confidence(features, cfg, noisyChannels),
	}
	if decay >= 0 {
		ev.DecayMs = features[decay].WindowStartMs
		ev.Stage = model.EventDecay
	} else if sustained > onset {
		ev.Stage = model.EventSustained
	} else {
		ev.Stage = model.EventInception
	}
	return []model.CavitationEvent{ev}, nil
}

// confidence 计算事件置信度：基于缺口比峰值相对阈值的超出幅度，扣除噪声通道惩罚。
func (c *Classifier) confidence(features []model.HarmonicFeatures, cfg *model.ThresholdConfig, noisyChannels int) float64 {
	if cfg.GapRatioThreshold <= 0 {
		return 0.5
	}
	peak := 0.0
	for _, f := range features {
		if f.GapRatio > peak {
			peak = f.GapRatio
		}
	}
	excess := (peak - cfg.GapRatioThreshold) / cfg.GapRatioThreshold
	base := 0.5 + 0.35*excess
	if base > 0.95 {
		base = 0.95
	}
	penalty := 0.05 * float64(noisyChannels)
	conf := base - penalty
	if conf < 0.05 {
		conf = 0.05
	}
	return math.Round(conf*1000) / 1000
}
