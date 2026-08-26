package policy

import (
	"fmt"
	"strings"

	"quarantine-workbench/internal/domain"
)

type EligibilityPolicy struct{ Clock Clock }

func (p EligibilityPolicy) Evaluate(c domain.QuarantineCase, risk domain.RiskAssessment, observations []domain.ObservationEntry, deviations []domain.Deviation) domain.EligibilityResult {
	now := p.Clock.Now().UTC()
	result := domain.EligibilityResult{Eligible: true, CheckedAt: now, Checks: make([]domain.EligibilityCheck, 0, 5)}
	add := func(key, label string, passed bool, detail string) {
		result.Checks = append(result.Checks, domain.EligibilityCheck{Key: key, Label: label, Passed: passed, Detail: detail})
		if !passed {
			result.Eligible = false
		}
	}
	started := c.ObservationStartedAt != nil
	if !started {
		add("duration", "最低隔离天数", false, "隔离观察尚未启动")
	} else {
		elapsed := int(day(now).Sub(day(*c.ObservationStartedAt)).Hours() / 24)
		remaining := risk.QuarantineDays - elapsed
		if remaining < 0 {
			remaining = 0
		}
		result.RemainingDays = remaining
		add("duration", "最低隔离天数", elapsed >= risk.QuarantineDays, fmt.Sprintf("已观察 %d 天，要求至少 %d 天", elapsed, risk.QuarantineDays))
	}
	through := now
	if c.ExpectedReleaseAt != nil && through.After(*c.ExpectedReleaseAt) {
		through = *c.ExpectedReleaseAt
	}
	counted := make([]domain.ObservationEntry, 0, len(observations))
	for _, o := range observations {
		if !o.Late || o.LateStatus == "approved" {
			counted = append(counted, o)
		}
	}
	continuity := ContinuityResult{}
	if started {
		continuity = CheckContinuity(*c.ObservationStartedAt, through, risk.ObservationIntervalDays, counted)
	}
	add("frequency", "观察连续性", continuity.Continuous && continuity.ActualCount >= continuity.RequiredCount, fmt.Sprintf("已登记 %d 次，当前应有 %d 次；缺失时段 %d 个", continuity.ActualCount, continuity.RequiredCount, len(continuity.MissingDates)))
	missingEvidence := evidenceGaps(observations)
	result.MissingEvidence = missingEvidence
	add("evidence", "观察证据完整", len(observations) > 0 && len(missingEvidence) == 0, evidenceDetail(missingEvidence))
	open := 0
	for _, deviation := range deviations {
		if deviation.Status == domain.DeviationOpen {
			open++
		}
	}
	add("deviations", "偏差全部关闭", open == 0, fmt.Sprintf("未关闭偏差 %d 项", open))
	riskOK := len(risk.ReleaseConditions) > 0 && risk.ReviewStatus == domain.ReviewApproved
	add("risk_conditions", "风险条件已确认", riskOK, fmt.Sprintf("风险等级 %s，放行条件 %d 项，审查状态 %s", risk.CalculatedLevel, len(risk.ReleaseConditions), risk.ReviewStatus))
	for i, condition := range risk.ReleaseConditions {
		ids := []string{}
		for _, o := range counted {
			if strings.TrimSpace(o.SampleReference) != "" {
				ids = append(ids, o.ID)
			}
		}
		passed := len(ids) > 0
		status := "passed"
		if !passed {
			status = "missing_evidence"
			result.Eligible = false
		}
		result.Checks = append(result.Checks, domain.EligibilityCheck{Key: fmt.Sprintf("release_condition_%d", i+1), Label: condition, Passed: passed, Status: status, EvidenceIDs: ids, Detail: map[bool]string{true: "已绑定观察样本证据", false: "缺少带样本编号的观察证据"}[passed]})
	}
	return result
}

func evidenceGaps(observations []domain.ObservationEntry) []string {
	var gaps []string
	seenSamples := map[string]string{}
	for _, observation := range observations {
		prefix := observation.ObservedOn.Format("2006-01-02")
		if strings.TrimSpace(observation.GrowthCondition) == "" {
			gaps = append(gaps, prefix+" 缺少长势")
		}
		if strings.TrimSpace(observation.PestSigns) == "" {
			gaps = append(gaps, prefix+" 缺少病虫征象")
		}
		if strings.TrimSpace(observation.ReproductionSigns) == "" {
			gaps = append(gaps, prefix+" 缺少繁殖迹象")
		}
		if strings.TrimSpace(observation.SampleReference) == "" {
			gaps = append(gaps, prefix+" 缺少样本编号")
		} else if prev, ok := seenSamples[strings.TrimSpace(observation.SampleReference)]; ok {
			gaps = append(gaps, prefix+" 样本编号冲突（原记录 "+prev+"）")
		} else {
			seenSamples[strings.TrimSpace(observation.SampleReference)] = prefix
		}
	}
	return gaps
}

func evidenceDetail(gaps []string) string {
	if len(gaps) == 0 {
		return "所有观察均包含长势、病虫征象、繁殖迹象和样本编号"
	}
	return fmt.Sprintf("共有 %d 个证据缺口", len(gaps))
}
