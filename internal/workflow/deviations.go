package workflow

import (
	"context"
	"strings"

	"quarantine-workbench/internal/domain"
)

type OpenDeviationInput struct {
	Meta
	Severity          string `json:"severity"`
	Scope             string `json:"scope"`
	Finding           string `json:"finding"`
	ContainmentAction string `json:"containment_action"`
}

type openDeviationPayload struct {
	Severity          string `json:"severity"`
	Scope             string `json:"scope"`
	Finding           string `json:"finding"`
	ContainmentAction string `json:"containment_action"`
}

func (s *Service) OpenDeviation(ctx context.Context, caseID string, input OpenDeviationInput) (Envelope[domain.CaseAggregate], error) {
	if err := require(input.Meta, RoleObserver, RoleReviewer); err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	now := s.clock.Now()
	result, err := s.repo.Mutate(fingerprintContext(ctx, openDeviationPayload{Severity: strings.TrimSpace(input.Severity), Scope: strings.TrimSpace(input.Scope), Finding: strings.TrimSpace(input.Finding), ContainmentAction: strings.TrimSpace(input.ContainmentAction)}), caseID, input.ExpectedRevision, input.RequestID, "deviation.opened", input.Actor, now, func(a *domain.CaseAggregate) (any, error) {
		if err := a.Case.Restrict(); err != nil {
			return nil, err
		}
		d, err := domain.NewDeviation(id("dev_"), caseID, input.Severity, input.Scope, input.Finding, input.ContainmentAction, now)
		if err != nil {
			return nil, err
		}
		d.SetDeadline(now)
		d.AssignedRole = "observer"
		if d.Severity == "high" {
			d.AssignedRole = "reviewer"
		}
		d.Rounds = []domain.DeviationRound{{Round: 1, Action: d.ContainmentAction}}
		a.Deviations = append(a.Deviations, *d)
		return a, nil
	})
	if err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	value, replayed, err := decodeResult[domain.CaseAggregate](result)
	return Envelope[domain.CaseAggregate]{Data: value, Replayed: replayed}, err
}

type VerifyDeviationInput struct {
	Meta
	VerificationNote     string `json:"verification_note"`
	VerificationEvidence string `json:"verification_evidence,omitempty"`
	Result               string `json:"result,omitempty"`
	NewContainmentAction string `json:"new_containment_action,omitempty"`
}

type verifyDeviationPayload struct {
	DeviationID          string `json:"deviation_id"`
	VerificationNote     string `json:"verification_note"`
	VerificationEvidence string `json:"verification_evidence,omitempty"`
	Result               string `json:"result,omitempty"`
	NewContainmentAction string `json:"new_containment_action,omitempty"`
}

func (s *Service) VerifyDeviation(ctx context.Context, caseID, deviationID string, input VerifyDeviationInput) (Envelope[domain.CaseAggregate], error) {
	if err := require(input.Meta, RoleReviewer, RoleObserver); err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	now := s.clock.Now()
	result, err := s.repo.Mutate(fingerprintContext(ctx, verifyDeviationPayload{DeviationID: deviationID, VerificationNote: strings.TrimSpace(input.VerificationNote), VerificationEvidence: strings.TrimSpace(input.VerificationEvidence), Result: strings.TrimSpace(input.Result), NewContainmentAction: strings.TrimSpace(input.NewContainmentAction)}), caseID, input.ExpectedRevision, input.RequestID, "deviation.verified", input.Actor, now, func(a *domain.CaseAggregate) (any, error) {
		d, err := a.FindDeviation(deviationID)
		if err != nil {
			return nil, err
		}
		resultValue := input.Result
		if resultValue == "" {
			resultValue = "passed"
		}
		if resultValue != "passed" && resultValue != "failed" {
			return nil, domain.FieldError("result", "验证结果必须为 passed 或 failed")
		}
		if len(d.Rounds) == 0 {
			d.Rounds = []domain.DeviationRound{{Round: 1, Action: d.ContainmentAction}}
		}
		round := &d.Rounds[len(d.Rounds)-1]
		round.Result = resultValue
		round.Evidence = input.VerificationEvidence
		round.VerifiedBy = input.Actor
		t := now.UTC()
		round.VerifiedAt = &t
		if resultValue == "failed" {
			if input.NewContainmentAction == "" {
				return nil, domain.FieldError("new_containment_action", "验证不通过时必须填写新的限制措施")
			}
			d.VerificationNote = input.VerificationNote
			d.ContainmentAction = input.NewContainmentAction
			d.Rounds = append(d.Rounds, domain.DeviationRound{Round: len(d.Rounds) + 1, Action: input.NewContainmentAction})
			d.SetDeadline(now)
			return a, nil
		}
		if err = d.Verify(input.VerificationNote, now); err != nil {
			return nil, err
		}
		d.VerificationEvidence = input.VerificationEvidence
		if d.VerificationDueAt != nil && now.After(*d.VerificationDueAt) {
			d.Escalated = true
		}
		other := false
		for _, candidate := range a.Deviations {
			if candidate.ID != deviationID && candidate.Status == domain.DeviationOpen {
				other = true
			}
		}
		if !other {
			if err = a.Case.Resume(false); err != nil {
				return nil, err
			}
		}
		return a, nil
	})
	if err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	value, replayed, err := decodeResult[domain.CaseAggregate](result)
	return Envelope[domain.CaseAggregate]{Data: value, Replayed: replayed}, err
}
