// Package summary aggregates work evidence for local calendar dates.
package summary

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/taupikpirdian/wlog/domain/dashboard"
	"github.com/taupikpirdian/wlog/domain/session"
)

type Day struct {
	Date       time.Time
	Seconds    int64
	Details    []string
	HasWorklog bool
}

// WeekDates returns Monday through Sunday, including dates with no records.
func WeekDates(now time.Time, location *time.Location) ([]time.Time, error) {
	if location == nil {
		return nil, fmt.Errorf("%w: missing local timezone", dashboard.ErrInvalidData)
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	daysSinceMonday := (int(start.Weekday()) + 6) % 7
	start = start.AddDate(0, 0, -daysSinceMonday)
	dates := make([]time.Time, 7)
	for i := range dates {
		dates[i] = start.AddDate(0, 0, i)
	}
	return dates, nil
}

type detail struct {
	at   time.Time
	kind int
	id   int64
	text string
}

// BuildDay clips sessions to the selected date and caps active time at now.
// Notes and commits contribute details, never additional tracked time.
func BuildDay(source dashboard.Snapshot, date, now time.Time, location *time.Location) (Day, error) {
	if location == nil {
		return Day{}, fmt.Errorf("%w: missing local timezone", dashboard.ErrInvalidData)
	}
	local := date.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	end := start.AddDate(0, 0, 1)
	day := Day{Date: start}
	contains := func(at time.Time) bool { return !at.Before(start) && at.Before(end) && !at.After(now) }
	var details []detail
	activeCount := 0
	for _, s := range source.Sessions {
		var until time.Time
		switch s.Status {
		case session.Active:
			activeCount++
			if s.EndedAt != nil || s.StartedAt.After(now) || activeCount > 1 {
				return Day{}, dashboard.ErrInvalidData
			}
			until = now
		case session.Completed:
			if s.EndedAt == nil || s.EndedAt.Before(s.StartedAt) {
				return Day{}, dashboard.ErrInvalidData
			}
			until = *s.EndedAt
			if until.After(now) {
				until = now
			}
		default:
			return Day{}, dashboard.ErrInvalidData
		}
		from := s.StartedAt
		if from.Before(start) {
			from = start
		}
		if until.After(end) {
			until = end
		}
		if until.After(from) || contains(s.StartedAt) {
			day.HasWorklog = true
			if until.After(from) {
				day.Seconds += int64(until.Sub(from) / time.Second)
			}
			details = append(details, detail{at: from, kind: 0, id: s.ID, text: s.Title})
		}
	}
	for _, a := range source.Activities {
		if !contains(a.At) {
			continue
		}
		if a.Type != "NOTE" && a.Type != "GIT_COMMIT" {
			return Day{}, dashboard.ErrInvalidData
		}
		day.HasWorklog = true
		details = append(details, detail{at: a.At, kind: 1, id: a.ID, text: a.Text})
	}
	sort.SliceStable(details, func(i, j int) bool {
		a, b := details[i], details[j]
		if !a.at.Equal(b.at) {
			return a.at.Before(b.at)
		}
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		return a.id < b.id
	})
	seen := map[string]bool{}
	for _, d := range details {
		for _, line := range strings.Split(d.text, "\n") {
			line = strings.Join(strings.Fields(line), " ")
			if line != "" && !seen[line] {
				day.Details = append(day.Details, line)
				seen[line] = true
			}
		}
	}
	return day, nil
}
