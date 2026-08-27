package filteredstatementreuse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"quarantine-workbench/internal/policy"
	"quarantine-workbench/internal/repository"
	"quarantine-workbench/internal/web"
	"quarantine-workbench/internal/workflow"
)

func TestRepeatedFilteredStatisticsKeepsStatementUsable(t *testing.T) {
	repo, err := repository.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open repository: %v", err)
	}
	defer repo.Close()

	clock := policy.FixedClock{Time: time.Date(2026, time.August, 26, 8, 0, 0, 0, time.UTC)}
	handler := web.New(workflow.New(repo, clock)).Handler()

	request := func() *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/statistics?status=draft", nil)
		handler.ServeHTTP(recorder, req)
		return recorder
	}

	first := request()
	if first.Code != http.StatusOK {
		t.Fatalf("first filtered statistics request returned %d: %s", first.Code, first.Body.String())
	}
	second := request()
	if second.Code != http.StatusOK {
		t.Fatalf("repeated filtered statistics request returned %d: %s", second.Code, second.Body.String())
	}
}
