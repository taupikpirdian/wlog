package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/domain/activity"
	"github.com/taupikpirdian/wlog/domain/session"
)

type noteStoreStub struct {
	active func(context.Context) (*session.Session, error)
	add    func(context.Context, activity.Note, session.Session) (activity.Note, error)
}

func (s noteStoreStub) Active(ctx context.Context) (*session.Session, error) { return s.active(ctx) }
func (s noteStoreStub) AddNote(ctx context.Context, value activity.Note, expected session.Session) (activity.Note, error) {
	return s.add(ctx, value, expected)
}

func TestAddNoteValidationAndDependencyFailures(t *testing.T) {
	ctx := context.Background()
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	failure := errors.New("store failed")
	active := &session.Session{ID: 12, TicketID: 2, TicketKey: "OOT-1", Status: session.Active, StartedAt: at}
	for _, tc := range []struct {
		name, description          string
		active                     *session.Session
		readErr, writeErr, wantErr error
		now                        time.Time
		reads, writes              int
	}{
		{"invalid description", " \t ", active, nil, nil, activity.ErrEmptyDescription, at, 0, 0},
		{"read fails", "Note", nil, failure, nil, failure, at, 1, 0},
		{"no active session", "Note", nil, nil, nil, session.ErrNoActive, at, 1, 0},
		{"clock backwards", "Note", active, nil, nil, activity.ErrClockBeforeSession, at.Add(-time.Second), 1, 0},
		{"write fails", "Note", active, nil, failure, failure, at, 1, 1},
		{"success", "  Note  ", active, nil, nil, nil, at.Add(time.Second), 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads, writes := 0, 0
			store := noteStoreStub{
				active: func(got context.Context) (*session.Session, error) {
					reads++
					if got != ctx {
						t.Fatal("read context changed")
					}
					return tc.active, tc.readErr
				},
				add: func(got context.Context, value activity.Note, expected session.Session) (activity.Note, error) {
					writes++
					if got != ctx || expected.ID != active.ID || value.SessionID != active.ID || value.TicketID != active.TicketID || value.Description != "Note" || !value.CreatedAt.Equal(tc.now) {
						t.Fatalf("note=%+v snapshot=%+v", value, expected)
					}
					value.ID = 101
					return value, tc.writeErr
				},
			}
			value, err := NewService(store, func() time.Time { return tc.now }).AddNote(ctx, tc.description)
			if !errors.Is(err, tc.wantErr) || reads != tc.reads || writes != tc.writes {
				t.Fatalf("error=%v reads=%d writes=%d", err, reads, writes)
			}
			if err == nil && value.ID != 101 {
				t.Fatalf("note=%+v", value)
			}
		})
	}
}
