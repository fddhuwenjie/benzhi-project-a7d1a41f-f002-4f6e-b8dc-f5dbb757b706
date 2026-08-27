package eligibilitypreviewstale

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/policy"
	"quarantine-workbench/internal/repository"
	"quarantine-workbench/internal/workflow"
)

func TestEligibilityPreviewRefreshesAfterObservation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	repo, err := repository.Open(ctx, filepath.Join(t.TempDir(), "preview.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	createdAt := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	c, err := domain.NewCase("case-preview", "CACHE-001", "Rosa testacea", "云南", "保育", "一区", createdAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, *c, "create-preview", "管理员"); err != nil {
		t.Fatal(err)
	}
	setup, err := repo.Mutate(ctx, c.ID, 1, "setup-observing", "case.prepared", "管理员", createdAt, func(a *domain.CaseAggregate) (any, error) {
		start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
		expected := start.AddDate(0, 0, 1)
		a.Case.Status = domain.StatusObserving
		a.Case.RiskLevel = domain.RiskLow
		a.Case.ObservationStartedAt = &start
		a.Case.ExpectedReleaseAt = &expected
		a.Risk = &domain.RiskAssessment{
			ID: "risk-preview", CaseID: c.ID,
			SpreadPathways: []string{"种子"}, PotentialHosts: []string{"蔷薇科"},
			SourceConfidence: "high", QuarantineDays: 1, ObservationIntervalDays: 1,
			ReleaseConditions: []string{"无病虫征象"}, CalculatedLevel: domain.RiskLow,
			ReviewStatus: domain.ReviewApproved, ReviewReason: "已确认", ReviewedBy: "审核员",
			SubmittedAt: createdAt,
		}
		return a, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(setup.Response) == 0 {
		t.Fatal("准备观察状态未返回聚合")
	}

	service := workflow.New(repo, policy.FixedClock{Time: now})
	before, err := service.PreviewEligibility(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.CaseRevision != 2 || before.Result.Checks[1].Passed {
		t.Fatalf("前置资格预览不符合测试条件：revision=%d frequency=%v", before.CaseRevision, before.Result.Checks[1].Passed)
	}

	added, err := service.AddObservation(ctx, c.ID, workflow.AddObservationInput{
		Meta:       workflow.Meta{ExpectedRevision: 2, RequestID: "add-observation", Actor: "观察员", Role: workflow.RoleObserver},
		ObservedOn: "2026-08-02", GrowthCondition: "长势稳定", PestSigns: "无",
		ReproductionSigns: "无", SampleReference: "SAMPLE-001", Notes: "期满观察",
	})
	if err != nil {
		t.Fatal(err)
	}
	if added.Data.Case.Revision != 3 {
		t.Fatalf("观察写入未提交：revision=%d", added.Data.Case.Revision)
	}

	after, err := service.PreviewEligibility(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.CaseRevision != 3 || len(after.Result.MissingEvidence) != 0 {
		t.Fatalf("观察提交后仍返回旧资格预览：revision=%d missing=%v", after.CaseRevision, after.Result.MissingEvidence)
	}
}
