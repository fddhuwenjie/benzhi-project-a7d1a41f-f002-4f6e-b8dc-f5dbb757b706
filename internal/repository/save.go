package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"quarantine-workbench/internal/domain"
)

func saveAggregate(ctx context.Context, tx *sql.Tx, a *domain.CaseAggregate) error {
	c := a.Case
	result, err := tx.ExecContext(ctx, `UPDATE cases SET accession_code=?,scientific_name=?,origin_region=?,introduction_purpose=?,quarantine_zone=?,status=?,risk_level=?,observation_started_at=?,expected_release_at=?,revision=?,closed_at=? WHERE id=? AND revision=?`, c.AccessionCode, c.ScientificName, c.OriginRegion, c.IntroductionPurpose, c.QuarantineZone, c.Status, c.RiskLevel, timeValue(c.ObservationStartedAt), timeValue(c.ExpectedReleaseAt), c.Revision, timeValue(c.ClosedAt), c.ID, c.Revision-1)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return domain.NewError(domain.CodeConflict, "修订号已被其他操作更新")
	}
	if a.Risk != nil {
		r := a.Risk
		pathways, _ := json.Marshal(r.SpreadPathways)
		hosts, _ := json.Marshal(r.PotentialHosts)
		conditions, _ := json.Marshal(r.ReleaseConditions)
		reasons, _ := json.Marshal(r.RiskReasons)
		_, err = tx.ExecContext(ctx, `INSERT INTO risk_assessments(id,case_id,spread_pathways,potential_hosts,source_confidence,quarantine_days,observation_interval_days,release_conditions,calculated_level,risk_reasons,review_status,review_reason,reviewed_by,reviewed_at,submitted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(case_id) DO UPDATE SET id=excluded.id,spread_pathways=excluded.spread_pathways,potential_hosts=excluded.potential_hosts,source_confidence=excluded.source_confidence,quarantine_days=excluded.quarantine_days,observation_interval_days=excluded.observation_interval_days,release_conditions=excluded.release_conditions,calculated_level=excluded.calculated_level,risk_reasons=excluded.risk_reasons,review_status=excluded.review_status,review_reason=excluded.review_reason,reviewed_by=excluded.reviewed_by,reviewed_at=excluded.reviewed_at,submitted_at=excluded.submitted_at`, r.ID, r.CaseID, string(pathways), string(hosts), r.SourceConfidence, r.QuarantineDays, r.ObservationIntervalDays, string(conditions), r.CalculatedLevel, string(reasons), r.ReviewStatus, r.ReviewReason, r.ReviewedBy, timeValue(r.ReviewedAt), formatTime(r.SubmittedAt))
		if err != nil {
			return err
		}
	}
	for _, b := range a.RiskBaselines {
		payload, _ := json.Marshal(b)
		if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO risk_baselines(case_id,version,payload,submitted_by,submitted_at) VALUES(?,?,?,?,?)`, c.ID, b.Version, string(payload), b.SubmittedBy, formatTime(b.SubmittedAt)); err != nil {
			return err
		}
	}
	if a.ReviewChecklist != nil {
		payload, _ := json.Marshal(a.ReviewChecklist)
		if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO review_checklists(case_id,baseline_version,payload) VALUES(?,?,?)`, c.ID, a.ReviewChecklist.BaselineVersion, string(payload)); err != nil {
			return err
		}
	}
	for _, snap := range a.EligibilitySnapshots {
		payload, _ := json.Marshal(snap)
		if _, err = tx.ExecContext(ctx, `INSERT OR REPLACE INTO eligibility_snapshots(case_id,revision,payload,created_at) VALUES(?,?,?,?)`, c.ID, snap.Revision, string(payload), formatTime(snap.CreatedAt)); err != nil {
			return err
		}
	}
	for _, o := range a.Observations {
		late := 0
		if o.Late {
			late = 1
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO observations(id,case_id,observed_on,growth_condition,pest_signs,reproduction_signs,sample_reference,notes,recorded_by,recorded_at,window_due_on,late,late_reason,late_status,late_reviewed_by,late_reviewed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET late_status=excluded.late_status,late_reviewed_by=excluded.late_reviewed_by,late_reviewed_at=excluded.late_reviewed_at`, o.ID, o.CaseID, formatTime(o.ObservedOn), o.GrowthCondition, o.PestSigns, o.ReproductionSigns, o.SampleReference, o.Notes, o.RecordedBy, formatTime(o.RecordedAt), timeValue(o.WindowDueOn), late, o.LateReason, o.LateStatus, o.LateReviewedBy, timeValue(o.LateReviewedAt))
		if err != nil {
			return err
		}
	}
	for _, d := range a.Deviations {
		escalated := 0
		if d.Escalated {
			escalated = 1
		}
		rounds, _ := json.Marshal(d.Rounds)
		_, err = tx.ExecContext(ctx, `INSERT INTO deviations(id,case_id,severity,scope,finding,containment_action,status,verification_note,opened_at,verified_at,verification_due_at,escalated,verification_evidence,assigned_role,rounds) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET containment_action=excluded.containment_action,status=excluded.status,verification_note=excluded.verification_note,verified_at=excluded.verified_at,verification_due_at=excluded.verification_due_at,escalated=excluded.escalated,verification_evidence=excluded.verification_evidence,assigned_role=excluded.assigned_role,rounds=excluded.rounds`, d.ID, d.CaseID, d.Severity, d.Scope, d.Finding, d.ContainmentAction, d.Status, d.VerificationNote, formatTime(d.OpenedAt), timeValue(d.VerifiedAt), timeValue(d.VerificationDueAt), escalated, d.VerificationEvidence, d.AssignedRole, string(rounds))
		if err != nil {
			return err
		}
	}
	if a.Decision != nil {
		d := a.Decision
		snapshot, _ := json.Marshal(d.EligibilitySnapshot)
		integrity := ""
		if a.ArchiveIntegrity != nil {
			b, _ := json.Marshal(a.ArchiveIntegrity)
			integrity = string(b)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO decisions(id,case_id,eligibility_snapshot,outcome,rationale,decided_by,decided_at,archived_at,integrity) VALUES(?,?,?,?,?,?,?,?,?)`, d.ID, d.CaseID, string(snapshot), d.Outcome, d.Rationale, d.DecidedBy, formatTime(d.DecidedAt), formatTime(d.ArchivedAt), integrity)
		if err != nil {
			return err
		}
	}
	return nil
}

func timeValue(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

func loadRequest(ctx context.Context, tx *sql.Tx, id string) (json.RawMessage, bool, error) {
	var raw string
	err := tx.QueryRowContext(ctx, `SELECT response FROM request_results WHERE request_id=?`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return json.RawMessage(raw), true, nil
}
func saveRequest(ctx context.Context, tx *sql.Tx, id, caseID string, response []byte, now time.Time) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO request_results(request_id,case_id,response,created_at) VALUES(?,?,?,?)`, id, caseID, string(response), formatTime(now))
	return err
}
func appendAudit(ctx context.Context, tx *sql.Tx, caseID, eventType, actor string, payload []byte, now time.Time) error {
	var sequence int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sequence),0)+1 FROM audit_events WHERE case_id=?`, caseID).Scan(&sequence); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_events(case_id,sequence,event_type,actor,payload,created_at) VALUES(?,?,?,?,?,?)`, caseID, sequence, eventType, actor, string(payload), formatTime(now))
	return err
}
