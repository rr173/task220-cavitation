package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task220-cavitation/internal/model"
)

// EventStore 空化事件表 CRUD。
type EventStore struct{ db *DB }

// NewEventStore 构造事件存储。
func NewEventStore(db *DB) *EventStore { return &EventStore{db: db} }

const eventCols = `id, trial_id, stage, onset_ms, sustained_ms, decay_ms, confidence, evidence_segments, reject_reason, created_at, updated_at`

func scanEvent(sc scanner) (*model.CavitationEvent, error) {
	var e model.CavitationEvent
	var created, updated string
	if err := sc.Scan(&e.ID, &e.TrialID, &e.Stage, &e.OnsetMs, &e.SustainedMs, &e.DecayMs,
		&e.Confidence, &e.EvidenceSegments, &e.RejectReason, &created, &updated); err != nil {
		return nil, err
	}
	var err error
	if e.CreatedAt, err = parseTS(created); err != nil {
		return nil, err
	}
	if e.UpdatedAt, err = parseTS(updated); err != nil {
		return nil, err
	}
	return &e, nil
}

// Insert 插入事件。
func (s *EventStore) Insert(e *model.CavitationEvent) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO events (id, trial_id, stage, onset_ms, sustained_ms, decay_ms, confidence, evidence_segments, reject_reason, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.TrialID, e.Stage, e.OnsetMs, e.SustainedMs, e.DecayMs,
		e.Confidence, e.EvidenceSegments, e.RejectReason, ts(e.CreatedAt), ts(e.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// Get 按 ID 读取事件。
func (s *EventStore) Get(id string) (*model.CavitationEvent, error) {
	row := s.db.SQL().QueryRow(`SELECT `+eventCols+` FROM events WHERE id = ?`, id)
	e, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

// ListByTrial 返回试验全部事件（按起始时间排序）。
func (s *EventStore) ListByTrial(trialID string) ([]model.CavitationEvent, error) {
	rows, err := s.db.SQL().Query(
		`SELECT `+eventCols+` FROM events WHERE trial_id = ? ORDER BY onset_ms DESC`, trialID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.CavitationEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// Update 更新事件阶段与时间锚点及置信度。
func (s *EventStore) Update(e *model.CavitationEvent) error {
	_, err := s.db.SQL().Exec(
		`UPDATE events SET stage=?, onset_ms=?, sustained_ms=?, decay_ms=?, confidence=?, evidence_segments=?, reject_reason=?, updated_at=? WHERE id=?`,
		e.Stage, e.OnsetMs, e.SustainedMs, e.DecayMs, e.Confidence, e.EvidenceSegments, e.RejectReason, ts(e.UpdatedAt), e.ID,
	)
	if err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	return nil
}
