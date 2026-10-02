package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/taupikpirdian/wlog/domain/dashboard"
	"github.com/taupikpirdian/wlog/domain/session"
)

type dailyReaderFunc func(context.Context) (domain.View, error)

func (f dailyReaderFunc) Read(ctx context.Context) (domain.View, error) { return f(ctx) }

func dailyView(t *testing.T) domain.View {
	t.Helper()
	loc := time.FixedZone("Jakarta", 7*3600)
	at := time.Date(2026, 10, 2, 13, 30, 0, 0, loc)
	start := at.Add(-30 * time.Minute)
	end := time.Date(2026, 10, 2, 11, 30, 0, 0, loc)
	sid := int64(1)
	value, err := domain.Build(domain.Snapshot{Sessions: []session.Session{
		{ID: 1, TicketKey: "OOT-3751", Title: "Fix tax calculation", StartedAt: end.Add(-150 * time.Minute), EndedAt: &end, Status: session.Completed},
		{ID: 2, TicketKey: "OOT-3668", Title: "Support QA", StartedAt: start, Status: session.Active},
	}, Activities: []domain.Activity{
		{ID: 1, TicketKey: "OOT-3751", SessionID: &sid, Type: "NOTE", Text: "Check tax calculation", At: end.Add(-135 * time.Minute)},
		{ID: 2, TicketKey: "OOT-3751", SessionID: &sid, Type: "GIT_COMMIT", Text: "fix tax calculation", Hash: "abc1234567", At: end.Add(-78 * time.Minute)},
		{ID: 3, TicketKey: "OOT-9", Type: "GIT_COMMIT", Text: "outside session", At: at},
		{ID: 4, Type: "GIT_COMMIT", At: at},
	}}, at, loc)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestDailyCommandsRenderViewsAndClose(t *testing.T) {
	value := dailyView(t)
	for _, tc := range []struct {
		args   []string
		want   []string
		absent string
	}{
		{nil, []string{"DEV WORKLOG", "OOT-3668 — Support QA", "Started : 13:00", "Duration: 30m", "OOT-3751     2h 30m", "Total        3h", "Unsessioned", "Unassigned"}, "wl ready"},
		{[]string{"today"}, []string{"02 Oct 2026", "09:00  START   OOT-3751", "09:15  NOTE    OOT-3751  Check tax calculation", "10:12  COMMIT  OOT-3751  fix tax calculation [abc1234]", "11:30  STOP    OOT-3751", "13:00  START   OOT-3668", "Total tracked: 3h"}, "STOP    OOT-3668"},
	} {
		reads, closes := 0, 0
		root := NewRootCommand(func(context.Context) (DashboardReader, func() error, error) {
			return dailyReaderFunc(func(context.Context) (domain.View, error) { reads++; return value, nil }), func() error { closes++; return nil }, nil
		}, "test", nil, nil)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs(tc.args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		for _, want := range tc.want {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("missing %q:\n%s", want, out.String())
			}
		}
		if strings.Contains(out.String(), tc.absent) || reads != 1 || closes != 1 {
			t.Fatalf("output/calls: %s %d %d", out.String(), reads, closes)
		}
	}
}

func TestDailyCommandsEmptyOvernightAndDurationFormatting(t *testing.T) {
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	value, err := domain.Build(domain.Snapshot{}, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if got := renderDashboard(value); !strings.Contains(got, "No active session") || !strings.Contains(got, "No tracked sessions today") || !strings.Contains(got, "Total        0m") {
		t.Fatal(got)
	}
	if got := renderTimeline(value); !strings.Contains(got, "No activities today") {
		t.Fatal(got)
	}
	value, err = domain.Build(domain.Snapshot{Sessions: []session.Session{{ID: 1, TicketKey: "OOT-1", Title: "overnight", StartedAt: now.Add(-2 * time.Hour), Status: session.Active}}}, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if got := renderDashboard(value); !strings.Contains(got, "Started : 2026-10-01 23:00") || !strings.Contains(got, "Duration: 2h") || !strings.Contains(got, "Total        1h") {
		t.Fatal(got)
	}
	for _, tc := range []struct {
		seconds int64
		want    string
	}{{0, "0m"}, {40, "0m"}, {80, "1m"}, {2700, "45m"}, {3600, "1h"}, {6120, "1h 42m"}, {90000, "25h"}} {
		if got := dailyDuration(tc.seconds); got != tc.want {
			t.Fatalf("duration %d: %s", tc.seconds, got)
		}
	}
}

func TestDailyHelpAndInvalidArgsRemainLazy(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"help"}, {"--version"}, {"version"}, {"today", "--help"}, {"help", "today"}, {"today", "unexpected"}, {"unknown"}, {"--unknown"}, {"today", "--unknown"}} {
		calls := 0
		root := NewRootCommand(func(context.Context) (DashboardReader, func() error, error) {
			calls++
			return nil, nil, errors.New("must remain lazy")
		}, "test", nil, nil)
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs(args)
		err := root.Execute()
		if calls != 0 {
			t.Fatalf("factory called: %v", args)
		}
		invalid := args[0] == "unknown" || args[0] == "--unknown" || (len(args) > 1 && (args[1] == "unexpected" || args[1] == "--unknown"))
		if invalid != (err != nil) {
			t.Fatalf("args %v error %v", args, err)
		}
	}
}

type dailyBadWriter struct{ err error }

func (w dailyBadWriter) Write([]byte) (int, error) { return 0, w.err }

func TestDailyFailuresCloseAndAvoidPartialSuccess(t *testing.T) {
	failure := errors.New("fixture failure")
	closeFailure := errors.New("close failure")
	for _, tc := range []struct {
		name                       string
		openErr, readErr, closeErr error
		writeFail                  bool
		closes                     int
		want                       error
	}{
		{name: "open", openErr: failure, want: failure},
		{name: "read", readErr: failure, closes: 1, want: failure},
		{name: "close", closeErr: closeFailure, closes: 1, want: closeFailure},
		{name: "output", writeFail: true, closes: 1, want: failure},
		{name: "read and close", readErr: failure, closeErr: closeFailure, closes: 1, want: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, args := range [][]string{nil, {"today"}} {
				closes := 0
				root := NewRootCommand(func(context.Context) (DashboardReader, func() error, error) {
					return dailyReaderFunc(func(context.Context) (domain.View, error) {
						v, _ := domain.Build(domain.Snapshot{}, time.Now(), time.UTC)
						return v, tc.readErr
					}), func() error { closes++; return tc.closeErr }, tc.openErr
				}, "test", nil, nil)
				var out bytes.Buffer
				root.SetOut(&out)
				if tc.writeFail {
					root.SetOut(dailyBadWriter{failure})
				}
				root.SetArgs(args)
				err := root.Execute()
				if !errors.Is(err, tc.want) || closes != tc.closes {
					t.Fatalf("err %v closes %d", err, closes)
				}
				if tc.readErr != nil && out.Len() != 0 {
					t.Fatalf("partial success: %s", out.String())
				}
				if tc.closeErr != nil && !errors.Is(err, tc.closeErr) {
					t.Fatal("close failure lost")
				}
			}
		})
	}
}

func TestSafeEvidenceText(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"Check\n税\tcalculation\r\x00", "Check 税 calculation "},
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b]8;;https://example.invalid\aURL\x1b]8;;\x1b\\", "URL"},
		{"\x1bPignored\x1b\\safe", "safe"},
		{"\u009b31mred\u009b0m", "red"},
		{"\u009dignored\u009csafe", "safe"},
		{"hello\x1b", "hello"},
		{"\x1b(BCHECK\u202e", "CHECK"},
		{"hello\x1b[31", "hello"},
	} {
		if got := safeText(tc.input); got != tc.want {
			t.Fatalf("safeText(%q)=%q want %q", tc.input, got, tc.want)
		}
	}
}
