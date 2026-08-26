package policy

import (
	"fmt"
	"sort"
	"time"

	"quarantine-workbench/internal/domain"
)

type ContinuityResult struct {
	RequiredCount  int      `json:"required_count"`
	ActualCount    int      `json:"actual_count"`
	LargestGapDays int      `json:"largest_gap_days"`
	Continuous     bool     `json:"continuous"`
	MissingDates   []string `json:"missing_dates"`
}

type ObservationWindow struct {
	DueDate        time.Time `json:"due_date"`
	EndDate        time.Time `json:"end_date"`
	Status         string    `json:"status"`
	ObservationIDs []string  `json:"observation_ids,omitempty"`
	Late           bool      `json:"late,omitempty"`
}

func BuildWindows(start, end time.Time, interval int, observations []domain.ObservationEntry, now time.Time) []ObservationWindow {
	if interval < 1 || end.Before(start) {
		return nil
	}
	var out []ObservationWindow
	for due := day(start); !due.After(day(end)); due = due.AddDate(0, 0, interval) {
		wend := due.AddDate(0, 0, interval-1)
		w := ObservationWindow{DueDate: due, EndDate: wend, Status: "overdue"}
		for _, o := range observations {
			d := day(o.ObservedOn)
			assigned := (!d.Before(due) && !d.After(wend)) || (o.WindowDueOn != nil && day(*o.WindowDueOn).Equal(due))
			if assigned {
				w.ObservationIDs = append(w.ObservationIDs, o.ID)
				if o.Late || d.After(wend) {
					w.Late = true
					if o.LateStatus == "approved" {
						w.Status = "completed"
					} else {
						w.Status = "late_pending"
					}
				} else {
					w.Status = "completed"
				}
			}
		}
		if len(w.ObservationIDs) == 0 {
			if day(now).Before(due) {
				w.Status = "upcoming"
			} else if !day(now).After(wend) {
				w.Status = "due"
			}
		}
		out = append(out, w)
	}
	return out
}

func CheckContinuity(start, through time.Time, intervalDays int, observations []domain.ObservationEntry) ContinuityResult {
	if intervalDays < 1 || through.Before(start) {
		return ContinuityResult{}
	}
	dates := make([]time.Time, 0, len(observations))
	seen := map[string]bool{}
	for _, observation := range observations {
		d := day(observation.ObservedOn)
		key := d.Format("2006-01-02")
		if !d.Before(day(start)) && !d.After(day(through)) && !seen[key] {
			seen[key] = true
			dates = append(dates, d)
		}
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	required := int(day(through).Sub(day(start)).Hours()/24)/intervalDays + 1
	missing := make([]string, 0)
	for due := day(start); !due.After(day(through)); due = due.AddDate(0, 0, intervalDays) {
		windowEnd := due.AddDate(0, 0, intervalDays-1)
		found := false
		for _, observed := range dates {
			if !observed.Before(due) && !observed.After(windowEnd) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, fmt.Sprintf("%s 至 %s", due.Format("2006-01-02"), windowEnd.Format("2006-01-02")))
		}
	}
	largest := 0
	previous := day(start)
	for _, observed := range dates {
		gap := int(observed.Sub(previous).Hours() / 24)
		if gap > largest {
			largest = gap
		}
		previous = observed
	}
	if gap := int(day(through).Sub(previous).Hours() / 24); gap > largest {
		largest = gap
	}
	return ContinuityResult{RequiredCount: required, ActualCount: len(dates), LargestGapDays: largest, Continuous: len(missing) == 0, MissingDates: missing}
}

func day(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
