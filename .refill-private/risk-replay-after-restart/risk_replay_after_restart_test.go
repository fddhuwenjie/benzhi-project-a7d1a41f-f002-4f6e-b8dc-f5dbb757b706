package riskreplayafterrestart

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

func TestCommittedRiskRequestReplaysAfterServiceRestart(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.Open(ctx, filepath.Join(t.TempDir(), "restart.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	clock := fixedClock{now: time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)}
	service := workflow.New(repo, clock)
	created, err := service.CreateCase(ctx, workflow.CreateCaseInput{RequestID: "create", Actor: "管理员", Role: workflow.RoleManager, AccessionCode: "RESTART-1", ScientificName: "Rosa testacea", OriginRegion: "云南", IntroductionPurpose: "研究", QuarantineZone: "一区"})
	if err != nil {
		t.Fatal(err)
	}
	input := workflow.SubmitRiskInput{Meta: workflow.Meta{ExpectedRevision: created.Data.Revision, RequestID: "risk-submit", Actor: "管理员", Role: workflow.RoleManager}, SpreadPathways: []string{"种子"}, PotentialHosts: []string{"蔷薇科"}, SourceConfidence: "high", QuarantineDays: 14, ObservationIntervalDays: 7, ReleaseConditions: []string{"无病虫害"}}
	trial, err := service.TrialRisk(ctx, created.Data.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	input.TrialToken = trial.Token
	if _, err = service.SubmitRisk(ctx, created.Data.ID, input); err != nil {
		t.Fatal(err)
	}

	restarted := workflow.New(repo, clock)
	replayed, err := restarted.SubmitRisk(ctx, created.Data.ID, input)
	if err != nil || !replayed.Replayed {
		t.Fatalf("已提交请求应由持久化幂等结果重放，result=%+v err=%v", replayed, err)
	}
}
