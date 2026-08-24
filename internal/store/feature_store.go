package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task220-cavitation/internal/model"
)

// FeatureStore 通道延迟与谐波特征表 CRUD。
type FeatureStore struct{ db *DB }

// NewFeatureStore 构造特征存储。
func NewFeatureStore(db *DB) *FeatureStore { return &FeatureStore{db: db} }

// UpsertChannelDelay 写入或更新通道延迟校准（主键 trial_id+channel_index）。
func (s *FeatureStore) UpsertChannelDelay(d *model.ChannelDelay) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO channel_delays (trial_id, channel_index, delay_ms, correlation_score, status, created_at)
		 VALUES (?,?,?,?,?,?)
		 ON CONFLICT(trial_id, channel_index) DO UPDATE SET
		   delay_ms=excluded.delay_ms, correlation_score=excluded.correlation_score, status=excluded.status`,
		d.TrialID, d.ChannelIndex, d.DelayMs, d.CorrelationScore, d.Status, ts(d.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("upsert channel delay: %w", err)
	}
	return nil
}

// GetChannelDelay 读取单个通道的延迟校准。
func (s *FeatureStore) GetChannelDelay(trialID string, channel int) (*model.ChannelDelay, error) {
	var d model.ChannelDelay
	var created string
	err := s.db.SQL().QueryRow(
		`SELECT trial_id, channel_index, delay_ms, correlation_score, status, created_at
		 FROM channel_delays WHERE trial_id = ? AND channel_index = ?`, trialID, channel).
		Scan(&d.TrialID, &d.ChannelIndex, &d.DelayMs, &d.CorrelationScore, &d.Status, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if d.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	return &d, nil
}

// ListChannelDelays 返回试验全部通道延迟。
func (s *FeatureStore) ListChannelDelays(trialID string) ([]model.ChannelDelay, error) {
	rows, err := s.db.SQL().Query(
		`SELECT trial_id, channel_index, delay_ms, correlation_score, status, created_at
		 FROM channel_delays WHERE trial_id = ? ORDER BY channel_index`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ChannelDelay
	for rows.Next() {
		var d model.ChannelDelay
		var created string
		if err := rows.Scan(&d.TrialID, &d.ChannelIndex, &d.DelayMs, &d.CorrelationScore, &d.Status, &created); err != nil {
			return nil, err
		}
		if d.CreatedAt, err = parseTS(created); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

const featureCols = `id, trial_id, window_start_ms, window_end_ms, fundamental_hz, harmonic_energy, broadband_energy, gap_ratio, created_at`

func scanFeature(sc scanner) (*model.HarmonicFeatures, error) {
	var f model.HarmonicFeatures
	var created string
	if err := sc.Scan(&f.ID, &f.TrialID, &f.WindowStartMs, &f.WindowEndMs, &f.FundamentalHz,
		&f.HarmonicEnergy, &f.BroadbandEnergy, &f.GapRatio, &created); err != nil {
		return nil, err
	}
	var err error
	if f.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	return &f, nil
}

// InsertFeature 插入谐波特征。窗口唯一约束冲突时返回 ErrDuplicate。
func (s *FeatureStore) InsertFeature(f *model.HarmonicFeatures) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO harmonic_features (id, trial_id, window_start_ms, window_end_ms, fundamental_hz, harmonic_energy, broadband_energy, gap_ratio, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?)`,
		f.ID, f.TrialID, f.WindowStartMs, f.WindowEndMs, f.FundamentalHz,
		f.HarmonicEnergy, f.BroadbandEnergy, f.GapRatio, ts(f.CreatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.ErrDuplicate
		}
		return fmt.Errorf("insert feature: %w", err)
	}
	return nil
}

// ListFeaturesByTrial 返回试验全部特征（按窗口起始时间排序）。
func (s *FeatureStore) ListFeaturesByTrial(trialID string) ([]model.HarmonicFeatures, error) {
	rows, err := s.db.SQL().Query(
		`SELECT `+featureCols+` FROM harmonic_features WHERE trial_id = ? ORDER BY window_start_ms`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.HarmonicFeatures
	for rows.Next() {
		f, err := scanFeature(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}
