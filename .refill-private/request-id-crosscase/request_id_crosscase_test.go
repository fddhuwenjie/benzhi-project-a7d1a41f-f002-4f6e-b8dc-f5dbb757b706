package requestidcrosscase

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/repository"
)

func TestRequestIDCannotReplayAnotherCase(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.Open(ctx, filepath.Join(t.TempDir(), "request-id.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	now := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)
	first, _ := domain.NewCase("case-one", "ACC-ONE", "Rosa one", "云南", "保育", "一区", now)
	second, _ := domain.NewCase("case-two", "ACC-TWO", "Rosa two", "四川", "研究", "二区", now)
	if _, err = repo.Create(ctx, *first, "shared-request", "甲"); err != nil {
		t.Fatal(err)
	}
	result, err := repo.Create(ctx, *second, "shared-request", "乙")
	if err == nil {
		t.Fatalf("不同个案复用 request_id 应返回冲突，实际静默重放：%s", result.Response)
	}
}
