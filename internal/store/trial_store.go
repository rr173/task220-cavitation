package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task220-cavitation/internal/model"
)

// TrialStore 试验表 CRUD。
type TrialStore struct{ db *DB }

// NewTrialStore 构造试验存储。
func NewTrialStore(db *DB) *TrialStore { return &TrialStore{db: db} }

const trialCols = `id, name, description, shaft_speed_rpm, inflow_pressure_kpa, reference_channel, status, fingerprint, created_at, updated_at`

func scanTrial(sc scanner) (*model.Trial, error) {
	var t model.Trial
	var created, updated string
	if err := sc.Scan(&t.ID, &t.Name, &t.Description, &t.ShaftSpeedRPM, &t.InflowPressureKPa,
		&t.ReferenceChannel, &t.Status, &t.Fingerprint, &created, &updated); err != nil {
		return nil, err
	}
	var err error
	if t.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	if t.UpdatedAt, err = parseTS(updated); err != nil {
		return nil, err
	}
	return &t, nil
}

// Create 插入试验。指纹冲突时返回 ErrDuplicate。
func (s *TrialStore) Create(t *model.Trial) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO trials (id, name, description, shaft_speed_rpm, inflow_pressure_kpa, reference_channel, status, fingerprint, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Name, t.Description, t.ShaftSpeedRPM, t.InflowPressureKPa, t.ReferenceChannel,
		t.Status, t.Fingerprint, ts(t.CreatedAt), ts(t.UpdatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.ErrDuplicate
		}
		return fmt.Errorf("insert trial: %w", err)
	}
	return nil
}

// Get 按 ID 读取试验。
func (s *TrialStore) Get(id string) (*model.Trial, error) {
	row := s.db.SQL().QueryRow(`SELECT `+trialCols+` FROM trials WHERE id = ?`, id)
	t, err := scanTrial(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

// List 返回全部试验（按创建时间倒序）。
func (s *TrialStore) List() ([]model.Trial, error) {
	rows, err := s.db.SQL().Query(`SELECT ` + trialCols + ` FROM trials ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Trial
	for rows.Next() {
		t, err := scanTrial(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// UpdateStatus 更新试验状态（仅当当前状态匹配 expected，防并发乱序流转）。
func (s *TrialStore) UpdateStatus(id, expected, next string) (bool, error) {
	res, err := s.db.SQL().Exec(
		`UPDATE trials SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		next, ts(nowUTC()), id, expected,
	)
	if err != nil {
		return false, fmt.Errorf("update trial status: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
