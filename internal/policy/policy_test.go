package policy

import (
	"testing"
	"time"

	"quarantine-workbench/internal/domain"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestCalculateRiskIsExplainable(t *testing.T) {
	result := CalculateRisk(RiskInput{
		SpreadPathways:   []string{"种子", "土壤", "昆虫"},
		PotentialHosts:   []string{"蔷薇", "苹果", "梨", "桃", "李", "杏"},
		SourceConfidence: "low",
	})
	if result.Level != domain.RiskHigh || result.Score != 9 {
		t.Fatalf("高风险计算错误：%+v", result)
	}
	if len(result.Reasons) != 3 {
		t.Fatalf("应逐项给出解释：%v", result.Reasons)
	}
}

func TestContinuityAndEligibility(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	now := start.AddDate(0, 0, 14)
	observations := []domain.ObservationEntry{
		observation(start, "S-001"),
		observation(start.AddDate(0, 0, 7), "S-002"),
		observation(start.AddDate(0, 0, 14), "S-003"),
	}
	continuity := CheckContinuity(start, now, 7, observations)
	if !continuity.Continuous || continuity.RequiredCount != 3 || continuity.ActualCount != 3 {
		t.Fatalf("连续性计算错误：%+v", continuity)
	}
	c := domain.QuarantineCase{Status: domain.StatusObserving, ObservationStartedAt: &start}
	risk := domain.RiskAssessment{
		QuarantineDays:          14,
		ObservationIntervalDays: 7,
		ReleaseConditions:       []string{"无病虫害"},
		ReviewStatus:            domain.ReviewApproved,
		CalculatedLevel:         domain.RiskMedium,
	}
	result := (EligibilityPolicy{Clock: fixedClock{now: now}}).Evaluate(c, risk, observations, nil)
	if !result.Eligible || result.RemainingDays != 0 {
		t.Fatalf("完整证据应通过资格检查：%+v", result)
	}
	deviations := []domain.Deviation{{Status: domain.DeviationOpen}}
	result = (EligibilityPolicy{Clock: fixedClock{now: now}}).Evaluate(c, risk, observations, deviations)
	if result.Eligible {
		t.Fatal("存在未关闭偏差时不应通过")
	}
}

func TestEligibilityReportsEvidenceGaps(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	c := domain.QuarantineCase{Status: domain.StatusObserving, ObservationStartedAt: &start}
	risk := domain.RiskAssessment{QuarantineDays: 1, ObservationIntervalDays: 1, ReleaseConditions: []string{"无病虫害"}, ReviewStatus: domain.ReviewApproved}
	entries := []domain.ObservationEntry{observation(start, "")}
	result := (EligibilityPolicy{Clock: fixedClock{now: start.AddDate(0, 0, 1)}}).Evaluate(c, risk, entries, nil)
	if result.Eligible || len(result.MissingEvidence) != 1 {
		t.Fatalf("样本编号缺口未报告：%+v", result)
	}
}

func observation(date time.Time, sample string) domain.ObservationEntry {
	return domain.ObservationEntry{
		ObservedOn:        date,
		GrowthCondition:   "长势稳定",
		PestSigns:         "无",
		ReproductionSigns: "无",
		SampleReference:   sample,
	}
}
