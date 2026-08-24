// Package harmonic 特征模块：从声纹抽样中估计基频、计算谐波与宽带
// 能量并得到"谐波缺口比"。空化起始的声学特征表现为螺旋桨谐波谱线
// 之间被空化泡宽带噪声填充，谐波缺口比随之上升，是阶段判定的核心量。
package harmonic

import "math"

// EstimateFundamental 用 Goertzel 谱扫描估计基频（Hz）：
// 在螺旋桨叶频合理范围（10~200 Hz）内以 2 Hz 步长扫描，
// 取单频功率最大的分量作为基频。相比自相关，对宽带噪声更稳健。
// 返回 0 表示信号过弱或无明显周期性（纯噪声）。
func EstimateFundamental(samples []float64, sampleRate float64) float64 {
	n := len(samples)
	if n < 16 || sampleRate <= 0 {
		return 0
	}
	// 总能量过弱视为静默。
	if totalPower(samples) < 1e-6 {
		return 0
	}
	bestFreq, bestPower := 0.0, 0.0
	for freq := 10.0; freq <= 200.0; freq += 2.0 {
		p := GoertzelPower(samples, sampleRate, freq)
		if p > bestPower {
			bestPower = p
			bestFreq = freq
		}
	}
	if bestPower < 1e-4 {
		return 0
	}
	return bestFreq
}

// GoertzelPower 用 Goertzel 算法估计单个频率分量的相对功率。
// 相比完整 FFT，可对任意目标频率做稀疏谱估计，适合谐波/频隙带内能量计算。
func GoertzelPower(samples []float64, sampleRate, targetFreq float64) float64 {
	n := len(samples)
	if n == 0 || sampleRate <= 0 {
		return 0
	}
	omega := 2 * math.Pi * targetFreq / sampleRate
	coeff := 2 * math.Cos(omega)
	var s0, s1, s2 float64
	for _, x := range samples {
		s0 = x + coeff*s1 - s2
		s2 = s1
		s1 = s0
	}
	power := s1*s1 + s2*s2 - coeff*s1*s2
	return power / float64(n*n)
}

// Autocorrelation 计算零均值归一化自相关 R(lag)（取值 -1~1）。
// R(0) 恒为 1，R(lag0) 反映基频周期成分强度：谐波主导信号接近 1，
// 宽带噪声主导信号显著下降。
func Autocorrelation(samples []float64, lag int) float64 {
	n := len(samples)
	if n == 0 || lag < 0 || lag >= n {
		return 0
	}
	m := mean(samples)
	var num, den1, den2 float64
	for i := 0; i+lag < n; i++ {
		d1 := samples[i] - m
		d2 := samples[i+lag] - m
		num += d1 * d2
		den1 += d1 * d1
		den2 += d2 * d2
	}
	if den1 == 0 || den2 == 0 {
		return 0
	}
	return num / math.Sqrt(den1*den2)
}

func mean(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}
