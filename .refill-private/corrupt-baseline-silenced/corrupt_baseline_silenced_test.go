package corruptbaselinesilenced

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/repository"
)

func TestMalformedRiskBaselineFailsAggregateLoad(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.Open(ctx, filepath.Join(t.TempDir(), "corrupt.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
	c, _ := domain.NewCase("case-corrupt", "CORRUPT-1", "Rosa testacea", "云南", "研究", "一区", now)
	if _, err = repo.Create(ctx, *c, "create", "管理员"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.DB().ExecContext(ctx, `INSERT INTO risk_baselines(case_id,version,payload,submitted_by,submitted_at) VALUES(?,?,?,?,?)`, c.ID, 1, `{"version":`, "管理员", now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if aggregate, err := repo.Get(ctx, c.ID); err == nil {
		t.Fatalf("损坏的持久化风险基线不应被静默丢弃：%+v", aggregate.RiskBaselines)
	}
}
