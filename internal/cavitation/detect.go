package cavitation

import (
	"task220-cavitation/internal/model"
	"task220-cavitation/internal/harmonic"
)

// Detector 空化阶段判定器：消费缺口比窗口序列与阈值配置，输出
// 事件的生命周期锚点（起始/持续/消退）。判定分两层：
//  1. 单窗口越界：缺口比越过阈值且能量高于下限；
//  2. 阶段确认：连续 confirmWindows 个窗口越界才确认起始，避免单点抖动。
type Detector struct {
	analyzer *harmonic.GapAnalyzer
}

// NewDetector 构造判定器。
func NewDetector() *Detector {
	return &Detector{analyzer: harmonic.NewGapAnalyzer()}
}

// WindowFlag 单窗口判定结果：越界与否。
type WindowFlag struct {
	Feature *model.HarmonicFeatures
	Above   bool // 缺口比越界
}

// JudgeWindows 对特征窗口序列做逐窗口越界判定。
func (d *Detector) JudgeWindows(features []model.HarmonicFeatures, cfg *model.ThresholdConfig) []WindowFlag {
	// 先对缺口比序列做平滑。
	ratios := make([]float64, len(features))
	for i, f := range features {
		ratios[i] = f.GapRatio
	}
	smoothed := d.analyzer.Smooth(ratios)

	flags := make([]WindowFlag, len(features))
	for i, f := range features {
		above := smoothed[i] > cfg.GapRatioThreshold && f.BroadbandEnergy > cfg.EnergyFloor
		cp := f
		flags[i] = WindowFlag{Feature: &cp, Above: above}
	}
	return flags
}

// Detect 从窗口序列中识别一次空化事件的起始/持续/消退锚点。
// 返回 (onsetIdx, sustainedIdx, decayIdx, found)。索引为窗口下标，未找到返回 -1。
func (d *Detector) Detect(features []model.HarmonicFeatures, cfg *model.ThresholdConfig) (onset, sustained, decay int, found bool) {
	flags := d.JudgeWindows(features, cfg)
	above := make([]bool, len(flags))
	for i, f := range flags {
		above[i] = f.Above
	}

	// 起始：首次出现连续 confirmWindows 个越界窗口。
	onset = firstStableStart(above, cfg.ConfirmWindows)
	if onset < 0 {
		return -1, -1, -1, false
	}
	// 持续：起始后再维持 confirmWindows 个窗口。
	sustained = onset + cfg.ConfirmWindows
	if sustained >= len(above) {
		sustained = len(above) - 1
	}
	// 消退：持续后首次连续 confirmWindows 个窗口回落。
	decay = firstStableEnd(above, sustained+1, cfg.ConfirmWindows)
	if decay < 0 {
		decay = -1
	}
	return onset, sustained, decay, true
}

// firstStableStart 从 0 起找首次连续 confirm 个 true 的起点下标。
func firstStableStart(flags []bool, confirm int) int {
	for i := 0; i+confirm <= len(flags); i++ {
		all := true
		for j := i; j < i+confirm; j++ {
			if !flags[j] {
				all = false
				break
			}
		}
		if all {
			return i
		}
	}
	return -1
}

// firstStableEnd 从 from 起找首次连续 confirm 个 false 的起点下标（消退）。
func firstStableEnd(flags []bool, from, confirm int) int {
	if from < 0 {
		from = 0
	}
	for i := from; i+confirm <= len(flags); i++ {
		all := true
		for j := i; j < i+confirm; j++ {
			if flags[j] {
				all = false
				break
			}
		}
		if all {
			return i
		}
	}
	return -1
}
