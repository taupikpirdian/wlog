package session

import (
	"context"
	"time"

	domain "github.com/taupikpirdian/wlog/domain/session"
	"github.com/taupikpirdian/wlog/domain/ticket"
)

type Store interface {
	Active(context.Context) (*domain.Session, error)
	Start(context.Context, domain.Session, *domain.Session) (domain.Session, error)
	Complete(context.Context, domain.Session) error
}

type Service struct {
	store      Store
	pattern    ticket.KeyPattern
	now        func() time.Time
	repository func(context.Context) string
}

func NewService(store Store, pattern ticket.KeyPattern, now func() time.Time, repository func(context.Context) string) *Service {
	return &Service{store: store, pattern: pattern, now: now, repository: repository}
}

func (s *Service) Validate(key, title string) error {
	if err := s.pattern.Validate(key); err != nil {
		return err
	}
	_, err := domain.ValidateTitle(title)
	return err
}

func (s *Service) Active(ctx context.Context) (*domain.Session, error) { return s.store.Active(ctx) }

// Start uses the snapshot approved by the caller; persistence rechecks it atomically.
func (s *Service) Start(ctx context.Context, key, title string, expected *domain.Session) (domain.Session, error) {
	if err := s.Validate(key, title); err != nil {
		return domain.Session{}, err
	}
	title, _ = domain.ValidateTitle(title)
	repo := s.repository(ctx)
	if err := ctx.Err(); err != nil {
		return domain.Session{}, err
	}
	at := s.now().UTC()
	value := domain.Session{TicketKey: key, Title: title, StartedAt: at, Status: domain.Active}
	if repo != "" {
		value.Repository = &repo
	}
	var completed *domain.Session
	if expected != nil {
		previous, err := expected.Complete(at)
		if err != nil {
			return domain.Session{}, err
		}
		completed = &previous
	}
	return s.store.Start(ctx, value, completed)
}

func (s *Service) Stop(ctx context.Context) (domain.Session, error) {
	value, err := s.store.Active(ctx)
	if err != nil {
		return domain.Session{}, err
	}
	if value == nil {
		return domain.Session{}, domain.ErrNoActive
	}
	completed, err := value.Complete(s.now())
	if err != nil {
		return domain.Session{}, err
	}
	if err := s.store.Complete(ctx, completed); err != nil {
		return domain.Session{}, err
	}
	return completed, nil
}
