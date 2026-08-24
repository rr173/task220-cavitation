package store

import (
	"task220-cavitation/internal/model"
)

// StatsStore 全局统计查询。
type StatsStore struct{ db *DB }

// NewStatsStore 构造统计存储。
func NewStatsStore(db *DB) *StatsStore { return &StatsStore{db: db} }

// Global 返回全局统计快照。
func (s *StatsStore) Global() (*model.StatSummary, error) {
	st := &model.StatSummary{}
	counts := []struct {
		query string
		dst   *int
	}{
		{`SELECT COUNT(*) FROM trials`, &st.Trials},
		{`SELECT COUNT(*) FROM segments`, &st.Segments},
		{`SELECT COUNT(*) FROM harmonic_features`, &st.Features},
		{`SELECT COUNT(*) FROM events`, &st.Events},
		{`SELECT COUNT(*) FROM packages`, &st.Packages},
		{`SELECT COUNT(*) FROM thresholds`, &st.Thresholds},
		{`SELECT COUNT(*) FROM trials WHERE status != 'sealed'`, &st.OpenTrials},
	}
	for _, c := range counts {
		if err := s.db.SQL().QueryRow(c.query).Scan(c.dst); err != nil {
			return nil, err
		}
	}
	return st, nil
}
