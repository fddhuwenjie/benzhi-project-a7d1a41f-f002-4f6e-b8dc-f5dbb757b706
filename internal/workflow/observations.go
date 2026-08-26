package workflow

import (
	"context"
	"sort"
	"strings"
	"time"

	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/policy"
)

func (s *Service) ObservationTrend(ctx context.Context, caseID, from, to, actor string) (policy.ObservationTrend, error) {
	a, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return policy.ObservationTrend{}, err
	}
	var f, t *time.Time
	if from != "" {
		v, e := time.Parse("2006-01-02", from)
		if e != nil {
			return policy.ObservationTrend{}, domain.FieldError("from", "日期格式必须为 YYYY-MM-DD")
		}
		f = &v
	}
	if to != "" {
		v, e := time.Parse("2006-01-02", to)
		if e != nil {
			return policy.ObservationTrend{}, domain.FieldError("to", "日期格式必须为 YYYY-MM-DD")
		}
		t = &v
	}
	var obs []domain.ObservationEntry
	for _, o := range a.Observations {
		if policy.InRange(o.ObservedOn, f, t) && (actor == "" || o.RecordedBy == actor) {
			obs = append(obs, o)
		}
	}
	return policy.CalculateTrend(obs), nil
}

func (s *Service) EvidenceLedger(ctx context.Context, caseID string) ([]domain.ObservationEntry, error) {
	a, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return nil, err
	}
	obs := append([]domain.ObservationEntry(nil), a.Observations...)
	sort.Slice(obs, func(i, j int) bool { return obs[i].ObservedOn.After(obs[j].ObservedOn) })
	return obs, nil
}

type AddObservationInput struct {
	Meta
	ObservedOn        string `json:"observed_on"`
	GrowthCondition   string `json:"growth_condition"`
	PestSigns         string `json:"pest_signs"`
	ReproductionSigns string `json:"reproduction_signs"`
	SampleReference   string `json:"sample_reference"`
	Notes             string `json:"notes"`
	WindowDueOn       string `json:"window_due_on,omitempty"`
	LateReason        string `json:"late_reason,omitempty"`
}

func (s *Service) AddObservation(ctx context.Context, caseID string, input AddObservationInput) (Envelope[domain.CaseAggregate], error) {
	if err := require(input.Meta, RoleObserver, RoleManager); err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	observed, err := time.Parse("2006-01-02", input.ObservedOn)
	if err != nil {
		return Envelope[domain.CaseAggregate]{}, domain.FieldError("observed_on", "观察日期格式必须为 YYYY-MM-DD")
	}
	now := s.clock.Now()
	result, err := s.repo.Mutate(ctx, caseID, input.ExpectedRevision, input.RequestID, "observation.recorded", input.Actor, now, func(a *domain.CaseAggregate) (any, error) {
		if err := a.Case.AddObservation(); err != nil {
			return nil, err
		}
		if a.Case.ObservationStartedAt != nil && observed.Before(*a.Case.ObservationStartedAt) {
			return nil, domain.FieldError("observed_on", "观察日期不能早于隔离启动日期")
		}
		o, err := domain.NewObservation(id("obs_"), caseID, observed, input.GrowthCondition, input.PestSigns, input.ReproductionSigns, input.SampleReference, input.Notes, input.Actor, now)
		if err != nil {
			return nil, err
		}
		if input.WindowDueOn != "" {
			due, e := time.Parse("2006-01-02", input.WindowDueOn)
			if e != nil {
				return nil, domain.FieldError("window_due_on", "窗口日期格式必须为 YYYY-MM-DD")
			}
			if a.Risk == nil {
				return nil, domain.NewError(domain.CodeState, "尚无风险基线")
			}
			end := due.AddDate(0, 0, a.Risk.ObservationIntervalDays-1)
			o.WindowDueOn = &due
			if observed.After(end) {
				o.Late = true
				o.LateStatus = "pending"
				o.LateReason = strings.TrimSpace(input.LateReason)
				if o.LateReason == "" {
					return nil, domain.FieldError("late_reason", "迟到补录必须填写原因")
				}
			}
		}
		for _, existing := range a.Observations {
			if existing.ObservedOn.Equal(o.ObservedOn) {
				return nil, domain.FieldError("observed_on", "该日期已登记观察")
			}
			if strings.TrimSpace(input.SampleReference) != "" && strings.TrimSpace(existing.SampleReference) == strings.TrimSpace(input.SampleReference) {
				e := domain.NewError(domain.CodeDuplicate, "样本编号已存在")
				e.Details = map[string]any{"sample_reference": existing.SampleReference, "observed_on": existing.ObservedOn.Format("2006-01-02"), "recorded_by": existing.RecordedBy}
				return nil, e
			}
		}
		a.Observations = append(a.Observations, *o)
		return a, nil
	})
	if err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	value, replayed, err := decodeResult[domain.CaseAggregate](result)
	return Envelope[domain.CaseAggregate]{Data: value, Replayed: replayed}, err
}

type ReviewLateObservationInput struct {
	Meta
	Approved bool   `json:"approved"`
	Reason   string `json:"reason"`
}

func (s *Service) ReviewLateObservation(ctx context.Context, caseID, observationID string, input ReviewLateObservationInput) (Envelope[domain.CaseAggregate], error) {
	if err := require(input.Meta, RoleReviewer); err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	now := s.clock.Now()
	result, err := s.repo.Mutate(ctx, caseID, input.ExpectedRevision, input.RequestID, "observation.late_reviewed", input.Actor, now, func(a *domain.CaseAggregate) (any, error) {
		if a.Case.Status.Closed() {
			return nil, domain.StateError(a.Case.Status, "审批补录")
		}
		for i := range a.Observations {
			if a.Observations[i].ID == observationID {
				if !a.Observations[i].Late || a.Observations[i].LateStatus != "pending" {
					return nil, domain.NewError(domain.CodeState, "观察记录不处于待审批补录状态")
				}
				a.Observations[i].LateReviewedBy = input.Actor
				a.Observations[i].LateStatus = "rejected"
				if input.Approved {
					a.Observations[i].LateStatus = "approved"
				}
				t := now.UTC()
				a.Observations[i].LateReviewedAt = &t
				return a, nil
			}
		}
		return nil, domain.NewError(domain.CodeNotFound, "未找到观察记录")
	})
	if err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	value, replayed, err := decodeResult[domain.CaseAggregate](result)
	return Envelope[domain.CaseAggregate]{Data: value, Replayed: replayed}, err
}
