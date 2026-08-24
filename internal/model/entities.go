package model

import "time"

// 水池试验状态机：
// preparing(准备) -> acquiring(采集) -> analyzing(分析中) -> confirmed(已确认) -> sealed(封存)。
// 封存为单向终态，封存后不可再写入片段、重新分析或修改事件。
const (
	TrialPreparing = "preparing" // 准备：已登记工况，尚未开始接收声纹
	TrialAcquiring = "acquiring" // 采集：正在接收多通道声纹片段
	TrialAnalyzing = "analyzing" // 分析中：已停止采集，正在提取特征/判定事件
	TrialConfirmed = "confirmed" // 已确认：空化事件已由工程师确认
	TrialSealed    = "sealed"    // 封存：结论已发布，冻结为不可变
)

// 声纹片段状态机：
// pending_calibration(待校准) -> valid(有效)/noisy(噪声)/missing(缺失)/duplicate(重复)。
// 机械噪声仅降低置信度，不删除原片段。
const (
	SegmentPendingCalibration = "pending_calibration" // 待校准：已入库，未完成通道延迟对齐
	SegmentValid              = "valid"               // 有效：通过校验，参与特征提取
	SegmentNoisy              = "mechanical_noise"    // 噪声：被标记为机械噪声，置信度降权
	SegmentMissing            = "missing"             // 缺失：通道数据缺口，不参与判定
	SegmentDuplicate          = "duplicate"           // 重复：幂等指纹冲突，跳过
)

// 空化事件阶段状态机：
// candidate(候选) -> inception(起始) -> sustained(持续) -> decay(消退)；任一阶段可被否决为 rejected。
const (
	EventCandidate = "candidate" // 候选：谐波缺口首次越过阈值
	EventInception = "inception" // 起始：连续窗口确认缺口稳定，判定空化起始
	EventSustained = "sustained" // 持续：缺口维持在阈值之上
	EventDecay     = "decay"     // 消退：缺口回落，空化消退
	EventRejected  = "rejected"  // 否决：工程师判定为误报/机械噪声
)

// 结论包状态机：
// draft(草稿) -> review(复核) -> published(发布)；发布后可产生替代版本 superseded。
const (
	PackageDraft      = "draft"      // 草稿：正在汇总事件与置信度
	PackageReview     = "review"     // 复核：等待工程师确认
	PackagePublished  = "published"  // 发布：已冻结为结论快照
	PackageSuperseded = "superseded" // 替代：被更新的结论版本取代
)

// ChannelDelayStatus 通道延迟校准状态。
const (
	DelayPending  = "pending"  // 待校准
	DelayLocked   = "locked"   // 已锁定：互相关稳定
	DelayExcluded = "excluded" // 已排除：无稳定峰，通道不可用
)

// Trial 是一次螺旋桨空化水池试验，承载工况（转速/来流压力）与声纹采集。
type Trial struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	ShaftSpeedRPM     float64   `json:"shaft_speed_rpm"`     // 螺旋桨转速（转/分）
	InflowPressureKPa float64   `json:"inflow_pressure_kpa"` // 来流压力（千帕）
	ReferenceChannel  int       `json:"reference_channel"`   // 延迟校准参考通道号
	Status            string    `json:"status"`
	Fingerprint       string    `json:"fingerprint"` // 幂等指纹：名称+工况哈希
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// AcousticSegment 是一段多通道声纹片段：携带波形摘要、采样率与时间窗。
type AcousticSegment struct {
	ID            string    `json:"id"`
	TrialID       string    `json:"trial_id"`
	ChannelIndex  int       `json:"channel_index"`  // 通道号（0 起始）
	SampleRateHz  float64   `json:"sample_rate_hz"` // 采样率（Hz）
	StartTimeMs   int64     `json:"start_time_ms"`  // 相对试验起始的毫秒偏移
	DurationMs    int64     `json:"duration_ms"`    // 片段时长（毫秒）
	Samples       string    `json:"samples"`        // 波形抽样摘要（JSON 数组，每窗口归一化幅度）
	PeakAmplitude float64   `json:"peak_amplitude"` // 峰值幅度（归一化 0~1）
	RMS           float64   `json:"rms"`            // 均方根幅度
	Fingerprint   string    `json:"fingerprint"`    // 幂等指纹：试验+通道+起始时间哈希
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// ChannelDelay 记录某通道相对参考通道的延迟校准结果。
type ChannelDelay struct {
	TrialID          string    `json:"trial_id"`
	ChannelIndex     int       `json:"channel_index"`
	DelayMs          float64   `json:"delay_ms"`          // 相对参考通道延迟（毫秒，可为负）
	CorrelationScore float64   `json:"correlation_score"` // 互相关峰值（0~1）
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

// HarmonicFeatures 是一个时间窗的谐波特征快照：基频、谐波能量与缺口比。
type HarmonicFeatures struct {
	ID              string    `json:"id"`
	TrialID         string    `json:"trial_id"`
	WindowStartMs   int64     `json:"window_start_ms"`
	WindowEndMs     int64     `json:"window_end_ms"`
	FundamentalHz   float64   `json:"fundamental_hz"`   // 估计基频（Hz）
	HarmonicEnergy  float64   `json:"harmonic_energy"`  // 谐波频带能量（归一化）
	BroadbandEnergy float64   `json:"broadband_energy"` // 宽带噪声能量（归一化）
	GapRatio        float64   `json:"gap_ratio"`        // 谐波缺口比 = 宽带/谐波
	CreatedAt       time.Time `json:"created_at"`
}

// CavitationEvent 是一次空化事件：跨窗口追踪起始/持续/消退。
type CavitationEvent struct {
	ID               string    `json:"id"`
	TrialID          string    `json:"trial_id"`
	Stage            string    `json:"stage"`
	OnsetMs          int64     `json:"onset_ms"`          // 起始时间偏移
	SustainedMs      int64     `json:"sustained_ms"`      // 持续确认时间偏移
	DecayMs          int64     `json:"decay_ms"`          // 消退时间偏移
	Confidence       float64   `json:"confidence"`        // 置信度（0~1）
	EvidenceSegments string    `json:"evidence_segments"` // 证据片段 ID（JSON 数组）
	RejectReason     string    `json:"reject_reason"`     // 否决原因（可空）
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ConclusionPackage 是发布的试验结论包：冻结事件与置信度的不可变快照。
type ConclusionPackage struct {
	ID               string     `json:"id"`
	TrialID          string     `json:"trial_id"`
	Version          int        `json:"version"`
	Status           string     `json:"status"`
	ThresholdVersion int        `json:"threshold_version"` // 使用的阈值版本
	EventsJSON       string     `json:"events_json"`       // 事件快照（JSON）
	Summary          string     `json:"summary"`
	Confidence       float64    `json:"confidence"`
	CreatedAt        time.Time  `json:"created_at"`
	PublishedAt      *time.Time `json:"published_at"`
}

// ThresholdConfig 是空化判定的阈值配置版本。
type ThresholdConfig struct {
	Version           int       `json:"version"`
	GapRatioThreshold float64   `json:"gap_ratio_threshold"` // 谐波缺口比判定阈值
	EnergyFloor       float64   `json:"energy_floor"`        // 能量下限（低于视为静默）
	ConfirmWindows    int       `json:"confirm_windows"`     // 连续确认窗口数
	CreatedAt         time.Time `json:"created_at"`
}

// StatSummary 是自检/统计接口返回的全局快照。
type StatSummary struct {
	Trials     int `json:"trials"`
	Segments   int `json:"segments"`
	Features   int `json:"features"`
	Events     int `json:"events"`
	Packages   int `json:"packages"`
	Thresholds int `json:"thresholds"`
	OpenTrials int `json:"open_trials"`
}
