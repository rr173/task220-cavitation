package harmonic

// GapAnalyzer 谐波缺口分析：基于缺口比序列判定空化相关声学状态。
// 它不直接落地状态机（状态机在 cavitation 包），只提供缺口比的
// 平滑与越界判断，供事件判定层消费。
type GapAnalyzer struct {
	// SmoothWindow 平滑窗口（滑动平均半径）。
	SmoothWindow int
}

// NewGapAnalyzer 构造缺口分析器（默认平滑窗口 2）。
func NewGapAnalyzer() *GapAnalyzer {
	return &GapAnalyzer{SmoothWindow: 2}
}

// Smooth 对缺口比序列做滑动平均，抑制单窗口抖动。
func (g *GapAnalyzer) Smooth(ratios []float64) []float64 {
	out := make([]float64, len(ratios))
	for i := range ratios {
		lo := i - g.SmoothWindow
		hi := i + g.SmoothWindow
		if lo < 0 {
			lo = 0
		}
		if hi >= len(ratios) {
			hi = len(ratios) - 1
		}
		var s float64
		for j := lo; j <= hi; j++ {
			s += ratios[j]
		}
		out[i] = s / float64(hi-lo+1)
	}
	return out
}

// IsAbove 判断平滑后的缺口比是否越过阈值（含 Inf 处理）。
func IsAbove(ratio, threshold float64) bool {
	return ratio > threshold
}

// Trend 返回缺口比序列首尾差（正=上升，负=下降），用于起始/消退判定。
func Trend(ratios []float64) float64 {
	if len(ratios) < 2 {
		return 0
	}
	return ratios[len(ratios)-1] - ratios[0]
}

// StableAbove 判断连续窗口数是否达到确认阈值。
func StableAbove(flags []bool, confirmWindows int) bool {
	if confirmWindows <= 0 {
		return false
	}
	if len(flags) < confirmWindows {
		return false
	}
	for i := len(flags) - confirmWindows; i < len(flags); i++ {
		if !flags[i] {
			return false
		}
	}
	return true
}
