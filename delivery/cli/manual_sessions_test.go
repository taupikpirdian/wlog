package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/taupikpirdian/wlog/domain/session"
)

type manualSpy struct {
	calls  int
	active bool
	value  domain.Session
	err    error
}

func TestOrdinaryStartKeepsExistingConfirmationAfterManualRegistration(t *testing.T) {
	for _, existing := range []bool{false, true} {
		service := &fakeSessions{}
		if existing {
			service.active = &domain.Session{ID: 1, Status: domain.Active, StartedAt: time.Now(), TicketKey: "OOT-2"}
		}
		root := NewRootCommand(nil, "test", func(context.Context) (SessionService, func() error, error) {
			return service, func() error { return nil }, nil
		}, nil)
		manualCalls := 0
		AddManualSessionCommands(root, func(context.Context) (ManualSessionService, func() error, error) {
			manualCalls++
			return nil, nil, errors.New("incorrect factory")
		})
		root.SetIn(strings.NewReader("yes\n"))
		root.SetOut(&bytes.Buffer{})
		root.SetArgs([]string{"s", "OOT-1", "Fix tax"})
		err := root.Execute()
		if manualCalls != 0 || existing != (err != nil) || service.startCalls != map[bool]int{false: 1, true: 0}[existing] {
			t.Fatalf("ordinary routing: existing=%t err=%v starts=%d manual=%d", existing, err, service.startCalls, manualCalls)
		}
	}
}

func (s *manualSpy) CreateCompleted(_ context.Context, key, title, from, to string) (domain.Session, error) {
	s.calls++
	if key != "OOT-1" || title != "Fix tax" || from != "09:00" || to != "11:00" {
		return domain.Session{}, errors.New("incorrect completed input")
	}
	return s.value, s.err
}
func (s *manualSpy) StartSince(_ context.Context, key, title, since string) (domain.Session, error) {
	s.calls++
	s.active = true
	if key != "OOT-1" || title != "Fix tax" || since != "09:00" {
		return domain.Session{}, errors.New("incorrect since input")
	}
	return s.value, s.err
}

func TestManualCommandsRouteAndRender(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)
	value, err := domain.NewManualSession(domain.ManualRequest{TicketKey: "OOT-1", Title: "Fix tax", From: "09:00", To: "11:00"}, now, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"session", "OOT-1", "--from", "09:00", "--to", "11:00", "--title", "Fix tax"},
		{"start", "OOT-1", "Fix tax", "--since", "09:00"},
		{"s", "OOT-1", "Fix tax", "-s", "09:00"},
	} {
		spy := &manualSpy{value: value}
		closes := 0
		root := NewRootCommand(nil, "test", nil, nil)
		AddManualSessionCommands(root, func(context.Context) (ManualSessionService, func() error, error) {
			return spy, func() error { closes++; return nil }, nil
		})
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		want := "Duration: 2h"
		if spy.active {
			want = "Started: 09:00"
		}
		if spy.calls != 1 || closes != 1 || !strings.Contains(out.String(), want) {
			t.Fatalf("output/calls: %s %+v %d", out.String(), spy, closes)
		}
	}
}

func TestManualValidationAndHelpStayLazy(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		valid bool
	}{
		{[]string{"session", "--help"}, true}, {[]string{"start", "--help"}, true}, {[]string{"help", "session"}, true}, {[]string{"--version"}, true},
		{[]string{"session"}, false},
		{[]string{"session", "OOT-1", "--from", "09:00", "--to", "11:00"}, false},
		{[]string{"session", "OOT-1", "--from", "9:00", "--to", "11:00", "--title", "Fix tax"}, false},
		{[]string{"session", "OOT-1", "--from", "09:00", "--to", "11:00", "--title", " "}, false},
		{[]string{"session", "OOT-1", "extra", "--from", "09:00", "--to", "11:00", "--title", "Fix tax"}, false},
		{[]string{"s", "OOT-1", "Fix tax", "--since", ""}, false}, {[]string{"s", "OOT-1", "Fix tax", "--since"}, false},
		{[]string{"s", "OOT-1", "Fix tax", "--since", "24:00"}, false}, {[]string{"s", "OOT-1", " ", "--since", "09:00"}, false},
		{[]string{"s", "OOT-1", "--since", "09:00"}, false},
	} {
		root := NewRootCommand(nil, "test", nil, nil)
		calls := 0
		AddManualSessionCommands(root, func(context.Context) (ManualSessionService, func() error, error) {
			calls++
			return nil, nil, errors.New("unexpected dependency")
		})
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs(tc.args)
		err := root.Execute()
		if calls != 0 || tc.valid != (err == nil) {
			t.Fatalf("args %v calls %d err %v", tc.args, calls, err)
		}
	}
}

func TestManualFailuresAndConflictOutput(t *testing.T) {
	failure := errors.New("failed")
	end := time.Date(2026, 10, 2, 11, 0, 0, 0, time.Local)
	seconds := int64(7200)
	value := domain.Session{TicketKey: "OOT-1", Title: "Fix\ntax\x1b[31m", StartedAt: end.Add(-2 * time.Hour), EndedAt: &end, DurationSeconds: &seconds}
	for _, tc := range []struct {
		name                          string
		openErr, serviceErr, closeErr error
		writerFail                    bool
		want                          error
		message                       string
	}{
		{name: "open", openErr: failure, want: failure},
		{name: "service", serviceErr: failure, want: failure},
		{name: "close", closeErr: failure, want: failure},
		{name: "output", writerFail: true, want: failure},
		{name: "active", serviceErr: &domain.ManualConflict{Existing: domain.Session{TicketKey: "OOT-2"}, Active: true}, want: domain.ErrActiveManual, message: "Active session exists: OOT-2"},
		{name: "overlap", serviceErr: &domain.ManualConflict{Existing: domain.Session{ID: 12, TicketKey: "OOT-2", StartedAt: value.StartedAt, EndedAt: &end}}, want: domain.ErrOverlap, message: "session #12"},
		{name: "active overlap", serviceErr: &domain.ManualConflict{Existing: domain.Session{ID: 12, TicketKey: "OOT-2", StartedAt: value.StartedAt}}, want: domain.ErrOverlap, message: "→ active"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := NewRootCommand(nil, "test", nil, nil)
			closes := 0
			AddManualSessionCommands(root, func(context.Context) (ManualSessionService, func() error, error) {
				return &manualSpy{value: value, err: tc.serviceErr}, func() error { closes++; return tc.closeErr }, tc.openErr
			})
			var out bytes.Buffer
			root.SetOut(&out)
			if tc.writerFail {
				root.SetOut(dailyBadWriter{failure})
			}
			root.SetArgs([]string{"session", "OOT-1", "--from", "09:00", "--to", "11:00", "--title", "Fix tax"})
			err := root.Execute()
			if !errors.Is(err, tc.want) || tc.message != "" && !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("err: %v", err)
			}
			if tc.openErr == nil && closes != 1 || tc.openErr != nil && closes != 0 {
				t.Fatal("close count")
			}
			if tc.serviceErr != nil && out.Len() != 0 {
				t.Fatal("partial success")
			}
			if strings.Contains(out.String(), "\x1b") || strings.Contains(out.String(), "Fix\ntax") {
				t.Fatal("unsafe output")
			}
		})
	}
}
