package domain

import (
	"testing"
	"time"
)

func TestQuarantineCaseLifecycleGuards(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	c, err := NewCase("case-1", "A-001", "Rosa testacea", "云南", "保育研究", "一区", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.StartObservation(30, now); err == nil {
		t.Fatal("未获批个案不应启动观察")
	}
	if err := c.SubmitRisk(RiskMedium); err != nil {
		t.Fatal(err)
	}
	if err := c.Review(true); err != nil {
		t.Fatal(err)
	}
	if err := c.StartObservation(30, now); err != nil {
		t.Fatal(err)
	}
	if c.ExpectedReleaseAt == nil || c.ExpectedReleaseAt.Format("2006-01-02") != "2026-08-31" {
		t.Fatalf("期望放行日期错误：%v", c.ExpectedReleaseAt)
	}
	if err := c.Restrict(); err != nil {
		t.Fatal(err)
	}
	if err := c.MarkEligible(EligibilityResult{Eligible: true}); err == nil {
		t.Fatal("受限期间不应申请结论")
	}
	if err := c.Resume(false); err != nil {
		t.Fatal(err)
	}
	if err := c.MarkEligible(EligibilityResult{Eligible: true}); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(OutcomeRelease, now.AddDate(0, 0, 30)); err != nil {
		t.Fatal(err)
	}
	if !c.Status.Closed() || c.ClosedAt == nil {
		t.Fatal("放行后应为关闭状态")
	}
	if err := c.AddObservation(); err == nil {
		t.Fatal("关闭后业务事实应只读")
	}
}

func TestClosedCaseDataIsReadOnly(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	c, err := NewCase("c", "A", "Plant", "区域", "用途", "隔离区", now)
	if err != nil {
		t.Fatal(err)
	}
	c.Status = StatusReleased
	if err = c.AddObservation(); err == nil {
		t.Fatal("关闭个案不应允许登记观察")
	}
	if err = c.SubmitRisk(RiskLow); err == nil {
		t.Fatal("关闭个案不应允许提交风险")
	}
}

func TestDomainObjectValidation(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	if _, err := NewRiskAssessment("risk", "case", nil, []string{"蔷薇科"}, "high", 30, 7, []string{"无病虫害"}, RiskLow, nil, now); err == nil {
		t.Fatal("缺少传播途径应被拒绝")
	}
	if _, err := NewObservation("obs", "case", now.AddDate(0, 0, 1), "正常", "无", "无", "S-1", "", "观察员", now); err == nil {
		t.Fatal("未来日期应被拒绝")
	}
	deviation, err := NewDeviation("dev", "case", "high", "隔离圃一区", "叶面斑点", "隔离并采样", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := deviation.Verify("", now); err == nil {
		t.Fatal("空验证说明应被拒绝")
	}
}
