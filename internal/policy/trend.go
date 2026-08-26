package policy

import (
	"sort"
	"strings"
	"time"

	"quarantine-workbench/internal/domain"
)

type TrendAlert struct {
	Level             string   `json:"level"`
	Code              string   `json:"code"`
	Reason            string   `json:"reason"`
	Dates             []string `json:"dates,omitempty"`
	MustOpenDeviation bool     `json:"must_open_deviation"`
}
type ObservationTrend struct {
	First                   *domain.ObservationEntry `json:"first,omitempty"`
	Last                    *domain.ObservationEntry `json:"last,omitempty"`
	Count                   int                      `json:"count"`
	GrowthChanges           int                      `json:"growth_changes"`
	PestChanges             int                      `json:"pest_changes"`
	ReproductionChanges     int                      `json:"reproduction_changes"`
	ConsecutiveAbnormalDays int                      `json:"consecutive_abnormal_days"`
	LatestSampleReference   string                   `json:"latest_sample_reference,omitempty"`
	EvidenceGaps            []string                 `json:"evidence_gaps,omitempty"`
	Alerts                  []TrendAlert             `json:"alerts,omitempty"`
}

func CalculateTrend(records []domain.ObservationEntry) ObservationTrend {
	in := append([]domain.ObservationEntry(nil), records...)
	sort.Slice(in, func(i, j int) bool { return in[i].ObservedOn.Before(in[j].ObservedOn) })
	out := ObservationTrend{}
	valid := make([]domain.ObservationEntry, 0, len(in))
	for _, o := range in {
		if strings.TrimSpace(o.SampleReference) == "" || strings.TrimSpace(o.GrowthCondition) == "" || strings.TrimSpace(o.PestSigns) == "" || strings.TrimSpace(o.ReproductionSigns) == "" {
			out.EvidenceGaps = append(out.EvidenceGaps, o.ID)
			continue
		}
		valid = append(valid, o)
	}
	out.Count = len(valid)
	if len(valid) == 0 {
		return out
	}
	out.First = &valid[0]
	out.Last = &valid[len(valid)-1]
	out.LatestSampleReference = valid[len(valid)-1].SampleReference
	abnormal := 0
	pestDates := map[string][]string{}
	reproductionDates := []string{}
	for i, o := range valid {
		if i > 0 {
			if o.GrowthCondition != valid[i-1].GrowthCondition {
				out.GrowthChanges++
			}
			if o.PestSigns != valid[i-1].PestSigns {
				out.PestChanges++
			}
			if o.ReproductionSigns != valid[i-1].ReproductionSigns {
				out.ReproductionChanges++
			}
		}
		date := o.ObservedOn.Format("2006-01-02")
		if abnormalText(o.GrowthCondition) || abnormalText(o.PestSigns) {
			abnormal++
		} else {
			abnormal = 0
		}
		if abnormal > out.ConsecutiveAbnormalDays {
			out.ConsecutiveAbnormalDays = abnormal
		}
		if abnormalText(o.PestSigns) {
			pestDates[strings.ToLower(o.PestSigns)] = append(pestDates[strings.ToLower(o.PestSigns)], date)
		}
		if abnormalText(o.ReproductionSigns) {
			reproductionDates = append(reproductionDates, date)
		}
	}
	if out.ConsecutiveAbnormalDays >= 2 {
		out.Alerts = append(out.Alerts, TrendAlert{Level: "critical", Code: "consecutive_deterioration", Reason: "连续两次记录显示长势或病虫征象异常", Dates: lastDates(valid, 2), MustOpenDeviation: true})
	}
	if len(reproductionDates) > 0 {
		out.Alerts = append(out.Alerts, TrendAlert{Level: "critical", Code: "reproduction_detected", Reason: "观察记录出现繁殖迹象", Dates: reproductionDates, MustOpenDeviation: true})
	}
	for _, dates := range pestDates {
		if len(dates) >= 2 {
			out.Alerts = append(out.Alerts, TrendAlert{Level: "warning", Code: "repeated_pest_sign", Reason: "同类病虫征象重复出现", Dates: dates})
			break
		}
	}
	return out
}

func abnormalText(s string) bool {
	n := strings.ToLower(strings.TrimSpace(s))
	return n != "" && n != "无" && n != "正常" && n != "长势稳定" && n != "none" && n != "normal"
}
func lastDates(in []domain.ObservationEntry, n int) []string {
	if len(in) < n {
		n = len(in)
	}
	out := make([]string, 0, n)
	for _, o := range in[len(in)-n:] {
		out = append(out, o.ObservedOn.Format("2006-01-02"))
	}
	return out
}

func InRange(t time.Time, from, to *time.Time) bool {
	return (from == nil || !t.Before(*from)) && (to == nil || !t.After(*to))
}
