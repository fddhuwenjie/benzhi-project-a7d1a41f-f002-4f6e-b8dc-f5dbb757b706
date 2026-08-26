package web

import (
	"net/http"
	"strings"

	"quarantine-workbench/internal/workflow"
)

type Server struct {
	service *workflow.Service
	mux     *http.ServeMux
}

func New(service *workflow.Service) *Server {
	s := &Server{service: service, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /", s.HandleWorkbench)
	s.mux.Handle("GET /assets/", http.FileServer(http.FS(assets)))
	s.mux.HandleFunc("GET /api/health", s.HandleHealth)
	s.mux.HandleFunc("GET /api/cases", s.HandleListCases)
	s.mux.HandleFunc("POST /api/cases", s.HandleCreateCase)
	s.mux.HandleFunc("GET /api/cases/{id}", s.HandleGetCase)
	s.mux.HandleFunc("PATCH /api/cases/{id}", s.HandleUpdateCase)
	s.mux.HandleFunc("GET /api/cases/{id}/timeline", s.HandleTimeline)
	s.mux.HandleFunc("GET /api/cases/{id}/evidence", s.HandleEvidence)
	s.mux.HandleFunc("POST /api/cases/{id}/risk", s.HandleSubmitRisk)
	s.mux.HandleFunc("POST /api/cases/{id}/risk/trial", s.HandleTrialRisk)
	s.mux.HandleFunc("POST /api/cases/{id}/review", s.HandleReviewRisk)
	s.mux.HandleFunc("POST /api/cases/{id}/start", s.HandleStartObservation)
	s.mux.HandleFunc("POST /api/cases/{id}/observations", s.HandleAddObservation)
	s.mux.HandleFunc("POST /api/cases/{id}/observations/{observation_id}/late-review", s.HandleReviewLateObservation)
	s.mux.HandleFunc("POST /api/cases/{id}/deviations", s.HandleOpenDeviation)
	s.mux.HandleFunc("POST /api/cases/{id}/deviations/{deviation_id}/verify", s.HandleVerifyDeviation)
	s.mux.HandleFunc("GET /api/cases/{id}/eligibility", s.HandlePreviewEligibility)
	s.mux.HandleFunc("POST /api/cases/{id}/eligibility-check", s.HandleConfirmEligibility)
	s.mux.HandleFunc("POST /api/cases/{id}/decision", s.HandleDecision)
	s.mux.HandleFunc("GET /api/cases/{id}/archive-preview", s.HandleArchivePreview)
	s.mux.HandleFunc("GET /api/cases/{id}/trends", s.HandleObservationTrend)
	s.mux.HandleFunc("GET /api/statistics", s.HandleStatistics)
	return s
}

func (s *Server) Handler() http.Handler { return requestLog(s.mux) }

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) HandleWorkbench(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "页面资源不可用", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func caseID(r *http.Request) string { return strings.TrimSpace(r.PathValue("id")) }
