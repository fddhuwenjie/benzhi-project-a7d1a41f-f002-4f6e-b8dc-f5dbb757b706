package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/policy"
)

func riskDiff(prev *domain.RiskAssessment, cur *domain.RiskAssessment) []domain.FieldChange {
	if prev == nil {
		return nil
	}
	var out []domain.FieldChange
	cmp := func(field string, a, b any) {
		if fmt.Sprint(a) != fmt.Sprint(b) {
			out = append(out, domain.FieldChange{Field: field, Kind: "modified", Before: a, After: b})
		}
	}
	cmp("spread_pathways", prev.SpreadPathways, cur.SpreadPathways)
	cmp("potential_hosts", prev.PotentialHosts, cur.PotentialHosts)
	cmp("source_confidence", prev.SourceConfidence, cur.SourceConfidence)
	cmp("quarantine_days", prev.QuarantineDays, cur.QuarantineDays)
	cmp("observation_interval_days", prev.ObservationIntervalDays, cur.ObservationIntervalDays)
	cmp("release_conditions", prev.ReleaseConditions, cur.ReleaseConditions)
	return out
}

type SubmitRiskInput struct {
	Meta
	SpreadPathways          []string `json:"spread_pathways"`
	PotentialHosts          []string `json:"potential_hosts"`
	SourceConfidence        string   `json:"source_confidence"`
	QuarantineDays          int      `json:"quarantine_days"`
	ObservationIntervalDays int      `json:"observation_interval_days"`
	ReleaseConditions       []string `json:"release_conditions"`
	TrialToken              string   `json:"trial_token,omitempty"`
}

func (s *Service) TrialRisk(ctx context.Context, caseID string, input SubmitRiskInput) (RiskTrial, error) {
	if err := require(input.Meta, RoleManager); err != nil {
		return RiskTrial{}, err
	}
	a, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return RiskTrial{}, err
	}
	if a.Case.Status != domain.StatusDraft && a.Case.Status != domain.StatusReturned {
		return RiskTrial{}, domain.StateError(a.Case.Status, "试算风险")
	}
	if a.Case.Revision != input.ExpectedRevision {
		return RiskTrial{}, domain.NewError(domain.CodeConflict, "修订号冲突")
	}
	ri := policy.RiskInput{SpreadPathways: input.SpreadPathways, PotentialHosts: input.PotentialHosts, SourceConfidence: input.SourceConfidence}
	if e := policy.ValidateRiskInput(ri); e != nil {
		return RiskTrial{}, e
	}
	if input.QuarantineDays < 1 {
		return RiskTrial{}, domain.FieldError("quarantine_days", "隔离期限必须大于零")
	}
	if input.ObservationIntervalDays < 1 {
		return RiskTrial{}, domain.FieldError("observation_interval_days", "观察间隔必须大于零")
	}
	res := policy.CalculateRisk(ri)
	days, interval := 14, 7
	if res.Level == domain.RiskMedium {
		days, interval = 30, 5
	}
	if res.Level == domain.RiskHigh {
		days, interval = 60, 3
	}
	token := id("trial_")
	trial := RiskTrial{Token: token, Revision: a.Case.Revision, Input: input, Result: res, SuggestedQuarantineDays: days, SuggestedObservationInterval: interval, CreatedAt: s.clock.Now()}
	s.trialMu.Lock()
	// 同一个案在同一 revision 上可能由不同页面或操作人分别试算，按 token 独立保存。
	// 清理严格过期（revision 已落后于当前）的试算，避免缓存无限增长并使旧 token 提前稳定失效。
	if s.trials[caseID] == nil {
		s.trials[caseID] = make(map[string]RiskTrial)
	}
	for tk, existing := range s.trials[caseID] {
		if existing.Revision != a.Case.Revision {
			delete(s.trials[caseID], tk)
		}
	}
	s.trials[caseID][token] = trial
	s.trialMu.Unlock()
	return trial, nil
}

func sameTrial(a, b SubmitRiskInput) bool {
	a.Meta = Meta{}
	b.Meta = Meta{}
	a.TrialToken = ""
	b.TrialToken = ""
	return fmt.Sprint(a) == fmt.Sprint(b)
}

// trialFor 在持有锁的状态下按 token 查找回一个案的有效试算。
func (s *Service) trialForLocked(caseID, token string) (RiskTrial, bool) {
	if token == "" {
		return RiskTrial{}, false
	}
	bucket, ok := s.trials[caseID]
	if !ok {
		return RiskTrial{}, false
	}
	trial, ok := bucket[token]
	return trial, ok
}

// clearTrials 在风险基线提交成功后清空同一个案的全部试算缓存。
// revision 已由仓储推进，其余 token 在下次提交时会在 trialForLocked 找不到而稳定失效。
func (s *Service) clearTrialsLocked(caseID string) {
	delete(s.trials, caseID)
}

func (s *Service) SubmitRisk(ctx context.Context, caseID string, input SubmitRiskInput) (Envelope[domain.CaseAggregate], error) {
	if err := require(input.Meta, RoleManager); err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	s.trialMu.Lock()
	trial, trialOK := s.trialForLocked(caseID, strings.TrimSpace(input.TrialToken))
	s.trialMu.Unlock()
	// 试算缓存命中时先做独立校验：revision 不匹配或输入已变更则直接拒绝。
	// 缓存未命中时不在此处提前失败——可能是成功提交后缓存已清空的幂等重放，
	// 交给仓储 Mutate 判定：重放则返回首次结果，revision 已推进则稳定冲突。
	if trialOK {
		if trial.Revision != input.ExpectedRevision || !sameTrial(trial.Input, input) {
			return Envelope[domain.CaseAggregate]{}, domain.NewError(domain.CodeConflict, "风险试算已过期或参数已变更，请重新试算")
		}
	}
	now := s.clock.Now()
	result, err := s.repo.Mutate(ctx, caseID, input.ExpectedRevision, input.RequestID, "risk.submitted", input.Actor, now, func(a *domain.CaseAggregate) (any, error) {
		// Mutate 仅在非幂等重放时调用此闭包。若此时试算已不在缓存中，
		// 说明 revision 仍未推进但 token 已失效，按试算过期处理。
		if !trialOK {
			return nil, domain.NewError(domain.CodeConflict, "风险试算已过期或参数已变更，请重新试算")
		}
		if missing := a.Case.MissingDraftFields(); len(missing) > 0 {
			e := domain.NewError(domain.CodeValidation, "个案资料尚不完整")
			e.Details = map[string]any{"missing_fields": missing}
			return nil, e
		}
		if a.Case.Status.Closed() {
			return nil, domain.StateError(a.Case.Status, "提交风险审查")
		}
		riskResult := policy.CalculateRisk(policy.RiskInput{SpreadPathways: input.SpreadPathways, PotentialHosts: input.PotentialHosts, SourceConfidence: input.SourceConfidence})
		risk, err := domain.NewRiskAssessment(id("risk_"), caseID, input.SpreadPathways, input.PotentialHosts, input.SourceConfidence, input.QuarantineDays, input.ObservationIntervalDays, input.ReleaseConditions, riskResult.Level, riskResult.Reasons, now)
		if err != nil {
			return nil, err
		}
		if err = a.Case.SubmitRisk(risk.CalculatedLevel); err != nil {
			return nil, err
		}
		version := len(a.RiskBaselines) + 1
		baseline := domain.RiskBaseline{Version: version, Risk: *risk, SubmittedBy: input.Actor, SubmittedAt: now}
		baseline.Trial = &domain.RiskTrialSummary{Token: trial.Token, Level: trial.Result.Level, Reasons: trial.Result.Reasons, SuggestedQuarantineDays: trial.SuggestedQuarantineDays, SuggestedObservationInterval: trial.SuggestedObservationInterval, CreatedAt: trial.CreatedAt}
		if a.Risk != nil {
			baseline.Diff = riskDiff(a.Risk, risk)
		}
		a.RiskBaselines = append(a.RiskBaselines, baseline)
		a.Risk = risk
		due := now.UTC().Add(48 * time.Hour)
		labels := []string{"隔离期限", "观察频率", "放行条件"}
		items := make([]domain.ReviewItem, 0, len(labels))
		for i, label := range labels {
			items = append(items, domain.ReviewItem{ID: fmt.Sprintf("item_%d", i+1), Label: label, Status: "pending", DueAt: &due, Assignee: "reviewer"})
		}
		a.ReviewChecklist = &domain.ReviewChecklist{BaselineVersion: version, Items: items}
		return a, nil
	})
	if err != nil {
		// 缓存未命中且仓储判定为修订号冲突（revision 已被其他操作推进）时，
		// 对调用方统一表现为试算过期，避免暴露内部修订号细节。
		if !trialOK && ErrorCode(err) == domain.CodeConflict {
			return Envelope[domain.CaseAggregate]{}, domain.NewError(domain.CodeConflict, "风险试算已过期或参数已变更，请重新试算")
		}
		return Envelope[domain.CaseAggregate]{}, err
	}
	// 提交成功（或幂等重放）后，revision 已推进或结果已固化，清空该案全部试算缓存，使其余 token 稳定失效。
	s.trialMu.Lock()
	s.clearTrialsLocked(caseID)
	s.trialMu.Unlock()
	value, replayed, err := decodeResult[domain.CaseAggregate](result)
	return Envelope[domain.CaseAggregate]{Data: value, Replayed: replayed}, err
}

type ReviewInput struct {
	Meta
	Approved bool                `json:"approved"`
	Reason   string              `json:"reason"`
	Items    []domain.ReviewItem `json:"items,omitempty"`
}

func (s *Service) ReviewRisk(ctx context.Context, caseID string, input ReviewInput) (Envelope[domain.CaseAggregate], error) {
	if err := require(input.Meta, RoleReviewer); err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	if strings.TrimSpace(input.Reason) == "" {
		return Envelope[domain.CaseAggregate]{}, domain.FieldError("reason", "审核理由不能为空")
	}
	now := s.clock.Now()
	result, err := s.repo.Mutate(ctx, caseID, input.ExpectedRevision, input.RequestID, "risk.reviewed", input.Actor, now, func(a *domain.CaseAggregate) (any, error) {
		if a.Risk == nil {
			return nil, domain.NewError(domain.CodeState, "个案尚未提交风险基线")
		}
		if len(input.Items) > 0 {
			for i := range input.Items {
				it := &input.Items[i]
				if input.Approved && it.Status != "confirmed" {
					return nil, domain.NewError(domain.CodeState, "存在未确认的审查项")
				}
				if it.Status == "needs_supplement" && strings.TrimSpace(it.Reason) == "" {
					return nil, domain.FieldError("items", "需补充项必须填写原因")
				}
				if it.Status == "confirmed" {
					t := now.UTC()
					it.ConfirmedAt = &t
					it.Escalated = false
				}
			}
			a.ReviewChecklist = &domain.ReviewChecklist{BaselineVersion: len(a.RiskBaselines), Items: input.Items, ReviewedBy: input.Actor, Approved: input.Approved}
			t := now.UTC()
			a.ReviewChecklist.ReviewedAt = &t
		} else if input.Approved && a.ReviewChecklist != nil {
			t := now.UTC()
			for i := range a.ReviewChecklist.Items {
				a.ReviewChecklist.Items[i].Status = "confirmed"
				a.ReviewChecklist.Items[i].ConfirmedAt = &t
				a.ReviewChecklist.Items[i].Escalated = false
			}
			a.ReviewChecklist.Approved = true
			a.ReviewChecklist.ReviewedBy = input.Actor
			a.ReviewChecklist.ReviewedAt = &t
		}
		if err := a.Case.Review(input.Approved); err != nil {
			return nil, err
		}
		a.Risk.ReviewReason = strings.TrimSpace(input.Reason)
		a.Risk.ReviewedBy = input.Actor
		t := now.UTC()
		a.Risk.ReviewedAt = &t
		if input.Approved {
			a.Risk.ReviewStatus = domain.ReviewApproved
		} else {
			a.Risk.ReviewStatus = domain.ReviewReturned
		}
		return a, nil
	})
	if err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	value, replayed, err := decodeResult[domain.CaseAggregate](result)
	return Envelope[domain.CaseAggregate]{Data: value, Replayed: replayed}, err
}

func (s *Service) StartObservation(ctx context.Context, caseID string, meta Meta) (Envelope[domain.CaseAggregate], error) {
	if err := require(meta, RoleManager, RoleReviewer); err != nil {
		return Envelope[domain.CaseAggregate]{}, err
	}
	now := s.clock.Now()
	result, err := s.repo.Mutate(ctx, caseID, meta.ExpectedRevision, meta.RequestID, "observation.started", meta.Actor, now, func(a *domain.CaseAggregate) (any, error) {
		if a.Risk == nil || a.Risk.ReviewStatus != domain.ReviewApproved {
			return nil, domain.NewError(domain.CodeState, "风险方案尚未获批")
		}
		if a.ReviewChecklist != nil {
			for _, it := range a.ReviewChecklist.Items {
				if it.Status != "confirmed" || (it.DueAt != nil && now.After(*it.DueAt) && it.Escalated) {
					return nil, domain.NewError(domain.CodeState, "审查清单尚未全部确认或存在逾期升级")
				}
			}
		}
		if err := a.Case.StartObservation(a.Risk.QuarantineDays, now); err != nil {
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
