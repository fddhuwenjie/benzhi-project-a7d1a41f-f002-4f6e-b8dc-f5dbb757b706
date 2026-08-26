package workflow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/policy"
	"quarantine-workbench/internal/repository"
)

type Role string

const (
	RoleManager  Role = "manager"
	RoleReviewer Role = "reviewer"
	RoleObserver Role = "observer"
)

type Meta struct {
	ExpectedRevision int64  `json:"expected_revision"`
	RequestID        string `json:"request_id"`
	Actor            string `json:"actor"`
	Role             Role   `json:"role"`
}

type Service struct {
	repo    *repository.Repository
	clock   policy.Clock
	trialMu sync.Mutex
	trials  map[string]RiskTrial
}

type RiskTrial struct {
	Token                        string            `json:"token"`
	Revision                     int64             `json:"revision"`
	Input                        SubmitRiskInput   `json:"input"`
	Result                       policy.RiskResult `json:"result"`
	SuggestedQuarantineDays      int               `json:"suggested_quarantine_days"`
	SuggestedObservationInterval int               `json:"suggested_observation_interval"`
	CreatedAt                    time.Time         `json:"created_at"`
}

func New(repo *repository.Repository, clock policy.Clock) *Service {
	return &Service{repo: repo, clock: clock, trials: make(map[string]RiskTrial)}
}

func (s *Service) Repository() *repository.Repository { return s.repo }

func require(meta Meta, roles ...Role) error {
	if strings.TrimSpace(meta.RequestID) == "" {
		return domain.FieldError("request_id", "request_id 不能为空")
	}
	if strings.TrimSpace(meta.Actor) == "" {
		return domain.FieldError("actor", "操作人不能为空")
	}
	for _, role := range roles {
		if meta.Role == role {
			return nil
		}
	}
	return domain.NewError(domain.CodeForbidden, "当前角色无权执行此操作")
}

func id(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + time.Now().UTC().Format("20060102150405.000000000")
	}
	return prefix + hex.EncodeToString(b[:])
}

func decodeResult[T any](r repository.MutationResult) (T, bool, error) {
	var value T
	err := json.Unmarshal(r.Response, &value)
	return value, r.Replayed, err
}

func ErrorCode(err error) domain.ErrorCode {
	var e *domain.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "internal_error"
}

type Envelope[T any] struct {
	Data     T    `json:"data"`
	Replayed bool `json:"replayed"`
}

func (s *Service) GetCase(ctx context.Context, caseID string) (*domain.CaseAggregate, error) {
	a, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	for i := range a.Deviations {
		if a.Deviations[i].Status == domain.DeviationOpen && a.Deviations[i].VerificationDueAt != nil {
			a.Deviations[i].Escalated = now.After(*a.Deviations[i].VerificationDueAt)
		}
	}
	if a.ReviewChecklist != nil {
		for i := range a.ReviewChecklist.Items {
			it := &a.ReviewChecklist.Items[i]
			if it.Status != "confirmed" && it.DueAt != nil && now.After(*it.DueAt) {
				it.Escalated = true
			}
		}
	}
	if a.Case.Status.Closed() && a.ArchiveIntegrity != nil {
		a.ArchiveIntegrity.Verified = policy.FingerprintArchive(*a, *a.ArchiveIntegrity) == a.ArchiveIntegrity.Fingerprint
	}
	return a, nil
}
func (s *Service) ListCases(ctx context.Context, status string) ([]domain.QuarantineCase, error) {
	return s.repo.List(ctx, status)
}
func (s *Service) ListCasesFiltered(ctx context.Context, f repository.CaseFilter) ([]domain.QuarantineCase, int, error) {
	return s.repo.ListFiltered(ctx, f)
}
func (s *Service) Timeline(ctx context.Context, caseID string) ([]domain.AuditEvent, error) {
	return s.repo.Timeline(ctx, caseID)
}
