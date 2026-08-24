// Package store 提供基于 SQLite（modernc.org/sqlite，纯 Go 驱动，CGO 无关）的
// 持久化实现：建表迁移与试验/片段/延迟/特征/事件/结论/阈值的 CRUD 及统计查询。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Open 打开（或创建）SQLite 数据库并执行迁移。
// 支持 ":memory:" 用于测试与冒烟验证。
func Open(path string) (*DB, error) {
	if path == ":memory:" {
		db, err := sql.Open("sqlite", "file::memory:?cache=shared")
		if err != nil {
			return nil, fmt.Errorf("open memory db: %w", err)
		}
		db.SetMaxOpenConns(1)
		d := &DB{db: db, path: path}
		if err := d.migrate(); err != nil {
			db.Close()
			return nil, err
		}
		return d, nil
	}

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir db dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set wal: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable fk: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	d := &DB{db: db, path: path}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

// DB 封装 SQLite 连接。
type DB struct {
	db   *sql.DB
	path string
}

// Close 关闭数据库连接。
func (d *DB) Close() error { return d.db.Close() }

// Path 返回数据库路径（调试用）。
func (d *DB) Path() string { return d.path }

// SQL 暴露底层连接（供 Store 实现使用）。
func (d *DB) SQL() *sql.DB { return d.db }

// migrate 建表：全部业务表 + 唯一约束（防重复）。
func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS trials (
			id                  TEXT PRIMARY KEY,
			name                TEXT NOT NULL,
			description         TEXT NOT NULL DEFAULT '',
			shaft_speed_rpm     REAL NOT NULL,
			inflow_pressure_kpa REAL NOT NULL,
			reference_channel   INTEGER NOT NULL DEFAULT 0,
			status              TEXT NOT NULL,
			fingerprint         TEXT NOT NULL,
			created_at          TEXT NOT NULL,
			updated_at          TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_trials_fingerprint ON trials(fingerprint)`,
		`CREATE TABLE IF NOT EXISTS segments (
			id             TEXT PRIMARY KEY,
			trial_id       TEXT NOT NULL REFERENCES trials(id),
			channel_index  INTEGER NOT NULL,
			sample_rate_hz REAL NOT NULL,
			start_time_ms  INTEGER NOT NULL,
			duration_ms    INTEGER NOT NULL,
			samples        TEXT NOT NULL DEFAULT '[]',
			peak_amplitude REAL NOT NULL DEFAULT 0,
			rms            REAL NOT NULL DEFAULT 0,
			fingerprint    TEXT NOT NULL,
			status         TEXT NOT NULL,
			created_at     TEXT NOT NULL,
			UNIQUE(trial_id, channel_index, start_time_ms)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_segments_trial ON segments(trial_id)`,
		`CREATE TABLE IF NOT EXISTS channel_delays (
			trial_id          TEXT NOT NULL REFERENCES trials(id),
			channel_index     INTEGER NOT NULL,
			delay_ms          REAL NOT NULL,
			correlation_score REAL NOT NULL,
			status            TEXT NOT NULL,
			created_at        TEXT NOT NULL,
			PRIMARY KEY (trial_id, channel_index)
		)`,
		`CREATE TABLE IF NOT EXISTS harmonic_features (
			id               TEXT PRIMARY KEY,
			trial_id         TEXT NOT NULL REFERENCES trials(id),
			window_start_ms  INTEGER NOT NULL,
			window_end_ms    INTEGER NOT NULL,
			fundamental_hz   REAL NOT NULL,
			harmonic_energy  REAL NOT NULL,
			broadband_energy REAL NOT NULL,
			gap_ratio        REAL NOT NULL,
			created_at       TEXT NOT NULL,
			UNIQUE(trial_id, window_start_ms, window_end_ms)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_features_trial ON harmonic_features(trial_id)`,
		`CREATE TABLE IF NOT EXISTS events (
			id                TEXT PRIMARY KEY,
			trial_id          TEXT NOT NULL REFERENCES trials(id),
			stage             TEXT NOT NULL,
			onset_ms          INTEGER NOT NULL,
			sustained_ms      INTEGER NOT NULL DEFAULT 0,
			decay_ms          INTEGER NOT NULL DEFAULT 0,
			confidence        REAL NOT NULL DEFAULT 0,
			evidence_segments TEXT NOT NULL DEFAULT '[]',
			reject_reason     TEXT NOT NULL DEFAULT '',
			created_at        TEXT NOT NULL,
			updated_at        TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_trial ON events(trial_id)`,
		`CREATE TABLE IF NOT EXISTS packages (
			id                TEXT PRIMARY KEY,
			trial_id          TEXT NOT NULL REFERENCES trials(id),
			version           INTEGER NOT NULL,
			status            TEXT NOT NULL,
			threshold_version INTEGER NOT NULL,
			events_json       TEXT NOT NULL DEFAULT '[]',
			summary           TEXT NOT NULL DEFAULT '',
			confidence        REAL NOT NULL DEFAULT 0,
			created_at        TEXT NOT NULL,
			published_at      TEXT,
			UNIQUE(trial_id, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_packages_trial ON packages(trial_id)`,
		`CREATE TABLE IF NOT EXISTS thresholds (
			version             INTEGER PRIMARY KEY,
			gap_ratio_threshold REAL NOT NULL,
			energy_floor        REAL NOT NULL,
			confirm_windows     INTEGER NOT NULL,
			created_at          TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w (%s)", err, firstWords(s, 10))
		}
	}
	// 确保至少存在一个默认阈值版本。
	// 默认缺口比阈值 0.15：正常谐波窗口缺口比约 0.003，空化窗口约 0.36，
	// 阈值取两者之间以区分空化起始。
	if _, err := d.db.Exec(`INSERT OR IGNORE INTO thresholds (version, gap_ratio_threshold, energy_floor, confirm_windows, created_at)
		VALUES (1, 0.15, 0.001, 3, ?)`, ts(time.Now().UTC())); err != nil {
		return fmt.Errorf("seed threshold: %w", err)
	}
	return nil
}

func firstWords(s string, n int) string {
	parts := strings.Fields(s)
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, " ")
}

func nowUTC() time.Time { return time.Now().UTC() }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTS(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }

type scanner interface{ Scan(dest ...any) error }
