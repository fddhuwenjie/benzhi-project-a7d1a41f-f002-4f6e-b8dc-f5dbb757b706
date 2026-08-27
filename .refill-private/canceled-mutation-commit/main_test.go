package canceledmutationcommit

import (
	"context"
	"errors"
	"testing"
	"time"

	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/policy"
	"quarantine-workbench/internal/repository"
	"quarantine-workbench/internal/workflow"
)

func TestCanceledMutationDoesNotCommit(t *testing.T) {
	repo, err := repository.Open(context.Background(), t.TempDir()+"/canceled-mutation.db")
	if err != nil {
		t.Fatalf("打开仓储: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	c, err := domain.NewCase("case_canceled", "CTX-2026-001", "Rosa testacea", "测试地区", "取消传播验证", "隔离区 A", now)
	if err != nil {
		t.Fatalf("构造个案: %v", err)
	}
	if _, err := repo.Create(context.Background(), *c, "request-create-fixture", "测试管理员"); err != nil {
		t.Fatalf("创建测试个案: %v", err)
	}
	if _, err := repo.Mutate(context.Background(), c.ID, c.Revision, "request-observing-fixture", "observation.started", "测试管理员", now, func(a *domain.CaseAggregate) (any, error) {
		a.Case.Status = domain.StatusObserving
		return a, nil
	}); err != nil {
		t.Fatalf("准备观察中个案: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := workflow.New(repo, policy.FixedClock{Time: now.Add(time.Minute)})
	_, err = service.OpenDeviation(ctx, c.ID, workflow.OpenDeviationInput{
		Meta:              workflow.Meta{ExpectedRevision: c.Revision + 1, RequestID: "request-canceled-mutation", Actor: "测试观察员", Role: workflow.RoleObserver},
		Severity:          "medium",
		Scope:             "隔离区 A 单株",
		Finding:           "叶片出现异常斑点",
		ContainmentAction: "隔离该植株并采样",
	})
	mutationErr := err
	stored, err := repo.Get(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("重新读取个案: %v", err)
	}
	var audits, requests int
	if err := repo.DB().QueryRow(`SELECT COUNT(*) FROM audit_events WHERE case_id=?`, c.ID).Scan(&audits); err != nil {
		t.Fatalf("查询审计记录: %v", err)
	}
	if err := repo.DB().QueryRow(`SELECT COUNT(*) FROM request_results WHERE request_id=?`, "request-canceled-mutation").Scan(&requests); err != nil {
		t.Fatalf("查询幂等结果: %v", err)
	}
	if !errors.Is(mutationErr, context.Canceled) || stored.Case.Revision != c.Revision+1 || stored.Case.Status != domain.StatusObserving || len(stored.Deviations) != 0 || audits != 2 || requests != 0 {
		t.Fatalf("TestCanceledMutationDoesNotCommit: err=%v revision=%d status=%q deviations=%d audits=%d requests=%d", mutationErr, stored.Case.Revision, stored.Case.Status, len(stored.Deviations), audits, requests)
	}
}
