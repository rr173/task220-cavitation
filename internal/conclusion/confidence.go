package conclusion

import (
	"task220-cavitation/internal/model"
)

// ConfidenceReport 置信度明细：拆解影响最终置信度的各项因子，供复核审计。
type ConfidenceReport struct {
	MaxConfidence     float64 `json:"max_confidence"`
	MeanConfidence    float64 `json:"mean_confidence"`
	EventCount        int     `json:"event_count"`
	RejectedCount     int     `json:"rejected_count"`
	Aggregated        float64 `json:"aggregated"`
}

// Report 生成置信度明细报告。
func Report(events []model.CavitationEvent) ConfidenceReport {
	r := ConfidenceReport{EventCount: len(events)}
	var sum float64
	active := 0
	for _, e := range events {
		if e.Stage == model.EventRejected {
			r.RejectedCount++
			continue
		}
		active++
		sum += e.Confidence
		if e.Confidence > r.MaxConfidence {
			r.MaxConfidence = e.Confidence
		}
	}
	if active > 0 {
		r.MeanConfidence = sum / float64(active)
	}
	r.Aggregated = AggregateConfidence(events)
	return r
}
