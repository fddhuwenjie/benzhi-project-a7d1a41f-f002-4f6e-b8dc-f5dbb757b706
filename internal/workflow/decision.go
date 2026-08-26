package workflow

import (
	"context"
	"strings"

	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/policy"
)

func (s *Service) PreviewArchive(ctx context.Context, caseID string) (domain.ArchiveIntegrity, error) {
	a, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return domain.ArchiveIntegrity{}, err
	}
	events, err := s.repo.Timeline(ctx, caseID)
	if err != nil {
		return domain.ArchiveIntegrity{}, err
	}
	return policy.ArchiveIntegrity(*a, len(events)), nil
}

type EligibilityView struct {
	Result       domain.EligibilityResult   `json:"result"`
	CaseRevision int64                      `json:"case_revision"`
	Windows      []policy.ObservationWindow `json:"windows,omitempty"`
}

func (s *Service) PreviewEligibility(ctx context.Context, caseID string) (EligibilityView, error) {
	agg, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return EligibilityView{}, err
	}
	if agg.Risk == nil {
		return EligibilityView{}, domain.NewError(domain.CodeState, "尚无风险基线")
	}
	result := policy.EligibilityPolicy{Clock: s.clock}.Evaluate(agg.Case, *agg.Risk, agg.Observations, agg.Deviations)
	var windows []policy.ObservationWindow
	if agg.Case.ObservationStartedAt != nil {
		end := s.clock.Now()
		if agg.Case.ExpectedReleaseAt != nil {
			end = *agg.Case.ExpectedReleaseAt
		}
		windows = policy.BuildWindows(*agg.Case.ObservationStartedAt, end, agg.Risk.ObservationIntervalDays, agg.Observations, s.clock.Now())
	}
	return EligibilityView{Result: result, CaseRevision: agg.Case.Revision, Windows: windows}, nil
}

func (s *Service) ConfirmEligibility(ctx context.Context, caseID string, meta Meta) (Envelope[domain.CaseAggregate], error) {
	if err := require(meta, RoleManager, RoleReviewer); err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	now := s.clock.Now()
	result, err := s.repo.Mutate(context.Background(), caseID, meta.ExpectedRevision, meta.RequestID, "eligibility.confirmed", meta.Actor, now, func(a *domain.CaseAggregate) (any, error) {
		if a.Risk == nil {
			return nil, domain.NewError(domain.CodeState, "尚无风险基线")
		}
		eligibility := policy.EligibilityPolicy{Clock: s.clock}.Evaluate(a.Case, *a.Risk, a.Observations, a.Deviations)
		a.EligibilitySnapshots = append(a.EligibilitySnapshots, domain.EligibilitySnapshot{Revision: a.Case.Revision + 1, Result: eligibility, CreatedAt: now})
		if err := a.Case.MarkEligible(eligibility); err != nil {
			return nil, err
		}
		return a, nil
	})
	if err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	value, replayed, err := decodeResult[domain.CaseAggregate](result)
	return Envelope[domain.CaseAggregate]{Data: value, Replayed: replayed}, err
}

type DecideInput struct {
	Meta
	Outcome   domain.Outcome `json:"outcome"`
	Rationale string         `json:"rationale"`
}

func (s *Service) Decide(ctx context.Context, caseID string, input DecideInput) (Envelope[domain.CaseAggregate], error) {
	if err := require(input.Meta, RoleReviewer); err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	now := s.clock.Now()
	events, err := s.repo.Timeline(ctx, caseID)
	if err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	result, err := s.repo.Mutate(context.Background(), caseID, input.ExpectedRevision, input.RequestID, "case.archived", input.Actor, now, func(a *domain.CaseAggregate) (any, error) {
		if a.Risk == nil {
			return nil, domain.NewError(domain.CodeState, "尚无风险基线")
		}
		eligibility := policy.EligibilityPolicy{Clock: s.clock}.Evaluate(a.Case, *a.Risk, a.Observations, a.Deviations)
		integrity := policy.ArchiveIntegrity(*a, len(events)+1)
		if input.Outcome == domain.OutcomeRelease && integrity.Status != "complete" {
			e := domain.NewError(domain.CodeState, "归档证据包不完整，不能放行")
			e.Details = integrity
			return nil, e
		}
		if input.Outcome == domain.OutcomeTerminate && integrity.Status != "complete" && !strings.Contains(input.Rationale, "缺口") {
			return nil, domain.FieldError("rationale", "证据不完整时终止依据必须明确说明缺口")
		}
		if input.Outcome == domain.OutcomeRelease {
			if len(a.EligibilitySnapshots) == 0 || a.EligibilitySnapshots[len(a.EligibilitySnapshots)-1].Revision != a.Case.Revision || !a.EligibilitySnapshots[len(a.EligibilitySnapshots)-1].Result.Eligible {
				return nil, domain.NewError(domain.CodeConflict, "资格快照已过期，请重新核验")
			}
		}
		decision, err := domain.NewDecision(id("decision_"), caseID, eligibility, input.Outcome, input.Rationale, input.Actor, now)
		if err != nil {
			return nil, err
		}
		if err = a.Case.Close(input.Outcome, now); err != nil {
			return nil, err
		}
		a.Decision = decision
		fingerprintAggregate := *a
		fingerprintAggregate.Case.Revision++
		integrity.Fingerprint = policy.FingerprintArchive(fingerprintAggregate, integrity)
		integrity.Verified = true
		a.ArchiveIntegrity = &integrity
		return a, nil
	})
	if err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	value, replayed, err := decodeResult[domain.CaseAggregate](result)
	return Envelope[domain.CaseAggregate]{Data: value, Replayed: replayed}, err
}
