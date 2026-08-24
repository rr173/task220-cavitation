package harmonic

import (
	"fmt"
	"math"

	"task220-cavitation/internal/model"
)

// MaxHarmonics 参与能量计算的最高谐波阶数。
const MaxHarmonics = 8

// ComputeFeatures 从一段抽样计算完整谐波特征：
//   - 基频（Goertzel 谱扫描估计）
//   - 谐波能量：基频周期处的归一化自相关（周期成分强度）
//   - 宽带能量：总能量减去周期成分（非周期成分强度）
//   - 缺口比：宽带能量 / 谐波能量
//
// 空化起始时空化泡噪声填充谐波频隙，周期成分相对总能量下降，缺口比增大。
func ComputeFeatures(samples []float64, sampleRate float64) (*model.HarmonicFeatures, error) {
	if len(samples) < 16 {
		return nil, fmt.Errorf("%w: too few samples for feature extraction", model.ErrInsufficientData)
	}
	f0 := EstimateFundamental(samples, sampleRate)

	var harmonicEnergy, broadbandEnergy, gapRatio float64
	if f0 > 0 {
		lag0 := int(sampleRate/f0 + 0.5)
		if lag0 < 1 {
			lag0 = 1
		}
		periodic := Autocorrelation(samples, lag0)
		if periodic < 0 {
			periodic = 0
		}
		// 谐波（周期）能量与宽带（非周期）能量。
		harmonicEnergy = periodic
		broadbandEnergy = 1.0 - periodic
		if harmonicEnergy > 1e-9 {
			gapRatio = broadbandEnergy / harmonicEnergy
		} else {
			gapRatio = math.Inf(1)
		}
	} else {
		// 无稳定基频：整体视为宽带噪声。
		broadbandEnergy = 1.0
		gapRatio = math.Inf(1)
	}

	return &model.HarmonicFeatures{
		FundamentalHz:   f0,
		HarmonicEnergy:  harmonicEnergy,
		BroadbandEnergy: broadbandEnergy,
		GapRatio:        gapRatio,
	}, nil
}

func totalPower(samples []float64) float64 {
	var s float64
	for _, x := range samples {
		s += x * x
	}
	return s / float64(len(samples))
}
