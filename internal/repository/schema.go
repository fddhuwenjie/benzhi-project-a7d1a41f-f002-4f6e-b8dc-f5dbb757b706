package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, dataSource string) (*Repository, error) {
	db, err := sql.Open("sqlite", dataSource)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.ExecContext(ctx, `PRAGMA foreign_keys=ON`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.ExecContext(ctx, `PRAGMA busy_timeout=5000`); err != nil {
		db.Close()
		return nil, err
	}
	if err = migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return New(db), nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS cases (id TEXT PRIMARY KEY, accession_code TEXT NOT NULL UNIQUE, scientific_name TEXT NOT NULL, origin_region TEXT NOT NULL, introduction_purpose TEXT NOT NULL, quarantine_zone TEXT NOT NULL, status TEXT NOT NULL, risk_level TEXT NOT NULL, observation_started_at TEXT, expected_release_at TEXT, revision INTEGER NOT NULL CHECK(revision>0), created_at TEXT NOT NULL, closed_at TEXT)`,
		`CREATE TABLE IF NOT EXISTS risk_assessments (id TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE REFERENCES cases(id) ON DELETE CASCADE, spread_pathways TEXT NOT NULL, potential_hosts TEXT NOT NULL, source_confidence TEXT NOT NULL, quarantine_days INTEGER NOT NULL, observation_interval_days INTEGER NOT NULL, release_conditions TEXT NOT NULL, calculated_level TEXT NOT NULL, risk_reasons TEXT NOT NULL, review_status TEXT NOT NULL, review_reason TEXT NOT NULL, reviewed_by TEXT NOT NULL, reviewed_at TEXT, submitted_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS observations (id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES cases(id) ON DELETE CASCADE, observed_on TEXT NOT NULL, growth_condition TEXT NOT NULL, pest_signs TEXT NOT NULL, reproduction_signs TEXT NOT NULL, sample_reference TEXT NOT NULL, notes TEXT NOT NULL, recorded_by TEXT NOT NULL, recorded_at TEXT NOT NULL, UNIQUE(case_id,observed_on))`,
		`CREATE TABLE IF NOT EXISTS deviations (id TEXT PRIMARY KEY, case_id TEXT NOT NULL REFERENCES cases(id) ON DELETE CASCADE, severity TEXT NOT NULL, scope TEXT NOT NULL, finding TEXT NOT NULL, containment_action TEXT NOT NULL, status TEXT NOT NULL, verification_note TEXT NOT NULL, opened_at TEXT NOT NULL, verified_at TEXT, verification_due_at TEXT, escalated INTEGER NOT NULL DEFAULT 0, verification_evidence TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS decisions (id TEXT PRIMARY KEY, case_id TEXT NOT NULL UNIQUE REFERENCES cases(id) ON DELETE CASCADE, eligibility_snapshot TEXT NOT NULL, outcome TEXT NOT NULL, rationale TEXT NOT NULL, decided_by TEXT NOT NULL, decided_at TEXT NOT NULL, archived_at TEXT NOT NULL, integrity TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS audit_events (id INTEGER PRIMARY KEY AUTOINCREMENT, case_id TEXT NOT NULL REFERENCES cases(id) ON DELETE CASCADE, sequence INTEGER NOT NULL, event_type TEXT NOT NULL, actor TEXT NOT NULL, payload TEXT NOT NULL, created_at TEXT NOT NULL, UNIQUE(case_id,sequence))`,
		`CREATE TABLE IF NOT EXISTS request_results (request_id TEXT PRIMARY KEY, case_id TEXT NOT NULL, operation_type TEXT NOT NULL DEFAULT '', request_hash TEXT NOT NULL DEFAULT '', response TEXT NOT NULL, created_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS risk_baselines (case_id TEXT NOT NULL, version INTEGER NOT NULL, payload TEXT NOT NULL, submitted_by TEXT NOT NULL, submitted_at TEXT NOT NULL, PRIMARY KEY(case_id,version), FOREIGN KEY(case_id) REFERENCES cases(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS review_checklists (case_id TEXT NOT NULL, baseline_version INTEGER NOT NULL, payload TEXT NOT NULL, PRIMARY KEY(case_id,baseline_version), FOREIGN KEY(case_id) REFERENCES cases(id) ON DELETE CASCADE)`,
		`CREATE TABLE IF NOT EXISTS eligibility_snapshots (case_id TEXT NOT NULL, revision INTEGER NOT NULL, payload TEXT NOT NULL, created_at TEXT NOT NULL, PRIMARY KEY(case_id,revision), FOREIGN KEY(case_id) REFERENCES cases(id) ON DELETE CASCADE)`,
		`CREATE INDEX IF NOT EXISTS idx_cases_status ON cases(status,created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_observations_case_date ON observations(case_id,observed_on)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_observations_case_sample ON observations(case_id,sample_reference) WHERE trim(sample_reference) <> ''`,
		`CREATE INDEX IF NOT EXISTS idx_deviations_case_status ON deviations(case_id,status)`,
		`ALTER TABLE deviations ADD COLUMN verification_due_at TEXT`,
		`ALTER TABLE deviations ADD COLUMN escalated INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE deviations ADD COLUMN verification_evidence TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE decisions ADD COLUMN integrity TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE observations ADD COLUMN window_due_on TEXT`,
		`ALTER TABLE observations ADD COLUMN late INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE observations ADD COLUMN late_reason TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE observations ADD COLUMN late_status TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE observations ADD COLUMN late_reviewed_by TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE observations ADD COLUMN late_reviewed_at TEXT`,
		`ALTER TABLE deviations ADD COLUMN assigned_role TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE deviations ADD COLUMN rounds TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE request_results ADD COLUMN operation_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE request_results ADD COLUMN request_hash TEXT NOT NULL DEFAULT ''`,
		`CREATE INDEX IF NOT EXISTS idx_request_results_lookup ON request_results(request_id, operation_type, request_hash)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
				continue
			}
			return fmt.Errorf("初始化数据库: %w", err)
		}
	}
	return nil
}

func formatTime(t time.Time) string    { return t.UTC().Format(time.RFC3339Nano) }
func parseTime(value string) time.Time { t, _ := time.Parse(time.RFC3339Nano, value); return t }
func parseNullable(value sql.NullString) *time.Time {
	if !value.Valid {
		return nil
	}
	t := parseTime(value.String)
	return &t
}
