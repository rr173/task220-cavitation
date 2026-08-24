// Package model 定义船舶螺旋桨空化声纹判定服务的领域实体、
// 状态常量与领域错误。实体为纯数据载体，不含持久化与 HTTP 逻辑，
// 业务规则由 acq/harmonic/cavitation/threshold/conclusion 各业务包承载。
package model

import "errors"

// 领域错误哨兵：service 与 httpapi 依据它们做错误映射。
var (
	// ErrNotFound 目标资源不存在。
	ErrNotFound = errors.New("not found")
	// ErrInvalidInput 入参非法（坐标缺失、时间倒退、采样率不一致等）。
	ErrInvalidInput = errors.New("invalid input")
	// ErrInvalidState 状态机不允许的流转（如对已封存试验继续写入）。
	ErrInvalidState = errors.New("invalid state transition")
	// ErrConflict 并发冲突或重复写入。
	ErrConflict = errors.New("conflict")
	// ErrSealed 目标已封存，不可修改。
	ErrSealed = errors.New("sealed")
	// ErrDuplicate 幂等指纹冲突（重复片段/重复通道）。
	ErrDuplicate = errors.New("duplicate")
	// ErrInsufficientData 数据不足（无法提取特征或判定）。
	ErrInsufficientData = errors.New("insufficient data")
	// ErrCalibrationFailed 通道延迟校准失败（互相关无稳定峰）。
	ErrCalibrationFailed = errors.New("calibration failed")
)
