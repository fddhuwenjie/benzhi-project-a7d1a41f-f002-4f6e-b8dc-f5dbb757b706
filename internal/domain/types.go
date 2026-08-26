package domain

import (
	"encoding/json"
	"strings"
	"time"
)

type CaseStatus string

const (
	StatusDraft         CaseStatus = "draft"
	StatusPendingReview CaseStatus = "pending_review"
	StatusApproved      CaseStatus = "approved"
	StatusReturned      CaseStatus = "returned"
	StatusObserving     CaseStatus = "observing"
	StatusRestricted    CaseStatus = "restricted"
	StatusEligible      CaseStatus = "eligible"
	StatusReleased      CaseStatus = "released"
	StatusTerminated    CaseStatus = "terminated"
)

func (s CaseStatus) Closed() bool { return s == StatusReleased || s == StatusTerminated }

type RiskLevel string

const (
	RiskUnknown RiskLevel = "unknown"
	RiskLow     RiskLevel = "low"
	RiskMedium  RiskLevel = "medium"
	RiskHigh    RiskLevel = "high"
)

type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "pending"
	ReviewApproved ReviewStatus = "approved"
	ReviewReturned ReviewStatus = "returned"
)

type DeviationStatus string

const (
	DeviationOpen     DeviationStatus = "open"
	DeviationVerified DeviationStatus = "verified"
)

type Outcome string

const (
	OutcomeRelease   Outcome = "release"
	OutcomeTerminate Outcome = "terminate"
)

type QuarantineCase struct {
	ID                   string     `json:"id"`
	AccessionCode        string     `json:"accession_code"`
	ScientificName       string     `json:"scientific_name"`
	OriginRegion         string     `json:"origin_region"`
	IntroductionPurpose  string     `json:"introduction_purpose"`
	QuarantineZone       string     `json:"quarantine_zone"`
	Status               CaseStatus `json:"status"`
	RiskLevel            RiskLevel  `json:"risk_level"`
	ObservationStartedAt *time.Time `json:"observation_started_at,omitempty"`
	ExpectedReleaseAt    *time.Time `json:"expected_release_at,omitempty"`
	Revision             int64      `json:"revision"`
	CreatedAt            time.Time  `json:"created_at"`
	ClosedAt             *time.Time `json:"closed_at,omitempty"`
}

func NewCase(id, accession, scientific, origin, purpose, zone string, now time.Time) (*QuarantineCase, error) {
	c := &QuarantineCase{ID: strings.TrimSpace(id), AccessionCode: strings.TrimSpace(accession), ScientificName: strings.TrimSpace(scientific), OriginRegion: strings.TrimSpace(origin), IntroductionPurpose: strings.TrimSpace(purpose), QuarantineZone: strings.TrimSpace(zone), Status: StatusDraft, RiskLevel: RiskUnknown, Revision: 1, CreatedAt: now.UTC()}
	if c.ID == "" {
		return nil, FieldError("id", "个案 ID 不能为空")
	}
	if c.AccessionCode == "" {
		return nil, FieldError("accession_code", "材料编号不能为空")
	}
	return c, nil
}

func (c QuarantineCase) MissingDraftFields() []string {
	var fields []string
	if strings.TrimSpace(c.ScientificName) == "" {
		fields = append(fields, "scientific_name")
	}
	if strings.TrimSpace(c.OriginRegion) == "" {
		fields = append(fields, "origin_region")
	}
	if strings.TrimSpace(c.IntroductionPurpose) == "" {
		fields = append(fields, "introduction_purpose")
	}
	if strings.TrimSpace(c.QuarantineZone) == "" {
		fields = append(fields, "quarantine_zone")
	}
	return fields
}

type AuditEvent struct {
	ID        int64           `json:"id"`
	CaseID    string          `json:"case_id"`
	Sequence  int64           `json:"sequence"`
	Type      string          `json:"type"`
	Actor     string          `json:"actor"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}
