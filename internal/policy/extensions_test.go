package policy

import (
	"testing"
	"time"

	"quarantine-workbench/internal/domain"
)

func TestValidateRiskInputRejectsDuplicate(t *testing.T) {
	err := ValidateRiskInput(RiskInput{SpreadPathways: []string{"种子", " 种子 "}, PotentialHosts: []string{"蔷薇科"}, SourceConfidence: "high"})
	if err == nil || err.Field != "spread_pathways" {
		t.Fatalf("应拒绝重复传播途径，实际 %#v", err)
	}
}

func TestCalculateTrendReturnsMandatoryAlert(t *testing.T) {
	base := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	records := []domain.ObservationEntry{{ID: "o1", ObservedOn: base, GrowthCondition: "衰弱", PestSigns: "叶斑", ReproductionSigns: "无", SampleReference: "s1"}, {ID: "o2", ObservedOn: base.AddDate(0, 0, 1), GrowthCondition: "衰弱", PestSigns: "叶斑", ReproductionSigns: "有花粉", SampleReference: "s2"}}
	result := CalculateTrend(records)
	mandatory := 0
	for _, alert := range result.Alerts {
		if alert.MustOpenDeviation {
			mandatory++
		}
	}
	if mandatory < 2 {
		t.Fatalf("预期连续异常与繁殖两个强制预警，实际 %#v", result.Alerts)
	}
}
