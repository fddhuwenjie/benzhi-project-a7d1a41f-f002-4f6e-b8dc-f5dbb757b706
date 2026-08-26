package domain

import (
	"strings"
	"time"
)

type RiskAssessment struct {
	ID                      string       `json:"id"`
	CaseID                  string       `json:"case_id"`
	SpreadPathways          []string     `json:"spread_pathways"`
	PotentialHosts          []string     `json:"potential_hosts"`
	SourceConfidence        string       `json:"source_confidence"`
	QuarantineDays          int          `json:"quarantine_days"`
	ObservationIntervalDays int          `json:"observation_interval_days"`
	ReleaseConditions       []string     `json:"release_conditions"`
	CalculatedLevel         RiskLevel    `json:"calculated_level"`
	RiskReasons             []string     `json:"risk_reasons"`
	ReviewStatus            ReviewStatus `json:"review_status"`
	ReviewReason            string       `json:"review_reason,omitempty"`
	ReviewedBy              string       `json:"reviewed_by,omitempty"`
	ReviewedAt              *time.Time   `json:"reviewed_at,omitempty"`
	SubmittedAt             time.Time    `json:"submitted_at"`
}

func NewRiskAssessment(id, caseID string, pathways, hosts []string, confidence string, days, interval int, conditions []string, level RiskLevel, reasons []string, now time.Time) (*RiskAssessment, error) {
	r := &RiskAssessment{ID: id, CaseID: caseID, SpreadPathways: cleanList(pathways), PotentialHosts: cleanList(hosts), SourceConfidence: strings.TrimSpace(confidence), QuarantineDays: days, ObservationIntervalDays: interval, ReleaseConditions: cleanList(conditions), CalculatedLevel: level, RiskReasons: cleanList(reasons), ReviewStatus: ReviewPending, SubmittedAt: now.UTC()}
	if len(r.SpreadPathways) == 0 {
		return nil, FieldError("spread_pathways", "至少填写一种传播途径")
	}
	if len(r.PotentialHosts) == 0 {
		return nil, FieldError("potential_hosts", "至少填写一种潜在寄主")
	}
	if r.SourceConfidence != "high" && r.SourceConfidence != "medium" && r.SourceConfidence != "low" {
		return nil, FieldError("source_confidence", "来源可信度必须为 high、medium 或 low")
	}
	if r.QuarantineDays < 1 {
		return nil, FieldError("quarantine_days", "隔离期限必须大于零")
	}
	if r.ObservationIntervalDays < 1 || r.ObservationIntervalDays > r.QuarantineDays {
		return nil, FieldError("observation_interval_days", "观察频率必须在隔离期限范围内")
	}
	if len(r.ReleaseConditions) == 0 {
		return nil, FieldError("release_conditions", "至少填写一项放行条件")
	}
	return r, nil
}

func cleanList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
