package corruptcasetime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/repository"
)

func TestMalformedStoredTimestampFailsCaseLoad(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.Open(ctx, filepath.Join(t.TempDir(), "timestamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
	c, _ := domain.NewCase("case-time", "TIME-1", "Rosa testacea", "云南", "研究", "一区", now)
	if _, err = repo.Create(ctx, *c, "create", "管理员"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.DB().ExecContext(ctx, `UPDATE cases SET created_at=? WHERE id=?`, "not-a-timestamp", c.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.Get(ctx, c.ID)
	if err == nil {
		t.Fatalf("损坏的持久化时间戳不应变成零时间继续跨层传播：%s", loaded.Case.CreatedAt)
	}
}
