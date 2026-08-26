package domain

import "time"

type CaseAggregate struct {
	Case                 QuarantineCase        `json:"case"`
	Risk                 *RiskAssessment       `json:"risk,omitempty"`
	Observations         []ObservationEntry    `json:"observations"`
	Deviations           []Deviation           `json:"deviations"`
	Decision             *ReleaseDecision      `json:"decision,omitempty"`
	RiskBaselines        []RiskBaseline        `json:"risk_baselines,omitempty"`
	ReviewChecklist      *ReviewChecklist      `json:"review_checklist,omitempty"`
	EligibilitySnapshots []EligibilitySnapshot `json:"eligibility_snapshots,omitempty"`
	ArchiveIntegrity     *ArchiveIntegrity     `json:"archive_integrity,omitempty"`
}
type ArchiveIntegrity struct {
	Status               string   `json:"status"`
	Missing              []string `json:"missing,omitempty"`
	AuditCount           int      `json:"audit_count"`
	RiskBaselines        int      `json:"risk_baselines"`
	ReviewItems          int      `json:"review_items"`
	Observations         int      `json:"observations"`
	Deviations           int      `json:"deviations"`
	EligibilitySnapshots int      `json:"eligibility_snapshots"`
	Fingerprint          string   `json:"fingerprint,omitempty"`
	Verified             bool     `json:"verified"`
}

type RiskBaseline struct {
	Version     int               `json:"version"`
	Risk        RiskAssessment    `json:"risk"`
	SubmittedBy string            `json:"submitted_by"`
	SubmittedAt time.Time         `json:"submitted_at"`
	Diff        []FieldChange     `json:"diff,omitempty"`
	Trial       *RiskTrialSummary `json:"trial,omitempty"`
}
type RiskTrialSummary struct {
	Token                        string    `json:"token"`
	Level                        RiskLevel `json:"level"`
	Reasons                      []string  `json:"reasons"`
	SuggestedQuarantineDays      int       `json:"suggested_quarantine_days"`
	SuggestedObservationInterval int       `json:"suggested_observation_interval"`
	CreatedAt                    time.Time `json:"created_at"`
}
type FieldChange struct {
	Field  string `json:"field"`
	Kind   string `json:"kind"`
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
}
type ReviewItem struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Status      string     `json:"status"`
	Reason      string     `json:"reason,omitempty"`
	DueAt       *time.Time `json:"due_at,omitempty"`
	Assignee    string     `json:"assignee,omitempty"`
	Escalated   bool       `json:"escalated,omitempty"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
}
type ReviewChecklist struct {
	BaselineVersion int          `json:"baseline_version"`
	Items           []ReviewItem `json:"items"`
	ReviewedBy      string       `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time   `json:"reviewed_at,omitempty"`
	Approved        bool         `json:"approved"`
}
type EligibilitySnapshot struct {
	Revision  int64             `json:"revision"`
	Result    EligibilityResult `json:"result"`
	CreatedAt time.Time         `json:"created_at"`
}

func (a CaseAggregate) OpenDeviationCount() int {
	count := 0
	for _, d := range a.Deviations {
		if d.Status == DeviationOpen {
			count++
		}
	}
	return count
}

func (a *CaseAggregate) FindDeviation(id string) (*Deviation, error) {
	for i := range a.Deviations {
		if a.Deviations[i].ID == id {
			return &a.Deviations[i], nil
		}
	}
	return nil, NewError(CodeNotFound, "未找到指定偏差")
}
