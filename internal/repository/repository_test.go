package repository

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"quarantine-workbench/internal/domain"
)

func TestCreateMutateIdempotencyAndTimeline(t *testing.T) {
	ctx := context.Background()
	repo, err := Open(ctx, filepath.Join(t.TempDir(), "repository.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	c, err := domain.NewCase("case-1", "ACC-001", "Rosa testacea", "云南", "保育", "一区", now)
	if err != nil {
		t.Fatal(err)
	}
	created, err := repo.Create(ctx, *c, "request-create", "管理员")
	if err != nil || created.Replayed {
		t.Fatalf("首次创建失败：result=%+v err=%v", created, err)
	}
	replayed, err := repo.Create(ctx, *c, "request-create", "管理员")
	if err != nil || !replayed.Replayed || string(replayed.Response) != string(created.Response) {
		t.Fatalf("创建幂等重放失败：result=%+v err=%v", replayed, err)
	}
	mutation, err := repo.Mutate(ctx, c.ID, 1, "request-risk", "risk.submitted", "管理员", now, func(aggregate *domain.CaseAggregate) (any, error) {
		if err := aggregate.Case.SubmitRisk(domain.RiskLow); err != nil {
			return nil, err
		}
		return aggregate, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var aggregate domain.CaseAggregate
	if err := json.Unmarshal(mutation.Response, &aggregate); err != nil {
		t.Fatal(err)
	}
	if aggregate.Case.Revision != 2 || aggregate.Case.Status != domain.StatusPendingReview {
		t.Fatalf("事务变更结果错误：%+v", aggregate.Case)
	}
	if _, err := repo.Mutate(ctx, c.ID, 1, "request-stale", "bad", "管理员", now, func(aggregate *domain.CaseAggregate) (any, error) {
		return aggregate, nil
	}); ErrorCode(err) != domain.CodeConflict {
		t.Fatalf("过期 revision 应冲突，得到 %v", err)
	}
	events, err := repo.Timeline(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("审计时间线错误：%+v", events)
	}
}

func TestAccessionCodeIsUnique(t *testing.T) {
	ctx := context.Background()
	repo, err := Open(ctx, filepath.Join(t.TempDir(), "unique.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	first, _ := domain.NewCase("case-1", "ACC-UNIQUE", "Rosa one", "云南", "保育", "一区", now)
	second, _ := domain.NewCase("case-2", "ACC-UNIQUE", "Rosa two", "四川", "研究", "二区", now)
	if _, err := repo.Create(ctx, *first, "create-1", "管理员"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, *second, "create-2", "管理员"); ErrorCode(err) != domain.CodeDuplicate {
		t.Fatalf("重复材料编号应返回稳定错误码，得到 %v", err)
	}
}

func ErrorCode(err error) domain.ErrorCode {
	if typed, ok := err.(*domain.Error); ok {
		return typed.Code
	}
	return ""
}
