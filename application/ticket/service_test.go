package ticket

import (
	"context"
	"errors"
	"testing"

	domain "github.com/taupikpirdian/wlog/domain/ticket"
)

type ticketStoreStub struct {
	create func(context.Context, domain.Ticket) (domain.Ticket, error)
	find   func(context.Context, string) (domain.Ticket, error)
	upsert func(context.Context, domain.Ticket) (domain.Ticket, error)
}

func (s ticketStoreStub) Create(ctx context.Context, v domain.Ticket) (domain.Ticket, error) {
	return s.create(ctx, v)
}
func (s ticketStoreStub) FindByKey(ctx context.Context, key string) (domain.Ticket, error) {
	return s.find(ctx, key)
}
func (s ticketStoreStub) Upsert(ctx context.Context, v domain.Ticket) (domain.Ticket, error) {
	return s.upsert(ctx, v)
}

func TestTicketServiceValidationAndPersistence(t *testing.T) {
	ctx := context.Background()
	pattern, _ := domain.NewKeyPattern(`OOT-[0-9]+`)
	failure := errors.New("store failed")
	for _, cause := range []error{nil, failure} {
		calls := 0
		write := func(got context.Context, value domain.Ticket) (domain.Ticket, error) {
			calls++
			if got != ctx || value.Key != "OOT-1" || value.Title == nil || *value.Title != "Title" {
				t.Fatalf("ticket=%+v context=%v", value, got)
			}
			value.ID = 12
			return value, cause
		}
		find := func(got context.Context, key string) (domain.Ticket, error) {
			calls++
			if got != ctx || key != "OOT-1" {
				t.Fatalf("find=%q context=%v", key, got)
			}
			return domain.Ticket{ID: 12, Key: key}, cause
		}
		service := NewService(ticketStoreStub{create: write, upsert: write, find: find}, pattern)
		for _, operation := range []func(context.Context, string, string) (domain.Ticket, error){service.Create, service.Upsert} {
			if _, err := operation(ctx, "OTHER-1", "Title"); !errors.Is(err, domain.ErrInvalidKey) {
				t.Fatalf("validation: %v", err)
			}
			got, err := operation(ctx, "OOT-1", "  Title  ")
			if !errors.Is(err, cause) || got.ID != 12 {
				t.Fatalf("write=%+v error=%v", got, err)
			}
		}
		if _, err := service.FindByKey(ctx, "bad"); !errors.Is(err, domain.ErrInvalidKey) {
			t.Fatalf("find validation: %v", err)
		}
		got, err := service.FindByKey(ctx, "OOT-1")
		if !errors.Is(err, cause) || got.ID != 12 {
			t.Fatalf("find=%+v error=%v", got, err)
		}
		if calls != 3 {
			t.Fatalf("invalid input reached store: calls=%d", calls)
		}
		if key, found := service.ExtractKey("OOT-2 first OOT-1 second"); !found || key != "OOT-2" {
			t.Fatalf("extract=%q %v", key, found)
		}
		if key, found := service.ExtractKey("no key"); found || key != "" {
			t.Fatalf("extract no match=%q %v", key, found)
		}
	}
}
