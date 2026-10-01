package activity

import (
	"errors"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/domain/session"
)

func TestNewNoteContentAndTime(t *testing.T) {
	start := time.Date(2026, 10, 1, 23, 59, 59, 0, time.FixedZone("test", 7*3600))
	repo := "/example/repo"
	active := session.Session{ID: 12, TicketID: 2, TicketKey: "OOT-3668", Status: session.Active, StartedAt: start, Repository: &repo}
	for _, tc := range []struct {
		name, description string
		at                time.Time
		err               error
	}{
		{"empty", " \t\n ", start, ErrEmptyDescription},
		{"clock backwards", "Note", start.Add(-time.Nanosecond), ErrClockBeforeSession},
		{"same start", "Note", start, nil},
		{"cross midnight", "  QA — OOT-9999\n'quoted'; DROP TABLE tickets;  ", start.Add(2 * time.Second), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := NewNote(tc.description, active, tc.at)
			if !errors.Is(err, tc.err) {
				t.Fatalf("error=%v", err)
			}
			if err != nil {
				return
			}
			if value.SessionID != active.ID || value.TicketID != active.TicketID || value.TicketKey != active.TicketKey || !value.CreatedAt.Equal(tc.at) || value.CreatedAt.Location() != time.UTC {
				t.Fatalf("note=%+v", value)
			}
			if tc.name == "cross midnight" && value.Description != "QA — OOT-9999\n'quoted'; DROP TABLE tickets;" {
				t.Fatalf("description=%q", value.Description)
			}
			*value.Repository = "changed"
			if repo != "/example/repo" {
				t.Fatal("note mutated snapshot repository")
			}
		})
	}
}

func TestNoteRejectsCompletedSession(t *testing.T) {
	if _, err := NewNote("Note", session.Session{Status: session.Completed}, time.Now()); !errors.Is(err, session.ErrConflict) {
		t.Fatalf("completed session: %v", err)
	}
}
