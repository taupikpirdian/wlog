package dashboard_test

import (
	"context"
	"errors"
	"testing"
	"time"

	application "github.com/taupikpirdian/wlog/application/dashboard"
	domain "github.com/taupikpirdian/wlog/domain/dashboard"
)

type storeFunc func(context.Context) (domain.Snapshot, error)

func (f storeFunc) ReadSnapshot(ctx context.Context) (domain.Snapshot, error) { return f(ctx) }

func TestReadOrchestration(t *testing.T) {
	failure := errors.New("store failed")
	for _, tc := range []struct {
		name                                string
		storeErr                            error
		cancelBefore, cancelDuring, badData bool
	}{
		{name: "empty success"}, {name: "storage failure", storeErr: failure}, {name: "already cancelled", cancelBefore: true}, {name: "cancelled during read", cancelDuring: true}, {name: "invalid snapshot", badData: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelBefore {
				cancel()
			}
			reads, clocks := 0, 0
			now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
			svc := application.NewService(storeFunc(func(got context.Context) (domain.Snapshot, error) {
				reads++
				if got != ctx {
					t.Fatal("context changed")
				}
				if tc.cancelDuring {
					cancel()
				}
				snapshot := domain.Snapshot{}
				if tc.badData {
					snapshot.Activities = []domain.Activity{{Type: "INVALID", At: now}}
				}
				return snapshot, tc.storeErr
			}), func() time.Time { clocks++; return now }, time.UTC)
			value, err := svc.Read(ctx)
			switch {
			case tc.cancelBefore:
				if !errors.Is(err, context.Canceled) || reads != 0 || clocks != 0 {
					t.Fatalf("cancel: %v %d %d", err, reads, clocks)
				}
			case tc.cancelDuring:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case tc.storeErr != nil:
				if !errors.Is(err, failure) {
					t.Fatal(err)
				}
			case tc.badData:
				if !errors.Is(err, domain.ErrInvalidData) {
					t.Fatal(err)
				}
			default:
				if err != nil || value.Now != now || value.TotalSeconds != 0 {
					t.Fatalf("view: %+v %v", value, err)
				}
			}
			if !tc.cancelBefore && (reads != 1 || clocks != 1) {
				t.Fatalf("calls: %d %d", reads, clocks)
			}
		})
	}
}
