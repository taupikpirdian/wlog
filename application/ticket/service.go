package ticket

import (
	"context"

	domainticket "github.com/taupikpirdian/wlog/domain/ticket"
)

// TicketStore describes persistence behavior consumed by this service.
type TicketStore interface {
	Create(context.Context, domainticket.Ticket) (domainticket.Ticket, error)
	FindByKey(context.Context, string) (domainticket.Ticket, error)
	Upsert(context.Context, domainticket.Ticket) (domainticket.Ticket, error)
}

// Service coordinates ticket validation and persistence.
type Service struct {
	store   TicketStore
	pattern domainticket.KeyPattern
}

func NewService(store TicketStore, pattern domainticket.KeyPattern) *Service {
	return &Service{store: store, pattern: pattern}
}

// ExtractKey returns the first configured ticket key in a commit message.
// A message without a match is not an error and causes no persistence changes.
func (s *Service) ExtractKey(message string) (string, bool) {
	return s.pattern.Extract(message)
}

func (s *Service) Create(ctx context.Context, key, title string) (domainticket.Ticket, error) {
	value, err := s.pattern.New(key, title)
	if err != nil {
		return domainticket.Ticket{}, err
	}
	return s.store.Create(ctx, value)
}

func (s *Service) FindByKey(ctx context.Context, key string) (domainticket.Ticket, error) {
	if err := s.pattern.Validate(key); err != nil {
		return domainticket.Ticket{}, err
	}
	return s.store.FindByKey(ctx, key)
}

func (s *Service) Upsert(ctx context.Context, key, title string) (domainticket.Ticket, error) {
	value, err := s.pattern.New(key, title)
	if err != nil {
		return domainticket.Ticket{}, err
	}
	return s.store.Upsert(ctx, value)
}
