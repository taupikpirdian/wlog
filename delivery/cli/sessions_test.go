package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	domain "github.com/taupikpirdian/wlog/domain/session"
)

func TestConfirmation(t *testing.T) {
	for _, tc := range []struct {
		name, input                  string
		interactive, confirmed, fail bool
	}{
		{"enter", "\n", true, true, false},
		{"yes", "YES\n", true, true, false},
		{"retry", "maybe\ny\n", true, true, false},
		{"cancel", "no\n", true, false, false},
		{"eof", "", true, false, true},
		{"partial eof", "yes", true, false, true},
		{"piped consent", "yes\n", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetIn(strings.NewReader(tc.input))
			cmd.SetOut(&bytes.Buffer{})
			got, err := confirmSwitch(cmd, domain.Session{StartedAt: time.Now(), TicketKey: "OOT-1"}, "OOT-2", tc.interactive)
			if got != tc.confirmed || (err != nil) != tc.fail {
				t.Fatalf("confirmation=%v error=%v", got, err)
			}
		})
	}
}

type fakeSessions struct {
	startCalls, stopCalls int
	active                *domain.Session
}

func (*fakeSessions) Validate(string, string) error                     { return nil }
func (s *fakeSessions) Active(context.Context) (*domain.Session, error) { return s.active, nil }
func (s *fakeSessions) Start(_ context.Context, key, title string, _ *domain.Session) (domain.Session, error) {
	s.startCalls++
	return domain.Session{TicketKey: key, Title: title, StartedAt: time.Now()}, nil
}
func (s *fakeSessions) Stop(context.Context) (domain.Session, error) {
	s.stopCalls++
	return (domain.Session{Status: domain.Active, StartedAt: time.Now().Add(-time.Minute)}).Complete(time.Now())
}

func TestCommandsAndLazyStorage(t *testing.T) {
	for _, tc := range []struct {
		args                 []string
		opens, starts, stops int
		fail                 bool
	}{
		{[]string{"--help"}, 0, 0, 0, false},
		{[]string{"help", "start"}, 0, 0, 0, false},
		{[]string{"start", "--help"}, 0, 0, 0, false},
		{[]string{"--version"}, 0, 0, 0, false},
		{[]string{"version"}, 0, 0, 0, false},
		{[]string{"s", "OOT-1", "Task"}, 1, 1, 0, false},
		{[]string{"x"}, 1, 0, 1, false},
		{[]string{"s", "OOT-1"}, 0, 0, 0, true},
		{[]string{"x", "unexpected"}, 0, 0, 0, true},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			opens, closes := 0, 0
			service := &fakeSessions{}
			root := NewRootCommand(nil, "test", func(context.Context) (SessionService, func() error, error) {
				opens++
				return service, func() error { closes++; return nil }, nil
			}, nil)
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs(tc.args)
			err := root.Execute()
			if (err != nil) != tc.fail || opens != tc.opens || closes != opens || service.startCalls != tc.starts || service.stopCalls != tc.stops {
				t.Fatalf("error=%v opens=%d closes=%d starts=%d stops=%d", err, opens, closes, service.startCalls, service.stopCalls)
			}
		})
	}
}

func TestNonInteractiveSwitchDoesNotWrite(t *testing.T) {
	service := &fakeSessions{active: &domain.Session{ID: 1, StartedAt: time.Now(), Status: domain.Active}}
	root := NewRootCommand(nil, "test", func(context.Context) (SessionService, func() error, error) {
		return service, func() error { return nil }, nil
	}, nil)
	root.SetArgs([]string{"s", "OOT-2", "Task"})
	root.SetIn(strings.NewReader("yes\n"))
	root.SetOut(&bytes.Buffer{})
	if err := root.Execute(); err == nil || service.startCalls != 0 {
		t.Fatalf("error=%v writes=%d", err, service.startCalls)
	}
}
