package web

import (
	"net/http"

	"quarantine-workbench/internal/workflow"
)

func (s *Server) HandleCreateCase(w http.ResponseWriter, r *http.Request) {
	var input workflow.CreateCaseInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.CreateCase(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 201, result)
}
func (s *Server) HandleUpdateCase(w http.ResponseWriter, r *http.Request) {
	var input workflow.UpdateCaseInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.UpdateCase(r.Context(), caseID(r), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) HandleTrialRisk(w http.ResponseWriter, r *http.Request) {
	var input workflow.SubmitRiskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.TrialRisk(r.Context(), caseID(r), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": result})
}
func (s *Server) HandleSubmitRisk(w http.ResponseWriter, r *http.Request) {
	var input workflow.SubmitRiskInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.SubmitRisk(r.Context(), caseID(r), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) HandleReviewRisk(w http.ResponseWriter, r *http.Request) {
	var input workflow.ReviewInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.ReviewRisk(r.Context(), caseID(r), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) HandleStartObservation(w http.ResponseWriter, r *http.Request) {
	var input workflow.Meta
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.StartObservation(r.Context(), caseID(r), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) HandleAddObservation(w http.ResponseWriter, r *http.Request) {
	var input workflow.AddObservationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.AddObservation(r.Context(), caseID(r), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) HandleReviewLateObservation(w http.ResponseWriter, r *http.Request) {
	var input workflow.ReviewLateObservationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.ReviewLateObservation(r.Context(), caseID(r), r.PathValue("observation_id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) HandleOpenDeviation(w http.ResponseWriter, r *http.Request) {
	var input workflow.OpenDeviationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.OpenDeviation(r.Context(), caseID(r), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) HandleVerifyDeviation(w http.ResponseWriter, r *http.Request) {
	var input workflow.VerifyDeviationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.VerifyDeviation(r.Context(), caseID(r), r.PathValue("deviation_id"), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) HandleConfirmEligibility(w http.ResponseWriter, r *http.Request) {
	var input workflow.Meta
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.ConfirmEligibility(r.Context(), caseID(r), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Server) HandleDecision(w http.ResponseWriter, r *http.Request) {
	var input workflow.DecideInput
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := s.service.Decide(r.Context(), caseID(r), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
