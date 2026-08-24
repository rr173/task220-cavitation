package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"task220-cavitation/internal/model"
	"task220-cavitation/internal/store"
)

// TrialService 水池试验生命周期服务。
type TrialService struct {
	store *store.TrialStore
}

// NewTrialService 构造试验服务。
func NewTrialService(s *store.TrialStore) *TrialService { return &TrialService{store: s} }

// Create 登记一次水池试验（初始状态 preparing）。
func (s *TrialService) Create(name, description string, shaftSpeedRPM, inflowPressureKPa float64, referenceChannel int) (*model.Trial, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: trial name required", model.ErrInvalidInput)
	}
	if shaftSpeedRPM <= 0 {
		return nil, fmt.Errorf("%w: shaft speed must be positive", model.ErrInvalidInput)
	}
	if inflowPressureKPa <= 0 {
		return nil, fmt.Errorf("%w: inflow pressure must be positive", model.ErrInvalidInput)
	}
	if referenceChannel < 0 {
		return nil, fmt.Errorf("%w: reference channel must be non-negative", model.ErrInvalidInput)
	}
	fp := trialFingerprint(name, shaftSpeedRPM, inflowPressureKPa)
	now := time.Now().UTC()
	t := &model.Trial{
		ID:                "trial-" + shortHash(fp),
		Name:              name,
		Description:       description,
		ShaftSpeedRPM:     shaftSpeedRPM,
		InflowPressureKPa: inflowPressureKPa,
		ReferenceChannel:  referenceChannel,
		Status:            model.TrialPreparing,
		Fingerprint:       fp,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := s.store.Create(t); err != nil {
		return nil, err
	}
	return t, nil
}

// Get 读取试验。
func (s *TrialService) Get(id string) (*model.Trial, error) { return s.store.Get(id) }

// List 返回全部试验。
func (s *TrialService) List() ([]model.Trial, error) { return s.store.List() }

// StartAcquisition 准备 -> 采集。
func (s *TrialService) StartAcquisition(id string) (*model.Trial, error) {
	return s.transition(id, model.TrialPreparing, model.TrialAcquiring)
}

// FinishAcquisition 采集 -> 分析中。
func (s *TrialService) FinishAcquisition(id string) (*model.Trial, error) {
	return s.transition(id, model.TrialAcquiring, model.TrialAnalyzing)
}

// Confirm 分析中 -> 已确认。
func (s *TrialService) Confirm(id string) (*model.Trial, error) {
	return s.transition(id, model.TrialAnalyzing, model.TrialConfirmed)
}

// Seal 已确认 -> 封存（仅由结论发布服务调用）。
func (s *TrialService) Seal(id string) (*model.Trial, error) {
	return s.transition(id, model.TrialConfirmed, model.TrialSealed)
}

// transition 执行状态流转，非法流转返回 ErrInvalidState。
func (s *TrialService) transition(id, expected, next string) (*model.Trial, error) {
	ok, err := s.store.UpdateStatus(id, expected, next)
	if err != nil {
		return nil, err
	}
	if !ok {
		cur, err := s.store.Get(id)
		if err != nil {
			return nil, err
		}
		if cur.Status == model.TrialSealed {
			return nil, model.ErrSealed
		}
		return nil, fmt.Errorf("%w: cannot move trial from %s to %s", model.ErrInvalidState, cur.Status, next)
	}
	return s.store.Get(id)
}

func trialFingerprint(name string, rpm, pressure float64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%.3f|%.3f", name, rpm, pressure)))
	return hex.EncodeToString(h[:])
}

func shortHash(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
