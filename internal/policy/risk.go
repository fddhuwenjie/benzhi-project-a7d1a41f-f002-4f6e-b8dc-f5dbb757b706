package policy

import (
	"fmt"
	"strings"

	"quarantine-workbench/internal/domain"
)

type RiskInput struct {
	SpreadPathways   []string
	PotentialHosts   []string
	SourceConfidence string
}

type RiskResult struct {
	Level   domain.RiskLevel `json:"level"`
	Score   int              `json:"score"`
	Reasons []string         `json:"reasons"`
}

func ValidateRiskInput(input RiskInput) *domain.Error {
	if len(input.SpreadPathways) == 0 {
		return domain.FieldError("spread_pathways", "至少填写一种传播途径")
	}
	if len(input.PotentialHosts) == 0 {
		return domain.FieldError("potential_hosts", "至少填写一种潜在寄主")
	}
	check := func(field string, vals []string) *domain.Error {
		seen := map[string]bool{}
		for _, v := range vals {
			n := strings.ToLower(strings.TrimSpace(v))
			if n == "" {
				return domain.FieldError(field, "不能包含空值")
			}
			if seen[n] {
				return domain.FieldError(field, "不允许重复项")
			}
			seen[n] = true
		}
		return nil
	}
	if e := check("spread_pathways", input.SpreadPathways); e != nil {
		return e
	}
	if e := check("potential_hosts", input.PotentialHosts); e != nil {
		return e
	}
	if strings.TrimSpace(input.SourceConfidence) != "high" && strings.TrimSpace(input.SourceConfidence) != "medium" && strings.TrimSpace(input.SourceConfidence) != "low" {
		return domain.FieldError("source_confidence", "来源可信度必须为 high、medium 或 low")
	}
	return nil
}

func CalculateRisk(input RiskInput) RiskResult {
	score := 0
	reasons := make([]string, 0, 4)
	pathways := uniqueNonEmpty(input.SpreadPathways)
	hosts := uniqueNonEmpty(input.PotentialHosts)
	switch len(pathways) {
	case 0:
		reasons = append(reasons, "未提供传播途径，风险资料不完整")
	case 1:
		score += 1
		reasons = append(reasons, "存在 1 种已识别传播途径")
	case 2:
		score += 2
		reasons = append(reasons, "存在 2 种传播途径，扩散机会增加")
	default:
		score += 3
		reasons = append(reasons, fmt.Sprintf("存在 %d 种传播途径，扩散机会较高", len(pathways)))
	}
	switch {
	case len(hosts) == 0:
		reasons = append(reasons, "未提供潜在寄主，风险资料不完整")
	case len(hosts) <= 2:
		score += 1
		reasons = append(reasons, "潜在寄主范围有限")
	case len(hosts) <= 5:
		score += 2
		reasons = append(reasons, "潜在寄主范围中等")
	default:
		score += 3
		reasons = append(reasons, "潜在寄主范围广泛")
	}
	switch strings.ToLower(strings.TrimSpace(input.SourceConfidence)) {
	case "high":
		reasons = append(reasons, "来源资料可信度高，未增加风险分")
	case "medium":
		score += 1
		reasons = append(reasons, "来源资料可信度中等")
	case "low":
		score += 3
		reasons = append(reasons, "来源资料可信度低，需加强隔离证据")
	default:
		score += 3
		reasons = append(reasons, "来源可信度未知，按低可信度计分")
	}
	level := domain.RiskLow
	if score >= 7 {
		level = domain.RiskHigh
	} else if score >= 4 {
		level = domain.RiskMedium
	}
	return RiskResult{Level: level, Score: score, Reasons: reasons}
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}
