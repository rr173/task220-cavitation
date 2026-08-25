package acq

import (
	"fmt"
	"math"

	"task220-cavitation/internal/model"
)

// Calibrator 多通道延迟校准器：用互相关估计通道相对参考通道的时间延迟。
// 螺旋桨空化声纹的多通道采集因水听器布放与采样时钟差异产生通道延迟，
// 校准时延是谐波特征对齐与空化阶段判定的前提。
type Calibrator struct {
	// MinCorrelation 互相关峰值下限，低于此值判定通道无稳定对齐，标记排除。
	MinCorrelation float64
}

// NewCalibrator 构造校准器（默认互相关下限 0.3）。
func NewCalibrator() *Calibrator {
	return &Calibrator{MinCorrelation: 0.3}
}

// CrossCorrelate 计算 ref 与 other 的零均值归一化互相关，返回最佳 lag（样本点）与峰值。
// lag 为正表示 other 落后于 ref。采用重叠区间归一化，并把搜索范围限制在
// 合理延迟区间内，避免边缘样本过少产生假峰。
func CrossCorrelate(ref, other []float64) (bestLag int, peak float64) {
	n, m := len(ref), len(other)
	if n == 0 || m == 0 {
		return 0, 0
	}
	refMean := mean(ref)
	otherMean := mean(other)

	// 搜索范围限制在 ±maxLag，maxLag 取较短序列的一半。
	maxLag := n / 2
	if m/2 < maxLag {
		maxLag = m / 2
	}
	if maxLag < 1 {
		maxLag = 1
	}

	for lag := -maxLag; lag <= maxLag; lag++ {
		var num, denRef, denOther float64
		for i := 0; i < n; i++ {
			j := i + lag
			if j < 0 || j >= m {
				continue
			}
			d1 := ref[i] - refMean
			d2 := other[j] - otherMean
			num += d1 * d2
			denRef += d1 * d1
			denOther += d2 * d2
		}
		if denRef == 0 || denOther == 0 {
			continue
		}
		c := num / math.Sqrt(denRef*denOther)
		if math.Abs(c) > math.Abs(peak) {
			peak = c
			bestLag = lag
		}
	}
	return bestLag, peak
}

// Calibrate 计算 other 相对 ref 的延迟（毫秒）与校准结果。
// sampleRate 为采样率（Hz），用于把样本 lag 换算为毫秒。
func (c *Calibrator) Calibrate(trialID string, channel int, ref, other []float64, sampleRate float64) (*model.ChannelDelay, error) {
	lag, corr := CrossCorrelate(ref, other)
	// 质量分数取互相关峰值的幅值，保持归一化（0~1）且非负：
	// 最佳对齐可能呈反极性（负相关），其幅值仍代表稳定对齐强度，
	// 不能把负相关值直接持久化为相关度。
	score := math.Abs(corr)
	d := &model.ChannelDelay{
		TrialID:          trialID,
		ChannelIndex:     channel,
		DelayMs:          float64(lag) / sampleRate * 1000.0,
		CorrelationScore: score,
		Status:           model.DelayLocked,
	}
	if d.CorrelationScore < c.MinCorrelation {
		d.Status = model.DelayExcluded
	}
	return d, nil
}

func mean(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

// String 描述校准结果（调试与日志）。
func (c *Calibrator) String() string {
	return fmt.Sprintf("calibrator(minCorrelation=%.2f)", c.MinCorrelation)
}
