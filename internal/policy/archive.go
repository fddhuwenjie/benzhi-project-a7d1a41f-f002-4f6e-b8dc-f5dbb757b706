package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"quarantine-workbench/internal/domain"
)

func ArchiveIntegrity(a domain.CaseAggregate, auditCount int) domain.ArchiveIntegrity {
	r := domain.ArchiveIntegrity{Status: "complete", AuditCount: auditCount, RiskBaselines: len(a.RiskBaselines), Observations: len(a.Observations), Deviations: len(a.Deviations), EligibilitySnapshots: len(a.EligibilitySnapshots)}
	if a.ReviewChecklist != nil {
		r.ReviewItems = len(a.ReviewChecklist.Items)
	}
	missing := func(v string) { r.Status = "incomplete"; r.Missing = append(r.Missing, v) }
	if r.RiskBaselines == 0 {
		missing("risk_baseline")
	}
	if a.ReviewChecklist == nil || !a.ReviewChecklist.Approved {
		missing("review_conclusion")
	}
	if r.Observations == 0 {
		missing("observations")
	}
	for _, o := range a.Observations {
		if o.SampleReference == "" {
			missing("observation_sample:" + o.ID)
		}
	}
	if r.EligibilitySnapshots == 0 {
		missing("eligibility_snapshot")
	}
	if auditCount < 1 {
		missing("audit_sequence")
	}
	return r
}

func FingerprintArchive(a domain.CaseAggregate, integrity domain.ArchiveIntegrity) string {
	integrity.Fingerprint = ""
	integrity.Verified = false
	raw, _ := json.Marshal(struct {
		Case         domain.QuarantineCase        `json:"case"`
		Risk         []domain.RiskBaseline        `json:"risk"`
		Review       *domain.ReviewChecklist      `json:"review"`
		Observations []domain.ObservationEntry    `json:"observations"`
		Deviations   []domain.Deviation           `json:"deviations"`
		Eligibility  []domain.EligibilitySnapshot `json:"eligibility"`
		Integrity    domain.ArchiveIntegrity      `json:"integrity"`
	}{a.Case, a.RiskBaselines, a.ReviewChecklist, a.Observations, a.Deviations, a.EligibilitySnapshots, integrity})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
