package summary

import (
	"context"
	"fmt"
	"time"

	domain "github.com/taupikpirdian/wlog/domain/summary"
)

type TicketReader interface {
	Reader
	Tickets(context.Context, time.Time) ([]domain.TicketDay, error)
	WeekTickets(context.Context) ([]domain.TicketDay, error)
	TicketSummary(context.Context, time.Time, string) (Result, error)
	TicketContext(context.Context, time.Time, string) (TicketAIContext, error)
	TicketDescriptionContext(context.Context, string) (TicketAIContext, error)
}

func (s *service) WeekTickets(ctx context.Context) ([]domain.TicketDay, error) {
	snapshot, err := s.store.ReadSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return domain.TicketsForWeek(snapshot, s.now(), s.location)
}

// TicketDescriptionContext uses the entire ticket without a selected-date scope.
func (s *service) TicketDescriptionContext(ctx context.Context, key string) (TicketAIContext, error) {
	snapshot, err := s.store.ReadSnapshot(ctx)
	if err != nil {
		return TicketAIContext{}, err
	}
	snapshot = domain.FilterTicket(snapshot, key)
	if len(snapshot.Sessions) == 0 && len(snapshot.Activities) == 0 {
		return TicketAIContext{}, fmt.Errorf("no recorded worklogs for %s", key)
	}
	return TicketAIContext{TicketKey: key, AllTicketWorklogs: snapshot, TicketOnly: true}, nil
}

func (s *service) Tickets(ctx context.Context, date time.Time) ([]domain.TicketDay, error) {
	snapshot, err := s.store.ReadSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return domain.TicketsForDay(snapshot, date, s.now(), s.location)
}

func (s *service) TicketSummary(ctx context.Context, date time.Time, key string) (Result, error) {
	value, err := s.TicketContext(ctx, date, key)
	return value.Summary, err
}

func (s *service) TicketContext(ctx context.Context, date time.Time, key string) (TicketAIContext, error) {
	snapshot, err := s.store.ReadSnapshot(ctx)
	if err != nil {
		return TicketAIContext{}, err
	}
	snapshot = domain.FilterTicket(snapshot, key)
	day, err := domain.BuildDay(snapshot, date, s.now(), s.location)
	if err != nil {
		return TicketAIContext{}, err
	}
	if !day.HasWorklog {
		return TicketAIContext{}, fmt.Errorf("no worklog for %s on %s", key, date.Format("2006-01-02"))
	}
	email, err := s.email.Email(ctx)
	if err != nil {
		return TicketAIContext{}, err
	}
	value := TicketAIContext{TicketKey: key, Summary: Result{Day: day, Email: email}, AllTicketWorklogs: snapshot}
	value.Summary.EnvironmentChanges = s.environment(ctx, value)
	return value, ctx.Err()
}
