// Package conclusion 结论模块：把空化事件与置信度冻结为可发布、可替代的
// 结论包。结论包是不可变快照，发布后只能建立替代版本，保证声纹证据
// 与判定结论的审计可回溯。
package conclusion

import (
	"fmt"
	"sort"
	"strings"

	"task220-cavitation/internal/model"
)

// Builder 结论包构建器：聚合事件、计算置信度并生成摘要。
type Builder struct{}

// NewBuilder 构造构建器。
func NewBuilder() *Builder { return &Builder{} }

// Build 从事件列表构建结论包（草稿态）。
func (b *Builder) Build(trial *model.Trial, events []model.CavitationEvent, thresholdVersion int, eventsJSON string) *model.ConclusionPackage {
	return &model.ConclusionPackage{
		TrialID:          trial.ID,
		Status:           model.PackageDraft,
		ThresholdVersion: thresholdVersion,
		EventsJSON:       eventsJSON,
		Summary:          b.summarize(trial, events),
		Confidence:       AggregateConfidence(events),
	}
}

// summarize 生成可读结论摘要。
func (b *Builder) summarize(trial *model.Trial, events []model.CavitationEvent) string {
	active := 0
	rejected := 0
	for _, e := range events {
		if e.Stage == model.EventRejected {
			rejected++
		} else {
			active++
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "试验 %s（转速 %.0f rpm，压力 %.1f kPa）：识别 %d 个空化事件",
		trial.Name, trial.ShaftSpeedRPM, trial.InflowPressureKPa, active)
	if rejected > 0 {
		fmt.Fprintf(&sb, "，否决 %d 个", rejected)
	}
	if active == 0 {
		sb.WriteString("，未观测到稳定空化起始")
	}
	return sb.String()
}

// AggregateConfidence 聚合多事件置信度：空化事件取最大置信度，无事件返回 0。
func AggregateConfidence(events []model.CavitationEvent) float64 {
	best := 0.0
	for _, e := range events {
		if e.Stage == model.EventRejected {
			continue
		}
		if e.Confidence > best {
			best = e.Confidence
		}
	}
	return best
}

// SortEvents 按起始时间升序排序事件（稳定排序）。
func SortEvents(events []model.CavitationEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].OnsetMs < events[j].OnsetMs
	})
}
