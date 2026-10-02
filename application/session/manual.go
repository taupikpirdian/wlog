package session

import (
	"context"
	"time"

	domain "github.com/taupikpirdian/wlog/domain/session"
	"github.com/taupikpirdian/wlog/domain/ticket"
)

type ManualStore interface {
	InsertManual(context.Context, domain.Session, time.Time) (domain.Session, error)
}
type ManualUsecase interface {
	CreateCompleted(context.Context, string, string, string, string) (domain.Session, error)
	StartSince(context.Context, string, string, string) (domain.Session, error)
}
type manualService struct {
	store      ManualStore
	pattern    ticket.KeyPattern
	now        func() time.Time
	location   *time.Location
	repository func(context.Context) string
}

func NewManualService(store ManualStore, pattern ticket.KeyPattern, now func() time.Time, location *time.Location, repository func(context.Context) string) ManualUsecase {
	return &manualService{store: store, pattern: pattern, now: now, location: location, repository: repository}
}
func (s *manualService) CreateCompleted(ctx context.Context, key, title, from, to string) (domain.Session, error) {
	return s.create(ctx, domain.ManualRequest{TicketKey: key, Title: title, From: from, To: to})
}
func (s *manualService) StartSince(ctx context.Context, key, title, since string) (domain.Session, error) {
	return s.create(ctx, domain.ManualRequest{TicketKey: key, Title: title, From: since, Backdated: true})
}
func (s *manualService) create(ctx context.Context, request domain.ManualRequest) (domain.Session, error) {
	if err := ctx.Err(); err != nil {
		return domain.Session{}, err
	}
	if err := s.pattern.Validate(request.TicketKey); err != nil {
		return domain.Session{}, err
	}
	now := s.now()
	value, err := domain.NewManualSession(request, now, s.location)
	if err != nil {
		return domain.Session{}, err
	}
	repo := s.repository(ctx)
	if repo != "" {
		value.Repository = &repo
	}
	if err := ctx.Err(); err != nil {
		return domain.Session{}, err
	}
	return s.store.InsertManual(ctx, value, now.UTC())
}
