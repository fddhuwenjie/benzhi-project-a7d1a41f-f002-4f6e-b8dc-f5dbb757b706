package web

import (
	"net/http"
	"quarantine-workbench/internal/domain"
	"quarantine-workbench/internal/repository"
)

func (s *Server) HandleListCases(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("outcome") != "" || q.Get("risk_level") != "" || q.Get("accession_code") != "" || q.Get("scientific_name") != "" || q.Get("from") != "" || q.Get("to") != "" {
		cases, total, err := s.service.ListCasesFiltered(r.Context(), repository.CaseFilter{Status: q.Get("status"), Outcome: q.Get("outcome"), RiskLevel: q.Get("risk_level"), Accession: q.Get("accession_code"), ScientificName: q.Get("scientific_name"), From: q.Get("from"), To: q.Get("to")})
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"data": cases, "total": total})
		return
	}
	cases, err := s.service.ListCases(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": cases})
}

func (s *Server) HandleGetCase(w http.ResponseWriter, r *http.Request) {
	agg, err := s.service.GetCase(r.Context(), caseID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	timeline, err := s.service.Timeline(r.Context(), caseID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	eligibility, eligibilityErr := s.service.PreviewEligibility(r.Context(), caseID(r))
	response := map[string]any{"data": agg, "timeline": timeline}
	if eligibilityErr == nil {
		response["eligibility"] = eligibility
	}
	writeJSON(w, 200, response)
}

func (s *Server) HandleTimeline(w http.ResponseWriter, r *http.Request) {
	events, err := s.service.Timeline(r.Context(), caseID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": events})
}

func (s *Server) HandleEvidence(w http.ResponseWriter, r *http.Request) {
	obs, err := s.service.EvidenceLedger(r.Context(), caseID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if sample := r.URL.Query().Get("sample_reference"); sample != "" {
		var filtered []domain.ObservationEntry
		for _, o := range obs {
			if o.SampleReference == sample {
				filtered = append(filtered, o)
			}
		}
		obs = filtered
	}
	writeJSON(w, 200, map[string]any{"data": obs})
}

func (s *Server) HandlePreviewEligibility(w http.ResponseWriter, r *http.Request) {
	view, err := s.service.PreviewEligibility(r.Context(), caseID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": view})
}

func (s *Server) HandleObservationTrend(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := s.service.ObservationTrend(r.Context(), caseID(r), q.Get("from"), q.Get("to"), q.Get("recorded_by"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": result})
}
func (s *Server) HandleArchivePreview(w http.ResponseWriter, r *http.Request) {
	result, err := s.service.PreviewArchive(r.Context(), caseID(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": result})
}

func (s *Server) HandleStatistics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cases, _, err := s.service.ListCasesFiltered(r.Context(), repository.CaseFilter{Status: q.Get("status"), Outcome: q.Get("outcome"), RiskLevel: q.Get("risk_level"), From: q.Get("from"), To: q.Get("to")})
	if err != nil {
		writeError(w, err)
		return
	}
	counts := map[string]int{}
	totalDays := 0
	incomplete := 0
	closed := 0
	for _, c := range cases {
		counts[string(c.Status)]++
		if c.ClosedAt != nil {
			closed++
			totalDays += int(c.ClosedAt.Sub(c.CreatedAt).Hours() / 24)
			a, e := s.service.GetCase(r.Context(), c.ID)
			if e == nil && (a.ArchiveIntegrity == nil || a.ArchiveIntegrity.Status != "complete") {
				incomplete++
			}
		}
	}
	avg := float64(0)
	if closed > 0 {
		avg = float64(totalDays) / float64(closed)
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"total": len(cases), "counts": counts, "average_quarantine_days": avg, "incomplete_archives": incomplete}})
}
