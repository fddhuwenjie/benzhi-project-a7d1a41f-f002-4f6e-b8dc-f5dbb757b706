package createretrypreflight

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"quarantine-workbench/internal/repository"
	"quarantine-workbench/internal/workflow"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestCreateCaseRetryReturnsOriginalEnvelope(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.Open(ctx, filepath.Join(t.TempDir(), "create-retry.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := workflow.New(repo, fixedClock{now: time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)})
	input := workflow.CreateCaseInput{RequestID: "create-once", Actor: "管理员", Role: workflow.RoleManager, AccessionCode: "RETRY-1", ScientificName: "Rosa testacea", OriginRegion: "云南", IntroductionPurpose: "研究", QuarantineZone: "一区"}
	first, err := service.CreateCase(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateCase(ctx, input)
	if err != nil || !second.Replayed || second.Data.ID != first.Data.ID {
		t.Fatalf("相同创建请求重试应重放首次结果，first=%+v second=%+v err=%v", first, second, err)
	}
}
