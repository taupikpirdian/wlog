// Package dashboard projects stored work evidence into a local daily view.
package dashboard

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/taupikpirdian/wlog/domain/session"
)

var ErrInvalidData = errors.New("invalid worklog data or device time; check stored timestamps and device clock")

// Snapshot is a consistent read of existing work records, not a new aggregate.
type Snapshot struct {
	Sessions   []session.Session
	Activities []Activity
}

type Activity struct {
	ID         int64
	TicketKey  string
	SessionID  *int64
	Type       string
	Text       string
	Hash       string
	At         time.Time
	Repository string
	Branch     string
}

type Event struct {
	At         time.Time
	Kind       string
	TicketKey  string
	Text       string
	Hash       string
	SourceID   int64
	Repository string
}

type TicketSummary struct {
	Key          string
	Seconds      int64
	Repositories []string
}
type ActiveView struct {
	Session        session.Session
	ElapsedSeconds int64
}
type View struct {
	Now, Start, End                 time.Time
	Location                        *time.Location
	Active                          *ActiveView
	Tickets                         []TicketSummary
	TotalSeconds                    int64
	Events, Unsessioned, Unassigned []Event
}

// Build applies calendar, duration, and evidence rules without modifying sources.
func Build(source Snapshot, now time.Time, location *time.Location) (View, error) {
	if location == nil {
		return View{}, fmt.Errorf("%w: missing local timezone", ErrInvalidData)
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	view := View{Now: now, Start: start, End: start.AddDate(0, 0, 1), Location: location}
	totals := map[string]int64{}
	repositories := map[string]map[string]struct{}{}
	addRepository := func(key, repository string) {
		if repository == "" {
			return
		}
		if repositories[key] == nil {
			repositories[key] = map[string]struct{}{}
		}
		repositories[key][repository] = struct{}{}
	}
	for _, s := range source.Sessions {
		repository := ""
		if s.Repository != nil {
			repository = *s.Repository
		}
		if s.TicketKey == "" {
			return View{}, fmt.Errorf("%w: session %d has no ticket", ErrInvalidData, s.ID)
		}
		var end time.Time
		switch s.Status {
		case session.Active:
			if s.EndedAt != nil || s.StartedAt.After(now) || view.Active != nil {
				return View{}, fmt.Errorf("%w: inconsistent active session %d", ErrInvalidData, s.ID)
			}
			view.Active = &ActiveView{Session: s, ElapsedSeconds: int64(now.Sub(s.StartedAt) / time.Second)}
			end = now
		case session.Completed:
			if s.EndedAt == nil || s.EndedAt.Before(s.StartedAt) {
				return View{}, fmt.Errorf("%w: invalid session range %d", ErrInvalidData, s.ID)
			}
			end = *s.EndedAt
			if end.After(now) {
				end = now
			}
		default:
			return View{}, fmt.Errorf("%w: unknown session status %q", ErrInvalidData, s.Status)
		}
		from, until := s.StartedAt, end
		if from.Before(view.Start) {
			from = view.Start
		}
		if until.After(from) {
			seconds := int64(until.Sub(from) / time.Second)
			totals[s.TicketKey] += seconds
			view.TotalSeconds += seconds
			addRepository(s.TicketKey, repository)
		}
		if view.contains(s.StartedAt) {
			view.Events = append(view.Events, Event{At: s.StartedAt, Kind: "START", TicketKey: s.TicketKey, SourceID: s.ID, Repository: repository})
			totals[s.TicketKey] += 0
			addRepository(s.TicketKey, repository)
		}
		if s.Status == session.Completed && view.contains(*s.EndedAt) {
			view.Events = append(view.Events, Event{At: *s.EndedAt, Kind: "STOP", TicketKey: s.TicketKey, SourceID: s.ID, Repository: repository})
			totals[s.TicketKey] += 0
			addRepository(s.TicketKey, repository)
		}
	}
	for _, a := range source.Activities {
		kind, text := a.Type, a.Text
		switch kind {
		case "NOTE":
		case "GIT_COMMIT":
			kind = "COMMIT"
			if text == "" {
				text = "(no commit message)"
			}
		default:
			return View{}, fmt.Errorf("%w: unknown activity type %q", ErrInvalidData, kind)
		}
		if !view.contains(a.At) {
			continue
		}
		event := Event{At: a.At, Kind: kind, TicketKey: a.TicketKey, Text: text, Hash: a.Hash, SourceID: a.ID, Repository: a.Repository}
		view.Events = append(view.Events, event)
		if a.TicketKey == "" {
			view.Unassigned = append(view.Unassigned, event)
		} else {
			totals[a.TicketKey] += 0
			addRepository(a.TicketKey, a.Repository)
			if a.SessionID == nil {
				view.Unsessioned = append(view.Unsessioned, event)
			}
		}
	}
	for key, seconds := range totals {
		summary := TicketSummary{Key: key, Seconds: seconds}
		for repository := range repositories[key] {
			summary.Repositories = append(summary.Repositories, repository)
		}
		sort.Strings(summary.Repositories)
		view.Tickets = append(view.Tickets, summary)
	}
	sort.Slice(view.Tickets, func(i, j int) bool { return view.Tickets[i].Key < view.Tickets[j].Key })
	sortEvents(view.Events)
	sortEvents(view.Unsessioned)
	sortEvents(view.Unassigned)
	return view, nil
}

func (v View) contains(at time.Time) bool { return !at.Before(v.Start) && at.Before(v.End) }
func sortEvents(events []Event) {
	ranks := map[string]int{"START": 0, "NOTE": 1, "COMMIT": 2, "STOP": 3}
	sort.Slice(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		if ranks[a.Kind] != ranks[b.Kind] {
			return ranks[a.Kind] < ranks[b.Kind]
		}
		return a.SourceID < b.SourceID
	})
}
