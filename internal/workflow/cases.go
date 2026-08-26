package workflow

import (
	"context"
	"strings"

	"quarantine-workbench/internal/domain"
)

type UpdateCaseInput struct {
	Meta
	AccessionCode       string `json:"accession_code"`
	ScientificName      string `json:"scientific_name"`
	OriginRegion        string `json:"origin_region"`
	IntroductionPurpose string `json:"introduction_purpose"`
	QuarantineZone      string `json:"quarantine_zone"`
}

// UpdateCase updates draft/returned case data while retaining the optimistic revision.
func (s *Service) UpdateCase(ctx context.Context, caseID string, input UpdateCaseInput) (Envelope[domain.CaseAggregate], error) {
	if err := require(input.Meta, RoleManager); err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	now := s.clock.Now()
	accession := strings.ToUpper(strings.TrimSpace(input.AccessionCode))
	if current, e := s.repo.Get(ctx, caseID); e != nil {
		return Envelope[domain.CaseAggregate]{}, e
	} else if accession == "" {
		accession = strings.ToUpper(strings.TrimSpace(current.Case.AccessionCode))
	}
	if existing, e := s.repo.FindByAccession(ctx, accession); e != nil {
		return Envelope[domain.CaseAggregate]{}, e
	} else if existing != nil && existing.ID != caseID {
		er := domain.NewError(domain.CodeDuplicate, "材料编号已存在")
		er.Details = map[string]any{"case_id": existing.ID, "summary": existing}
		return Envelope[domain.CaseAggregate]{}, er
	}
	result, err := s.repo.Mutate(context.Background(), caseID, input.ExpectedRevision, input.RequestID, "case.revised", input.Actor, now, func(a *domain.CaseAggregate) (any, error) {
		if a.Case.Status != domain.StatusDraft && a.Case.Status != domain.StatusReturned {
			return nil, domain.StateError(a.Case.Status, "修订个案资料")
		}
		fields := map[string]string{"scientific_name": input.ScientificName, "origin_region": input.OriginRegion, "introduction_purpose": input.IntroductionPurpose, "quarantine_zone": input.QuarantineZone}
		for k, v := range fields {
			if strings.TrimSpace(v) == "" {
				return nil, domain.FieldError(k, "该字段不能为空")
			}
		}
		a.Case.AccessionCode, a.Case.ScientificName, a.Case.OriginRegion = accession, strings.TrimSpace(input.ScientificName), strings.TrimSpace(input.OriginRegion)
		a.Case.IntroductionPurpose, a.Case.QuarantineZone = strings.TrimSpace(input.IntroductionPurpose), strings.TrimSpace(input.QuarantineZone)
		return a, nil
	})
	if err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	value, replayed, err := decodeResult[domain.CaseAggregate](result)
	return Envelope[domain.CaseAggregate]{Data: value, Replayed: replayed}, err
}

type CreateCaseInput struct {
	RequestID           string `json:"request_id"`
	Actor               string `json:"actor"`
	Role                Role   `json:"role"`
	AccessionCode       string `json:"accession_code"`
	ScientificName      string `json:"scientific_name"`
	OriginRegion        string `json:"origin_region"`
	IntroductionPurpose string `json:"introduction_purpose"`
	QuarantineZone      string `json:"quarantine_zone"`
}

func (s *Service) CreateCase(ctx context.Context, input CreateCaseInput) (Envelope[domain.QuarantineCase], error) {
	meta := Meta{RequestID: input.RequestID, Actor: input.Actor, Role: input.Role}
	if err := require(meta, RoleManager); err != nil {
		return Envelope[domain.QuarantineCase]{}, err
	}
	now := s.clock.Now()
	input.AccessionCode = strings.ToUpper(strings.TrimSpace(input.AccessionCode))
	if existing, err := s.repo.FindByAccession(ctx, input.AccessionCode); err != nil {
		return Envelope[domain.QuarantineCase]{}, err
	} else if existing != nil {
		e := domain.NewError(domain.CodeDuplicate, "材料编号已存在")
		e.Details = map[string]any{"case_id": existing.ID, "status": existing.Status, "summary": existing}
		return Envelope[domain.QuarantineCase]{}, e
	}
	c, err := domain.NewCase(id("case_"), input.AccessionCode, input.ScientificName, input.OriginRegion, input.IntroductionPurpose, input.QuarantineZone, now)
	if err != nil {
		return Envelope[domain.QuarantineCase]{}, err
	}
	result, err := s.repo.Create(ctx, *c, input.RequestID, input.Actor)
	if err != nil {
		return Envelope[domain.QuarantineCase]{}, err
	}
	value, replayed, err := decodeResult[domain.QuarantineCase](result)
	return Envelope[domain.QuarantineCase]{Data: value, Replayed: replayed}, err
}
