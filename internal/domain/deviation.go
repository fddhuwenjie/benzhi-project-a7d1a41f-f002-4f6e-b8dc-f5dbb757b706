package domain

import (
	"strings"
	"time"
)

type Deviation struct {
	ID                   string           `json:"id"`
	CaseID               string           `json:"case_id"`
	Severity             string           `json:"severity"`
	Scope                string           `json:"scope"`
	Finding              string           `json:"finding"`
	ContainmentAction    string           `json:"containment_action"`
	Status               DeviationStatus  `json:"status"`
	VerificationNote     string           `json:"verification_note,omitempty"`
	OpenedAt             time.Time        `json:"opened_at"`
	VerifiedAt           *time.Time       `json:"verified_at,omitempty"`
	VerificationDueAt    *time.Time       `json:"verification_due_at,omitempty"`
	Escalated            bool             `json:"escalated"`
	VerificationEvidence string           `json:"verification_evidence,omitempty"`
	AssignedRole         string           `json:"assigned_role"`
	Rounds               []DeviationRound `json:"rounds,omitempty"`
}
type DeviationRound struct {
	Round            int        `json:"round"`
	Action           string     `json:"action"`
	Result           string     `json:"result,omitempty"`
	Evidence         string     `json:"evidence,omitempty"`
	VerifiedBy       string     `json:"verified_by,omitempty"`
	VerifiedAt       *time.Time `json:"verified_at,omitempty"`
	EscalationReason string     `json:"escalation_reason,omitempty"`
}

func NewDeviation(id, caseID, severity, scope, finding, action string, now time.Time) (*Deviation, error) {
	d := &Deviation{ID: id, CaseID: caseID, Severity: strings.TrimSpace(severity), Scope: strings.TrimSpace(scope), Finding: strings.TrimSpace(finding), ContainmentAction: strings.TrimSpace(action), Status: DeviationOpen, OpenedAt: now.UTC()}
	if d.Severity != "low" && d.Severity != "medium" && d.Severity != "high" {
		return nil, FieldError("severity", "严重度必须为 low、medium 或 high")
	}
	if d.Scope == "" {
		return nil, FieldError("scope", "影响范围不能为空")
	}
	if d.Finding == "" {
		return nil, FieldError("finding", "异常发现不能为空")
	}
	if d.ContainmentAction == "" {
		return nil, FieldError("containment_action", "限制措施不能为空")
	}
	return d, nil
}

func (d *Deviation) Verify(note string, now time.Time) error {
	if d.Status != DeviationOpen {
		return NewError(CodeState, "偏差已经完成验证")
	}
	note = strings.TrimSpace(note)
	if note == "" {
		return FieldError("verification_note", "验证说明不能为空")
	}
	d.Status, d.VerificationNote = DeviationVerified, note
	t := now.UTC()
	d.VerifiedAt = &t
	return nil
}

func (d *Deviation) SetDeadline(now time.Time) {
	days := 7
	if d.Severity == "medium" {
		days = 3
	}
	if d.Severity == "high" {
		days = 1
	}
	t := now.UTC().AddDate(0, 0, days)
	d.VerificationDueAt = &t
	d.Escalated = now.After(t)
}

type ReleaseDecision struct {
	ID                  string            `json:"id"`
	CaseID              string            `json:"case_id"`
	EligibilitySnapshot EligibilityResult `json:"eligibility_snapshot"`
	Outcome             Outcome           `json:"outcome"`
	Rationale           string            `json:"rationale"`
	DecidedBy           string            `json:"decided_by"`
	DecidedAt           time.Time         `json:"decided_at"`
	ArchivedAt          time.Time         `json:"archived_at"`
}

type EligibilityCheck struct {
	Key           string   `json:"key"`
	Label         string   `json:"label"`
	Passed        bool     `json:"passed"`
	Detail        string   `json:"detail"`
	Status        string   `json:"status,omitempty"`
	EvidenceIDs   []string `json:"evidence_ids,omitempty"`
	MissingWindow string   `json:"missing_window,omitempty"`
}

type EligibilityResult struct {
	Eligible        bool               `json:"eligible"`
	CheckedAt       time.Time          `json:"checked_at"`
	Checks          []EligibilityCheck `json:"checks"`
	RemainingDays   int                `json:"remaining_days"`
	MissingEvidence []string           `json:"missing_evidence"`
}

func NewDecision(id, caseID string, result EligibilityResult, outcome Outcome, rationale, actor string, now time.Time) (*ReleaseDecision, error) {
	if outcome != OutcomeRelease && outcome != OutcomeTerminate {
		return nil, FieldError("outcome", "结论必须为 release 或 terminate")
	}
	if outcome == OutcomeRelease && !result.Eligible {
		return nil, NewError(CodeState, "资格检查未通过，不能放行")
	}
	if strings.TrimSpace(rationale) == "" {
		return nil, FieldError("rationale", "决定依据不能为空")
	}
	if strings.TrimSpace(actor) == "" {
		return nil, FieldError("actor", "决定人不能为空")
	}
	return &ReleaseDecision{ID: id, CaseID: caseID, EligibilitySnapshot: result, Outcome: outcome, Rationale: strings.TrimSpace(rationale), DecidedBy: strings.TrimSpace(actor), DecidedAt: now.UTC(), ArchivedAt: now.UTC()}, nil
}
