package risktrialoverwrite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"quarantine-workbench/internal/policy"
	"quarantine-workbench/internal/repository"
	"quarantine-workbench/internal/workflow"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

var _ policy.Clock = fixedClock{}

func TestSecondTrialDoesNotInvalidateFirstToken(t *testing.T) {
	ctx := context.Background()
	repo, err := repository.Open(ctx, filepath.Join(t.TempDir(), "trials.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := workflow.New(repo, fixedClock{now: time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)})
	created, err := service.CreateCase(ctx, workflow.CreateCaseInput{RequestID: "create", Actor: "管理员", Role: workflow.RoleManager, AccessionCode: "TRIAL-1", ScientificName: "Rosa testacea", OriginRegion: "云南", IntroductionPurpose: "研究", QuarantineZone: "一区"})
	if err != nil {
		t.Fatal(err)
	}
	base := workflow.SubmitRiskInput{Meta: workflow.Meta{ExpectedRevision: created.Data.Revision, RequestID: "trial-a", Actor: "甲", Role: workflow.RoleManager}, SpreadPathways: []string{"种子"}, PotentialHosts: []string{"蔷薇科"}, SourceConfidence: "high", QuarantineDays: 14, ObservationIntervalDays: 7, ReleaseConditions: []string{"无病虫害"}}
	first, err := service.TrialRisk(ctx, created.Data.ID, base)
	if err != nil {
		t.Fatal(err)
	}
	other := base
	other.Meta.RequestID = "trial-b"
	other.SpreadPathways = []string{"土壤"}
	if _, err = service.TrialRisk(ctx, created.Data.ID, other); err != nil {
		t.Fatal(err)
	}
	base.Meta.RequestID = "submit-a"
	base.TrialToken = first.Token
	if _, err = service.SubmitRisk(ctx, created.Data.ID, base); err != nil {
		t.Fatalf("仍匹配原始输入和 revision 的首个试算 token 应可提交：%v", err)
	}
}
