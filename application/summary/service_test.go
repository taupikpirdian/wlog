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

type store struct {
	snapshot dashboard.Snapshot
	err      error
	calls    int
}

func (s *store) ReadSnapshot(ctx context.Context) (dashboard.Snapshot, error) {
	s.calls++
	if err := ctx.Err(); err != nil {
		return dashboard.Snapshot{}, err
	}
	return s.snapshot, s.err
}

type emailReader struct {
	value string
	err   error
	calls int
}

func (e *emailReader) Email(context.Context) (string, error) { e.calls++; return e.value, e.err }

func TestServiceWeekAndFreshSummary(t *testing.T) {
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	s := &store{}
	e := &emailReader{value: "developer@example.com"}
	reader := application.NewService(s, e, func() time.Time { return now }, time.UTC)
	days, err := reader.Week(context.Background())
	if err != nil || len(days) != 7 || s.calls != 1 || e.calls != 0 {
		t.Fatalf("days=%v err=%v store=%d email=%d", days, err, s.calls, e.calls)
	}
	for _, day := range days {
		if day.HasWorklog || day.Seconds != 0 {
			t.Fatalf("empty day=%+v", day)
		}
	}
	// Work recorded after the menu is read must appear in the selected summary.
	s.snapshot.Sessions = []session.Session{{Title: "Investigate", StartedAt: now.Add(-time.Hour), Status: session.Active}}
	result, err := reader.Summary(context.Background(), days[4].Date)
	if err != nil || result.Day.Seconds != 3600 || result.Email != e.value || len(result.Day.Details) != 1 || s.calls != 2 || e.calls != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestServiceErrors(t *testing.T) {
	failure := errors.New("read failed")
	for _, tc := range []struct {
		name               string
		storeErr, emailErr error
	}{
		{"store", failure, nil}, {"email", nil, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := application.NewService(&store{err: tc.storeErr}, &emailReader{err: tc.emailErr}, time.Now, time.UTC)
			if _, err := reader.Summary(context.Background(), time.Now()); !errors.Is(err, failure) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	reader := application.NewService(&store{err: failure}, &emailReader{}, time.Now, time.UTC)
	if _, err := reader.Week(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("err=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.Week(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	reader = application.NewService(&store{}, &emailReader{}, time.Now, nil)
	if _, err := reader.Week(context.Background()); err == nil {
		t.Fatal("missing timezone accepted")
	}
}
