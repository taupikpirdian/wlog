package session

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/taupikpirdian/wlog/domain/session"
	"github.com/taupikpirdian/wlog/domain/ticket"
)

type sessionStoreStub struct {
	active   func(context.Context) (*domain.Session, error)
	start    func(context.Context, domain.Session, *domain.Session) (domain.Session, error)
	complete func(context.Context, domain.Session) error
}

func (s sessionStoreStub) Active(ctx context.Context) (*domain.Session, error) { return s.active(ctx) }
func (s sessionStoreStub) Start(ctx context.Context, value domain.Session, previous *domain.Session) (domain.Session, error) {
	return s.start(ctx, value, previous)
}
func (s sessionStoreStub) Complete(ctx context.Context, value domain.Session) error {
	return s.complete(ctx, value)
}

func TestStartValidationCancellationAndClock(t *testing.T) {
	pattern, _ := ticket.NewKeyPattern(`OOT-[0-9]+`)
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	previous := &domain.Session{ID: 12, Status: domain.Active, StartedAt: at.Add(time.Hour)}
	for _, tc := range []struct {
		name, key, title string
		cancel           bool
		previous         *domain.Session
		want             error
	}{
		{"invalid key", "bad", "Title", false, nil, ticket.ErrInvalidKey},
		{"empty title", "OOT-1", " \t ", false, nil, domain.ErrEmptyTitle},
		{"cancelled repository lookup", "OOT-1", "Title", true, nil, context.Canceled},
		{"clock before previous", "OOT-1", "Title", false, previous, domain.ErrClockBackwards},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store := sessionStoreStub{start: func(context.Context, domain.Session, *domain.Session) (domain.Session, error) {
				t.Fatal("unexpected write")
				return domain.Session{}, nil
			}}
			service := NewService(store, pattern, func() time.Time { return at }, func(context.Context) string {
				if tc.cancel {
					cancel()
				}
				return ""
			})
			if _, err := service.Start(ctx, tc.key, tc.title, tc.previous); !errors.Is(err, tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestStartAndSwitchUseOnePivot(t *testing.T) {
	ctx := context.Background()
	pattern, _ := ticket.NewKeyPattern(`OOT-[0-9]+`)
	at := time.Date(2026, 10, 1, 9, 0, 0, 123, time.FixedZone("test", 7*3600))
	failure := errors.New("write failed")
	previous := &domain.Session{ID: 12, Status: domain.Active, StartedAt: at.Add(-time.Hour)}
	for _, tc := range []struct {
		name, repo string
		previous   *domain.Session
		cause      error
	}{
		{"without repo", "", nil, nil}, {"switch with repo", "/repo", previous, nil}, {"write failure", "", nil, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clocks := 0
			store := sessionStoreStub{start: func(got context.Context, value domain.Session, completed *domain.Session) (domain.Session, error) {
				if got != ctx || value.Title != "Title" || value.TicketKey != "OOT-1" || value.Status != domain.Active || !value.StartedAt.Equal(at) || value.StartedAt.Location() != time.UTC || (value.Repository != nil) != (tc.repo != "") {
					t.Fatalf("new session=%+v", value)
				}
				if value.Repository != nil && *value.Repository != tc.repo {
					t.Fatalf("repository=%q", *value.Repository)
				}
				if tc.previous == nil && completed != nil {
					t.Fatal("unexpected previous session")
				}
				if tc.previous != nil && (completed == nil || completed.ID != previous.ID || completed.Status != domain.Completed || !completed.EndedAt.Equal(value.StartedAt) || *completed.DurationSeconds != 3600) {
					t.Fatalf("completed=%+v", completed)
				}
				value.ID = 101
				return value, tc.cause
			}}
			service := NewService(store, pattern, func() time.Time { clocks++; return at }, func(context.Context) string { return tc.repo })
			value, err := service.Start(ctx, "OOT-1", "  Title  ", tc.previous)
			if !errors.Is(err, tc.cause) || clocks != 1 || value.ID != 101 {
				t.Fatalf("session=%+v error=%v clocks=%d", value, err, clocks)
			}
		})
	}
}

func TestActiveAndStopOutcomes(t *testing.T) {
	ctx := context.Background()
	pattern, _ := ticket.NewKeyPattern(`OOT-[0-9]+`)
	at := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	failure := errors.New("store failed")
	active := &domain.Session{ID: 12, Status: domain.Active, StartedAt: at.Add(-90 * time.Second)}
	for _, tc := range []struct {
		name                          string
		active                        *domain.Session
		readErr, completeErr, wantErr error
		now                           time.Time
		writes                        int
	}{
		{"read fails", nil, failure, nil, failure, at, 0},
		{"no active", nil, nil, nil, domain.ErrNoActive, at, 0},
		{"clock backwards", active, nil, nil, domain.ErrClockBackwards, active.StartedAt.Add(-time.Second), 0},
		{"completion fails", active, nil, failure, failure, at, 1},
		{"success", active, nil, nil, nil, at, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			store := sessionStoreStub{
				active: func(got context.Context) (*domain.Session, error) {
					if got != ctx {
						t.Fatal("context changed")
					}
					return tc.active, tc.readErr
				},
				complete: func(got context.Context, value domain.Session) error {
					writes++
					if got != ctx || value.ID != 12 || value.Status != domain.Completed || !value.EndedAt.Equal(at) || *value.DurationSeconds != 90 {
						t.Fatalf("completion=%+v", value)
					}
					return tc.completeErr
				},
			}
			service := NewService(store, pattern, func() time.Time { return tc.now }, func(context.Context) string { return "" })
			found, readErr := service.Active(ctx)
			if !errors.Is(readErr, tc.readErr) || found != tc.active {
				t.Fatalf("active=%+v error=%v", found, readErr)
			}
			value, err := service.Stop(ctx)
			if !errors.Is(err, tc.wantErr) || writes != tc.writes {
				t.Fatalf("error=%v writes=%d", err, writes)
			}
			if err == nil && (value.ID != active.ID || *value.DurationSeconds != 90) {
				t.Fatalf("stop=%+v", value)
			}
			if active.Status != domain.Active || active.EndedAt != nil {
				t.Fatal("mutated original snapshot")
			}
		})
	}
}
