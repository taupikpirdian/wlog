package summary_test

import (
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/taupikpirdian/wlog/application/summary"
	"github.com/taupikpirdian/wlog/domain/dashboard"
	"github.com/taupikpirdian/wlog/domain/session"
)

func TestWeeklyTicketSelectionKeepsWholeTicketDescriptionContext(t *testing.T) {
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	snapshot := dashboard.Snapshot{Sessions: []session.Session{
		{ID: 1, TicketKey: "OOT-1", Title: "Previous week", StartedAt: now.AddDate(0, 0, -8), EndedAt: timePtr(now.AddDate(0, 0, -8).Add(time.Hour)), Status: session.Completed},
		{ID: 2, TicketKey: "OOT-1", Title: "This week", StartedAt: now.AddDate(0, 0, -1), EndedAt: timePtr(now.AddDate(0, 0, -1).Add(time.Hour)), Status: session.Completed},
		{ID: 3, TicketKey: "OLD", Title: "Excluded", StartedAt: now.AddDate(0, 0, -8), EndedAt: timePtr(now.AddDate(0, 0, -8).Add(time.Hour)), Status: session.Completed},
	}}
	s := &store{snapshot: snapshot}
	e := &emailReader{err: errors.New("email is not needed for ticket descriptions")}
	reader := application.NewService(s, e, func() time.Time { return now }, time.UTC)
	tickets, err := reader.WeekTickets(context.Background())
	if err != nil || len(tickets) != 1 || tickets[0].Key != "OOT-1" || tickets[0].Day.Seconds != 3600 || s.calls != 1 {
		t.Fatalf("tickets=%+v error=%v calls=%d", tickets, err, s.calls)
	}
	value, err := reader.TicketDescriptionContext(context.Background(), tickets[0].Key)
	if err != nil || !value.TicketOnly || !value.Summary.Day.Date.IsZero() || len(value.AllTicketWorklogs.Sessions) != 2 || e.calls != 0 {
		t.Fatalf("context=%+v error=%v", value, err)
	}
	if _, err := reader.TicketDescriptionContext(context.Background(), "MISSING"); err == nil {
		t.Fatal("missing ticket accepted")
	}
}

func TestWeeklyTicketStoreErrors(t *testing.T) {
	failure := errors.New("database unavailable")
	reader := application.NewService(&store{err: failure}, &emailReader{}, time.Now, time.UTC)
	if _, err := reader.WeekTickets(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
	if _, err := reader.TicketDescriptionContext(context.Background(), "OOT-1"); !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
}
