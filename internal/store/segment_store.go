package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task220-cavitation/internal/model"
)

// SegmentStore 声纹片段表 CRUD。
type SegmentStore struct{ db *DB }

// NewSegmentStore 构造片段存储。
func NewSegmentStore(db *DB) *SegmentStore { return &SegmentStore{db: db} }

const segmentCols = `id, trial_id, channel_index, sample_rate_hz, start_time_ms, duration_ms, samples, peak_amplitude, rms, fingerprint, status, created_at`

func scanSegment(sc scanner) (*model.AcousticSegment, error) {
	var s model.AcousticSegment
	var created string
	if err := sc.Scan(&s.ID, &s.TrialID, &s.ChannelIndex, &s.SampleRateHz, &s.StartTimeMs,
		&s.DurationMs, &s.Samples, &s.PeakAmplitude, &s.RMS, &s.Fingerprint, &s.Status, &created); err != nil {
		return nil, err
	}
	var err error
	if s.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	return &s, nil
}

// Insert 插入片段。唯一约束 (trial_id, channel_index, start_time_ms) 冲突时返回 ErrDuplicate。
func (s *SegmentStore) Insert(seg *model.AcousticSegment) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO segments (id, trial_id, channel_index, sample_rate_hz, start_time_ms, duration_ms, samples, peak_amplitude, rms, fingerprint, status, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		seg.ID, seg.TrialID, seg.ChannelIndex, seg.SampleRateHz, seg.StartTimeMs, seg.DurationMs,
		seg.Samples, seg.PeakAmplitude, seg.RMS, seg.Fingerprint, seg.Status, ts(seg.CreatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.ErrDuplicate
		}
		return fmt.Errorf("insert segment: %w", err)
	}
	return nil
}

// Get 按 ID 读取片段。
func (s *SegmentStore) Get(id string) (*model.AcousticSegment, error) {
	row := s.db.SQL().QueryRow(`SELECT `+segmentCols+` FROM segments WHERE id = ?`, id)
	seg, err := scanSegment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return seg, nil
}

// ListByTrial 返回试验的全部片段（按通道与起始时间排序）。
func (s *SegmentStore) ListByTrial(trialID string) ([]model.AcousticSegment, error) {
	rows, err := s.db.SQL().Query(
		`SELECT `+segmentCols+` FROM segments WHERE trial_id = ? ORDER BY channel_index, start_time_ms`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.AcousticSegment
	for rows.Next() {
		seg, err := scanSegment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *seg)
	}
	return out, rows.Err()
}

// UpdateStatus 更新片段状态。机械噪声标记写入 noisy 状态本身，
// 不得在持久化时改写为 valid（否则会丢失 noisy 标记并使噪声片段错误地参与特征提取）。
func (s *SegmentStore) UpdateStatus(id, status string) error {
	_, err := s.db.SQL().Exec(`UPDATE segments SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update segment status: %w", err)
	}
	return nil
}

// ListValidByTrial 返回试验中状态为 valid 的片段（特征提取用）。
func (s *SegmentStore) ListValidByTrial(trialID string) ([]model.AcousticSegment, error) {
	rows, err := s.db.SQL().Query(
		`SELECT `+segmentCols+` FROM segments WHERE trial_id = ? AND status = ? ORDER BY start_time_ms`, trialID, model.SegmentValid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.AcousticSegment
	for rows.Next() {
		seg, err := scanSegment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *seg)
	}
	return out, rows.Err()
}
