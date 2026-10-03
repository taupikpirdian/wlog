package summary

import (
	"context"
	"time"

	"github.com/taupikpirdian/wlog/domain/dashboard"
	environment "github.com/taupikpirdian/wlog/domain/environment"
	domain "github.com/taupikpirdian/wlog/domain/summary"
)

type SnapshotStore interface {
	ReadSnapshot(context.Context) (dashboard.Snapshot, error)
}
type EmailReader interface {
	Email(context.Context) (string, error)
}
type Result struct {
	Day                domain.Day
	Email              string
	EnvironmentChanges environment.Changes `json:"environment_changes"`
}
type EnvironmentCheck func(context.Context, TicketAIContext) environment.Changes
type Reader interface {
	Week(context.Context) ([]domain.Day, error)
	Summary(context.Context, time.Time) (Result, error)
}
type service struct {
	store            SnapshotStore
	email            EmailReader
	now              func() time.Time
	location         *time.Location
	checkEnvironment EnvironmentCheck
}

func NewService(store SnapshotStore, email EmailReader, now func() time.Time, location *time.Location, checks ...EnvironmentCheck) TicketReader {
	s := &service{store: store, email: email, now: now, location: location}
	if len(checks) > 0 {
		s.checkEnvironment = checks[0]
	}
	return s
}

func (s *service) environment(ctx context.Context, value TicketAIContext) environment.Changes {
	if s.checkEnvironment != nil {
		return s.checkEnvironment(ctx, value)
	}
	return environment.Changes{Status: "failed", NewVariables: []string{}, MissingFromTemplate: []string{}}
}

func (s *service) Week(ctx context.Context) ([]domain.Day, error) {
	now := s.now()
	dates, err := domain.WeekDates(now, s.location)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.store.ReadSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	days := make([]domain.Day, len(dates))
	for i, date := range dates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		days[i], err = domain.BuildDay(snapshot, date, now, s.location)
		if err != nil {
			return nil, err
		}
	}
	return days, nil
}

func (s *service) Summary(ctx context.Context, date time.Time) (Result, error) {
	now := s.now()
	snapshot, err := s.store.ReadSnapshot(ctx)
	if err != nil {
		return Result{}, err
	}
	day, err := domain.BuildDay(snapshot, date, now, s.location)
	if err != nil {
		return Result{}, err
	}
	email, err := s.email.Email(ctx)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	result := Result{Day: day, Email: email}
	result.EnvironmentChanges = s.environment(ctx, TicketAIContext{Summary: result, AllTicketWorklogs: snapshot})
	return result, ctx.Err()
}
