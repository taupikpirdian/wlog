package session_test

import (
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/taupikpirdian/wlog/application/session"
	domain "github.com/taupikpirdian/wlog/domain/session"
	"github.com/taupikpirdian/wlog/domain/ticket"
)

type manualStoreFunc func(context.Context, domain.Session, time.Time) (domain.Session, error)

func (f manualStoreFunc) InsertManual(ctx context.Context, s domain.Session, at time.Time) (domain.Session, error) {
	return f(ctx, s, at)
}

func TestManualOrchestration(t *testing.T) {
	failure := errors.New("storage failed")
	pattern, _ := ticket.NewKeyPattern(`OOT-[0-9]+`)
	for _, tc := range []struct {
		name, key, title, from, repo string
		cancelBefore, cancelDuring   bool
		storeErr                     error
		calls                        int
	}{
		{name: "completed", key: "OOT-1", title: " Fix tax ", from: "09:00", repo: "/fixture", calls: 1},
		{name: "outside git", key: "OOT-1", title: "Fix", from: "09:00", calls: 1},
		{name: "invalid key", key: "bad", title: "Fix", from: "09:00"},
		{name: "empty title", key: "OOT-1", title: " ", from: "09:00"},
		{name: "future", key: "OOT-1", title: "Fix", from: "13:00"},
		{name: "cancel before", key: "OOT-1", title: "Fix", from: "09:00", cancelBefore: true},
		{name: "cancel repository", key: "OOT-1", title: "Fix", from: "09:00", cancelDuring: true},
		{name: "store failure", key: "OOT-1", title: "Fix", from: "09:00", storeErr: failure, calls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelBefore {
				cancel()
			}
			now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
			calls, clocks := 0, 0
			svc := application.NewManualService(manualStoreFunc(func(got context.Context, s domain.Session, at time.Time) (domain.Session, error) {
				calls++
				if got != ctx || !at.Equal(now) || s.Title == "" || s.Status != domain.Completed || *s.DurationSeconds != 7200 {
					t.Fatalf("store values: %+v %v", s, at)
				}
				if (s.Repository == nil) != (tc.repo == "") {
					t.Fatal("repository mapping")
				}
				s.ID = 10
				s.TicketID = 2
				return s, tc.storeErr
			}), pattern, func() time.Time { clocks++; return now }, time.UTC, func(context.Context) string {
				if tc.cancelDuring {
					cancel()
				}
				return tc.repo
			})
			result, err := svc.CreateCompleted(ctx, tc.key, tc.title, tc.from, "11:00")
			if calls != tc.calls {
				t.Fatalf("calls=%d", calls)
			}
			if tc.calls == 1 && tc.storeErr == nil {
				if err != nil || result.ID != 10 {
					t.Fatalf("result: %+v %v", result, err)
				}
			} else if err == nil {
				t.Fatal("error missing")
			}
			if tc.storeErr != nil && !errors.Is(err, tc.storeErr) {
				t.Fatal(err)
			}
			if tc.cancelBefore || tc.cancelDuring {
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			}
			if tc.calls == 1 && clocks != 1 {
				t.Fatalf("clock=%d", clocks)
			}
		})
	}
}

func TestManualStartSince(t *testing.T) {
	pattern, _ := ticket.NewKeyPattern(`OOT-[0-9]+`)
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	svc := application.NewManualService(manualStoreFunc(func(_ context.Context, s domain.Session, at time.Time) (domain.Session, error) {
		if s.Status != domain.Active || s.EndedAt != nil || s.DurationSeconds != nil || s.StartedAt.Hour() != 9 || at != now {
			t.Fatalf("since: %+v %v", s, at)
		}
		return s, nil
	}), pattern, func() time.Time { return now }, time.UTC, func(context.Context) string { return "" })
	if _, err := svc.StartSince(context.Background(), "OOT-1", "Fix", "09:00"); err != nil {
		t.Fatal(err)
	}
}
