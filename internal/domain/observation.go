package domain

import (
	"strings"
	"time"
)

type ObservationEntry struct {
	ID                string     `json:"id"`
	CaseID            string     `json:"case_id"`
	ObservedOn        time.Time  `json:"observed_on"`
	GrowthCondition   string     `json:"growth_condition"`
	PestSigns         string     `json:"pest_signs"`
	ReproductionSigns string     `json:"reproduction_signs"`
	SampleReference   string     `json:"sample_reference"`
	Notes             string     `json:"notes"`
	RecordedBy        string     `json:"recorded_by"`
	RecordedAt        time.Time  `json:"recorded_at"`
	WindowDueOn       *time.Time `json:"window_due_on,omitempty"`
	Late              bool       `json:"late"`
	LateReason        string     `json:"late_reason,omitempty"`
	LateStatus        string     `json:"late_status,omitempty"`
	LateReviewedBy    string     `json:"late_reviewed_by,omitempty"`
	LateReviewedAt    *time.Time `json:"late_reviewed_at,omitempty"`
}

func NewObservation(id, caseID string, observedOn time.Time, growth, pest, reproduction, sample, notes, actor string, now time.Time) (*ObservationEntry, error) {
	o := &ObservationEntry{ID: id, CaseID: caseID, ObservedOn: dateOnly(observedOn), GrowthCondition: strings.TrimSpace(growth), PestSigns: strings.TrimSpace(pest), ReproductionSigns: strings.TrimSpace(reproduction), SampleReference: strings.TrimSpace(sample), Notes: strings.TrimSpace(notes), RecordedBy: strings.TrimSpace(actor), RecordedAt: now.UTC()}
	if o.ObservedOn.IsZero() {
		return nil, FieldError("observed_on", "观察日期不能为空")
	}
	if o.ObservedOn.After(dateOnly(now)) {
		return nil, FieldError("observed_on", "观察日期不能晚于今天")
	}
	if o.GrowthCondition == "" {
		return nil, FieldError("growth_condition", "长势记录不能为空")
	}
	if o.PestSigns == "" {
		return nil, FieldError("pest_signs", "病虫征象不能为空，无异常时请填写无")
	}
	if o.ReproductionSigns == "" {
		return nil, FieldError("reproduction_signs", "繁殖迹象不能为空，无异常时请填写无")
	}
	if o.RecordedBy == "" {
		return nil, FieldError("actor", "记录人不能为空")
	}
	if o.SampleReference == "" {
		return nil, FieldError("sample_reference", "样本编号不能为空")
	}
	return o, nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
