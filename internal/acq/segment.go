// Package acq 采集模块：声纹片段的校验、去重与多通道延迟校准。
// 片段为波形抽样（归一化幅度数组），本包负责其基本合法性校验与
// 通道间互相关对齐，是空化判定的前置数据治理层。
package acq

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"

	"task220-cavitation/internal/model"
)

// Fingerprint 计算片段的幂等指纹：试验 + 通道 + 起始时间 的稳定哈希。
// 相同指纹的重复片段会被去重跳过，保证事件重放幂等。
func Fingerprint(trialID string, channel int, startMs int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d", trialID, channel, startMs)))
	return hex.EncodeToString(h[:16])
}

// Validate 校验片段基本合法性：采样率、时长、时间偏移与通道号。
// 抽样长度（Samples）的校验由调用方在序列化前完成。
func Validate(seg *model.AcousticSegment) error {
	if seg.SampleRateHz <= 0 {
		return fmt.Errorf("%w: sample rate must be positive", model.ErrInvalidInput)
	}
	if seg.DurationMs <= 0 {
		return fmt.Errorf("%w: duration must be positive", model.ErrInvalidInput)
	}
	if seg.StartTimeMs < 0 {
		return fmt.Errorf("%w: start time must be non-negative", model.ErrInvalidInput)
	}
	if seg.ChannelIndex < 0 {
		return fmt.Errorf("%w: channel index must be non-negative", model.ErrInvalidInput)
	}
	return nil
}

// ComputePeakRMS 从抽样数组计算峰值与均方根幅度。
func ComputePeakRMS(samples []float64) (peak, rms float64) {
	var sum float64
	for _, v := range samples {
		a := math.Abs(v)
		if a > peak {
			peak = a
		}
		sum += v * v
	}
	rms = math.Sqrt(sum / float64(len(samples)))
	return peak, rms
}
