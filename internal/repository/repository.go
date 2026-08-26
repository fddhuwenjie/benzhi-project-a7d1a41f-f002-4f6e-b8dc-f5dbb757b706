package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"quarantine-workbench/internal/domain"
)

type Repository struct{ db *sql.DB }

type Mutation func(*domain.CaseAggregate) (any, error)

type MutationResult struct {
	Response json.RawMessage
	Replayed bool
}

func (r *Repository) FindByAccession(ctx context.Context, accession string) (*domain.QuarantineCase, error) {
	accession = strings.ToUpper(strings.TrimSpace(accession))
	if accession == "" {
		return nil, domain.FieldError("accession_code", "材料编号不能为空")
	}
	c, err := scanCase(r.db.QueryRowContext(ctx, `SELECT id,accession_code,scientific_name,origin_region,introduction_purpose,quarantine_zone,status,risk_level,observation_started_at,expected_release_at,revision,created_at,closed_at FROM cases WHERE UPPER(TRIM(accession_code))=?`, accession))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) DB() *sql.DB { return r.db }

func (r *Repository) Close() error { return r.db.Close() }

func (r *Repository) Create(ctx context.Context, c domain.QuarantineCase, requestID, actor string) (MutationResult, error) {
	if strings.TrimSpace(requestID) == "" {
		return MutationResult{}, domain.FieldError("request_id", "request_id 不能为空")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return MutationResult{}, err
	}
	defer tx.Rollback()
	if response, ok, err := loadRequest(ctx, tx, requestID); err != nil {
		return MutationResult{}, err
	} else if ok {
		return MutationResult{Response: response, Replayed: true}, nil
	}
	existing, scanErr := scanCase(tx.QueryRowContext(ctx, `SELECT id,accession_code,scientific_name,origin_region,introduction_purpose,quarantine_zone,status,risk_level,observation_started_at,expected_release_at,revision,created_at,closed_at FROM cases WHERE UPPER(TRIM(accession_code))=?`, strings.ToUpper(strings.TrimSpace(c.AccessionCode))))
	err = scanErr
	if err == nil {
		e := domain.NewError(domain.CodeDuplicate, "材料编号已存在")
		e.Details = map[string]any{"case_id": existing.ID, "status": existing.Status, "summary": existing}
		return MutationResult{}, e
	}
	if err != sql.ErrNoRows {
		return MutationResult{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cases(id,accession_code,scientific_name,origin_region,introduction_purpose,quarantine_zone,status,risk_level,revision,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, c.ID, c.AccessionCode, c.ScientificName, c.OriginRegion, c.IntroductionPurpose, c.QuarantineZone, c.Status, c.RiskLevel, c.Revision, formatTime(c.CreatedAt))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			if existing, scanErr := scanCase(tx.QueryRowContext(ctx, `SELECT id,accession_code,scientific_name,origin_region,introduction_purpose,quarantine_zone,status,risk_level,observation_started_at,expected_release_at,revision,created_at,closed_at FROM cases WHERE accession_code=?`, c.AccessionCode)); scanErr == nil {
				e := domain.NewError(domain.CodeDuplicate, "材料编号已存在")
				e.Details = map[string]any{"case_id": existing.ID, "status": existing.Status, "summary": existing}
				return MutationResult{}, e
			}
			return MutationResult{}, domain.NewError(domain.CodeDuplicate, "材料编号已存在")
		}
		return MutationResult{}, err
	}
	response, _ := json.Marshal(c)
	if err = appendAudit(ctx, tx, c.ID, "case.created", actor, response, c.CreatedAt); err != nil {
		return MutationResult{}, err
	}
	if err = saveRequest(ctx, tx, requestID, c.ID, response, c.CreatedAt); err != nil {
		return MutationResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Response: response}, nil
}

func (r *Repository) Mutate(ctx context.Context, caseID string, expected int64, requestID, eventType, actor string, now time.Time, fn Mutation) (MutationResult, error) {
	if strings.TrimSpace(requestID) == "" {
		return MutationResult{}, domain.FieldError("request_id", "request_id 不能为空")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return MutationResult{}, err
	}
	defer tx.Rollback()
	if response, ok, err := loadRequest(ctx, tx, requestID); err != nil {
		return MutationResult{}, err
	} else if ok {
		return MutationResult{Response: response, Replayed: true}, nil
	}
	agg, err := loadAggregate(ctx, tx, caseID)
	if err != nil {
		return MutationResult{}, err
	}
	if agg.Case.Revision != expected {
		return MutationResult{}, domain.NewError(domain.CodeConflict, fmt.Sprintf("修订号冲突：当前为 %d", agg.Case.Revision))
	}
	before, _ := json.Marshal(agg)
	value, err := fn(agg)
	if err != nil {
		return MutationResult{}, err
	}
	agg.Case.Revision++
	response, err := json.Marshal(value)
	if err != nil {
		return MutationResult{}, err
	}
	if err = saveAggregate(ctx, tx, agg); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "idx_observations_case_sample") || strings.Contains(strings.ToLower(err.Error()), "observations.case_id, observations.sample_reference") {
			return MutationResult{}, domain.NewError(domain.CodeDuplicate, "样本编号已存在")
		}
		if strings.Contains(strings.ToLower(err.Error()), "cases.accession_code") {
			return MutationResult{}, domain.NewError(domain.CodeDuplicate, "材料编号已存在")
		}
		return MutationResult{}, err
	}
	payload, _ := json.Marshal(map[string]any{"revision": agg.Case.Revision, "before": json.RawMessage(before), "after": value, "missing_fields": agg.Case.MissingDraftFields()})
	if err = appendAudit(ctx, tx, caseID, eventType, actor, payload, now); err != nil {
		return MutationResult{}, err
	}
	if err = saveRequest(ctx, tx, requestID, caseID, response, now); err != nil {
		return MutationResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return MutationResult{}, err
	}
	return MutationResult{Response: response}, nil
}

func (r *Repository) Get(ctx context.Context, id string) (*domain.CaseAggregate, error) {
	return loadAggregate(ctx, r.db, id)
}

func (r *Repository) List(ctx context.Context, status string) ([]domain.QuarantineCase, error) {
	query := `SELECT id,accession_code,scientific_name,origin_region,introduction_purpose,quarantine_zone,status,risk_level,observation_started_at,expected_release_at,revision,created_at,closed_at FROM cases`
	args := []any{}
	if status != "" {
		query += ` WHERE status=?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cases []domain.QuarantineCase
	for rows.Next() {
		c, err := scanCase(rows)
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}
	return cases, rows.Err()
}

type CaseFilter struct{ Status, Outcome, RiskLevel, Accession, ScientificName, From, To string }

func (r *Repository) ListFiltered(ctx context.Context, f CaseFilter) ([]domain.QuarantineCase, int, error) {
	q := `SELECT id,accession_code,scientific_name,origin_region,introduction_purpose,quarantine_zone,status,risk_level,observation_started_at,expected_release_at,revision,created_at,closed_at FROM cases WHERE 1=1`
	args := []any{}
	if f.Status != "" {
		q += " AND status=?"
		args = append(args, f.Status)
	}
	if f.RiskLevel != "" {
		q += " AND risk_level=?"
		args = append(args, f.RiskLevel)
	}
	if f.Accession != "" {
		q += " AND accession_code=?"
		args = append(args, f.Accession)
	}
	if f.ScientificName != "" {
		q += " AND scientific_name LIKE ?"
		args = append(args, "%"+f.ScientificName+"%")
	}
	if f.From != "" {
		q += " AND closed_at>=?"
		args = append(args, f.From)
	}
	if f.To != "" {
		q += " AND closed_at<=?"
		args = append(args, f.To)
	}
	q += " ORDER BY COALESCE(closed_at,created_at) DESC,id"
	if f.Outcome != "" {
		q = strings.Replace(q, " WHERE 1=1", " WHERE 1=1 AND id IN (SELECT case_id FROM decisions WHERE outcome=?)", 1)
		args = append([]any{f.Outcome}, args...)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []domain.QuarantineCase
	for rows.Next() {
		c, e := scanCase(rows)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, c)
	}
	return out, len(out), rows.Err()
}

func (r *Repository) Timeline(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,case_id,sequence,event_type,actor,payload,created_at FROM audit_events WHERE case_id=? ORDER BY sequence`, caseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		var raw, created string
		if err := rows.Scan(&e.ID, &e.CaseID, &e.Sequence, &e.Type, &e.Actor, &raw, &created); err != nil {
			return nil, err
		}
		e.Payload = json.RawMessage(raw)
		e.CreatedAt = parseTime(created)
		events = append(events, e)
	}
	return events, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scanCase(s scanner) (domain.QuarantineCase, error) {
	var c domain.QuarantineCase
	var status, risk, created string
	var started, expected, closed sql.NullString
	err := s.Scan(&c.ID, &c.AccessionCode, &c.ScientificName, &c.OriginRegion, &c.IntroductionPurpose, &c.QuarantineZone, &status, &risk, &started, &expected, &c.Revision, &created, &closed)
	if err != nil {
		return c, err
	}
	c.Status = domain.CaseStatus(status)
	c.RiskLevel = domain.RiskLevel(risk)
	c.CreatedAt = parseTime(created)
	c.ObservationStartedAt = parseNullable(started)
	c.ExpectedReleaseAt = parseNullable(expected)
	c.ClosedAt = parseNullable(closed)
	return c, nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewError(domain.CodeNotFound, "未找到指定个案")
	}
	return err
}
