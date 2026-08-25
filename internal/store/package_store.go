package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task220-cavitation/internal/model"
)

// PackageStore 结论包与阈值配置表 CRUD。
type PackageStore struct{ db *DB }

// NewPackageStore 构造结论/阈值存储。
func NewPackageStore(db *DB) *PackageStore { return &PackageStore{db: db} }

// --- 阈值配置 ---

// InsertThreshold 插入阈值配置版本。
func (s *PackageStore) InsertThreshold(c *model.ThresholdConfig) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO thresholds (version, gap_ratio_threshold, energy_floor, confirm_windows, created_at)
		 VALUES (?,?,?,?,?)`,
		c.Version, c.GapRatioThreshold, c.EnergyFloor, c.ConfirmWindows, ts(c.CreatedAt),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.ErrDuplicate
		}
		return fmt.Errorf("insert threshold: %w", err)
	}
	return nil
}

// GetThreshold 按版本读取阈值。
func (s *PackageStore) GetThreshold(version int) (*model.ThresholdConfig, error) {
	var c model.ThresholdConfig
	var created string
	err := s.db.SQL().QueryRow(
		`SELECT version, gap_ratio_threshold, energy_floor, confirm_windows, created_at FROM thresholds WHERE version = ?`, version).
		Scan(&c.Version, &c.GapRatioThreshold, &c.EnergyFloor, &c.ConfirmWindows, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if c.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	return &c, nil
}

// LatestThreshold 返回最大版本阈值。
func (s *PackageStore) LatestThreshold() (*model.ThresholdConfig, error) {
	var c model.ThresholdConfig
	var created string
	err := s.db.SQL().QueryRow(
		`SELECT version, gap_ratio_threshold, energy_floor, confirm_windows, created_at FROM thresholds ORDER BY version DESC LIMIT 1`).
		Scan(&c.Version, &c.GapRatioThreshold, &c.EnergyFloor, &c.ConfirmWindows, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if c.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	return &c, nil
}

// ListThresholds 返回全部阈值版本。
func (s *PackageStore) ListThresholds() ([]model.ThresholdConfig, error) {
	rows, err := s.db.SQL().Query(
		`SELECT version, gap_ratio_threshold, energy_floor, confirm_windows, created_at FROM thresholds ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ThresholdConfig
	for rows.Next() {
		var c model.ThresholdConfig
		var created string
		if err := rows.Scan(&c.Version, &c.GapRatioThreshold, &c.EnergyFloor, &c.ConfirmWindows, &created); err != nil {
			return nil, err
		}
		if c.CreatedAt, err = parseTS(created); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// --- 结论包 ---

const packageCols = `id, trial_id, version, status, threshold_version, events_json, summary, confidence, created_at, published_at`

func scanPackage(sc scanner) (*model.ConclusionPackage, error) {
	var p model.ConclusionPackage
	var created string
	var published sql.NullString
	if err := sc.Scan(&p.ID, &p.TrialID, &p.Version, &p.Status, &p.ThresholdVersion,
		&p.EventsJSON, &p.Summary, &p.Confidence, &created, &published); err != nil {
		return nil, err
	}
	var err error
	if p.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	if published.Valid {
		pt, err := parseTS(published.String)
		if err != nil {
			return nil, err
		}
		p.PublishedAt = &pt
	}
	return &p, nil
}

// InsertPackage 插入结论包。events_json 是冻结的事件快照，必须随包
// 一并落库，否则重读结论包时事件证据丢失，与发布时不一致。
func (s *PackageStore) InsertPackage(p *model.ConclusionPackage) error {
	var published any
	if p.PublishedAt != nil {
		published = ts(*p.PublishedAt)
	}
	eventsJSON := p.EventsJSON
	if eventsJSON == "" {
		eventsJSON = "[]"
	}
	_, err := s.db.SQL().Exec(
		`INSERT INTO packages (id, trial_id, version, status, threshold_version, events_json, summary, confidence, created_at, published_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.TrialID, p.Version, p.Status, p.ThresholdVersion, eventsJSON, p.Summary, p.Confidence, ts(p.CreatedAt), published,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.ErrDuplicate
		}
		return fmt.Errorf("insert package: %w", err)
	}
	return nil
}

// GetPackage 按 ID 读取结论包。
func (s *PackageStore) GetPackage(id string) (*model.ConclusionPackage, error) {
	row := s.db.SQL().QueryRow(`SELECT `+packageCols+` FROM packages WHERE id = ?`, id)
	p, err := scanPackage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// ListPackagesByTrial 返回试验全部结论包（按版本）。
func (s *PackageStore) ListPackagesByTrial(trialID string) ([]model.ConclusionPackage, error) {
	rows, err := s.db.SQL().Query(
		`SELECT `+packageCols+` FROM packages WHERE trial_id = ? ORDER BY version`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ConclusionPackage
	for rows.Next() {
		p, err := scanPackage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// LatestPublished 返回试验最新已发布结论包。
func (s *PackageStore) LatestPublished(trialID string) (*model.ConclusionPackage, error) {
	row := s.db.SQL().QueryRow(
		`SELECT `+packageCols+` FROM packages WHERE trial_id = ? AND status = ? ORDER BY version DESC LIMIT 1`,
		trialID, model.PackagePublished)
	p, err := scanPackage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateStatus 更新结论包状态（发布写入 published_at）。
func (s *PackageStore) UpdateStatus(id, status string, publishedAt *time.Time) error {
	var published any
	if publishedAt != nil {
		published = ts(*publishedAt)
	}
	_, err := s.db.SQL().Exec(
		`UPDATE packages SET status = ?, published_at = ? WHERE id = ?`, status, published, id)
	if err != nil {
		return fmt.Errorf("update package status: %w", err)
	}
	return nil
}
