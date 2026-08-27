package eligibilitysnapshotstoragefailure

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/repository"
)

func TestEligibilitySnapshotStorageFailureIsReported(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.Open(ctx, filepath.Join(t.TempDir(), "storage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	now := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	caseValue, err := domain.NewCase("case-storage", "ACC-STORAGE", "Rosa storage", "云南", "保育", "一区", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, *caseValue, "request-storage", "管理员"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DB().ExecContext(ctx,
		"INSERT INTO risk_baselines(case_id,version,payload,submitted_by,submitted_at) VALUES(?,?,?,?,?)",
		caseValue.ID, 1, `{"version":1}`, "管理员", now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DB().ExecContext(ctx, "DROP TABLE eligibility_snapshots"); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.Get(ctx, caseValue.ID); err == nil {
		t.Fatal("资格快照存储不可用时应返回错误")
	}
}
