package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"quarantine-workbench/internal/domain"
)

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadAggregate(ctx context.Context, q queryer, id string) (*domain.CaseAggregate, error) {
	c, err := scanCase(q.QueryRowContext(ctx, `SELECT id,accession_code,scientific_name,origin_region,introduction_purpose,quarantine_zone,status,risk_level,observation_started_at,expected_release_at,revision,created_at,closed_at FROM cases WHERE id=?`, id))
	if err != nil {
		return nil, normalizeNotFound(err)
	}
	agg := &domain.CaseAggregate{Case: c, Observations: []domain.ObservationEntry{}, Deviations: []domain.Deviation{}}
	var r domain.RiskAssessment
	var pathways, hosts, conditions, reasons, level, review, reviewedAt, submitted string
	err = q.QueryRowContext(ctx, `SELECT id,case_id,spread_pathways,potential_hosts,source_confidence,quarantine_days,observation_interval_days,release_conditions,calculated_level,risk_reasons,review_status,review_reason,reviewed_by,COALESCE(reviewed_at,''),submitted_at FROM risk_assessments WHERE case_id=?`, id).Scan(&r.ID, &r.CaseID, &pathways, &hosts, &r.SourceConfidence, &r.QuarantineDays, &r.ObservationIntervalDays, &conditions, &level, &reasons, &review, &r.ReviewReason, &r.ReviewedBy, &reviewedAt, &submitted)
	if err == nil {
		json.Unmarshal([]byte(pathways), &r.SpreadPathways)
		json.Unmarshal([]byte(hosts), &r.PotentialHosts)
		json.Unmarshal([]byte(conditions), &r.ReleaseConditions)
		json.Unmarshal([]byte(reasons), &r.RiskReasons)
		r.CalculatedLevel = domain.RiskLevel(level)
		r.ReviewStatus = domain.ReviewStatus(review)
		r.SubmittedAt = parseTime(submitted)
		if reviewedAt != "" {
			t := parseTime(reviewedAt)
			r.ReviewedAt = &t
		}
		agg.Risk = &r
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	rowsB, err := q.QueryContext(ctx, `SELECT version, payload FROM risk_baselines WHERE case_id=? ORDER BY version`, id)
	if err != nil {
		return nil, fmt.Errorf("加载风险基线失败(case_id=%s, table=risk_baselines): %w", id, err)
	}
	for rowsB.Next() {
		var version int
		var raw string
		if err = rowsB.Scan(&version, &raw); err != nil {
			rowsB.Close()
			return nil, fmt.Errorf("扫描风险基线失败(case_id=%s, table=risk_baselines): %w", id, err)
		}
		var b domain.RiskBaseline
		if err = json.Unmarshal([]byte(raw), &b); err != nil {
			rowsB.Close()
			return nil, fmt.Errorf("解析风险基线失败(case_id=%s, table=risk_baselines, version=%d): %w", id, version, err)
		}
		agg.RiskBaselines = append(agg.RiskBaselines, b)
	}
	if err = rowsB.Err(); err != nil {
		rowsB.Close()
		return nil, fmt.Errorf("遍历风险基线失败(case_id=%s, table=risk_baselines): %w", id, err)
	}
	rowsB.Close()
	var checklistRaw string
	if err = q.QueryRowContext(ctx, `SELECT payload FROM review_checklists WHERE case_id=? ORDER BY baseline_version DESC LIMIT 1`, id).Scan(&checklistRaw); err == nil {
		var cl domain.ReviewChecklist
		if json.Unmarshal([]byte(checklistRaw), &cl) == nil {
			agg.ReviewChecklist = &cl
		}
	}
	rowsS, err := q.QueryContext(ctx, `SELECT payload FROM eligibility_snapshots WHERE case_id=? ORDER BY revision`, id)
	if err == nil {
		for rowsS.Next() {
			var raw string
			if rowsS.Scan(&raw) == nil {
				var snap domain.EligibilitySnapshot
				if json.Unmarshal([]byte(raw), &snap) == nil {
					agg.EligibilitySnapshots = append(agg.EligibilitySnapshots, snap)
				}
			}
		}
		rowsS.Close()
	}
	rows, err := q.QueryContext(ctx, `SELECT id,case_id,observed_on,growth_condition,pest_signs,reproduction_signs,sample_reference,notes,recorded_by,recorded_at,COALESCE(window_due_on,''),late,late_reason,late_status,late_reviewed_by,COALESCE(late_reviewed_at,'') FROM observations WHERE case_id=? ORDER BY observed_on`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var o domain.ObservationEntry
		var observed, recorded, windowDue, lateReviewed string
		var late int
		if err = rows.Scan(&o.ID, &o.CaseID, &observed, &o.GrowthCondition, &o.PestSigns, &o.ReproductionSigns, &o.SampleReference, &o.Notes, &o.RecordedBy, &recorded, &windowDue, &late, &o.LateReason, &o.LateStatus, &o.LateReviewedBy, &lateReviewed); err != nil {
			rows.Close()
			return nil, err
		}
		o.ObservedOn = parseTime(observed)
		o.RecordedAt = parseTime(recorded)
		o.Late = late != 0
		if windowDue != "" {
			t := parseTime(windowDue)
			o.WindowDueOn = &t
		}
		if lateReviewed != "" {
			t := parseTime(lateReviewed)
			o.LateReviewedAt = &t
		}
		agg.Observations = append(agg.Observations, o)
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT id,case_id,severity,scope,finding,containment_action,status,verification_note,opened_at,COALESCE(verified_at,''),COALESCE(verification_due_at,''),escalated,verification_evidence,assigned_role,rounds FROM deviations WHERE case_id=? ORDER BY opened_at`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d domain.Deviation
		var status, opened, verified, due, rounds string
		var escalated int
		if err = rows.Scan(&d.ID, &d.CaseID, &d.Severity, &d.Scope, &d.Finding, &d.ContainmentAction, &status, &d.VerificationNote, &opened, &verified, &due, &escalated, &d.VerificationEvidence, &d.AssignedRole, &rounds); err != nil {
			rows.Close()
			return nil, err
		}
		d.Status = domain.DeviationStatus(status)
		d.OpenedAt = parseTime(opened)
		if verified != "" {
			t := parseTime(verified)
			d.VerifiedAt = &t
		}
		if due != "" {
			t := parseTime(due)
			d.VerificationDueAt = &t
		}
		d.Escalated = escalated != 0
		json.Unmarshal([]byte(rounds), &d.Rounds)
		agg.Deviations = append(agg.Deviations, d)
	}
	rows.Close()
	var d domain.ReleaseDecision
	var snapshot, outcome, decided, archived string
	var integrity string
	err = q.QueryRowContext(ctx, `SELECT id,case_id,eligibility_snapshot,outcome,rationale,decided_by,decided_at,archived_at,COALESCE(integrity,'') FROM decisions WHERE case_id=?`, id).Scan(&d.ID, &d.CaseID, &snapshot, &outcome, &d.Rationale, &d.DecidedBy, &decided, &archived, &integrity)
	if err == nil {
		json.Unmarshal([]byte(snapshot), &d.EligibilitySnapshot)
		d.Outcome = domain.Outcome(outcome)
		d.DecidedAt = parseTime(decided)
		d.ArchivedAt = parseTime(archived)
		if integrity != "" {
			var ai domain.ArchiveIntegrity
			if json.Unmarshal([]byte(integrity), &ai) == nil {
				agg.ArchiveIntegrity = &ai
			}
		}
		agg.Decision = &d
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	return agg, nil
}
