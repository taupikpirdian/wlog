package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	application "github.com/taupikpirdian/wlog/application/summary"
	domain "github.com/taupikpirdian/wlog/domain/summary"
)

func TestFormatSummary(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result application.Result
		want   string
	}{
		{"full", application.Result{Day: domain.Day{Seconds: 7200, Details: []string{"Pekerjaan 1", "Pekerjaan 2", "Pekerjaan 3"}}, Email: "developer@example.com"}, "Time:\n2h\n\nDetail:\n- Pekerjaan 1\n- Pekerjaan 2\n- Pekerjaan 3\n\nHasil:\n-\n\nDev By:\ndeveloper@example.com\n"},
		{"empty", application.Result{}, "Time:\n0m\n\nDetail:\n-\n\nHasil:\n-\n\nDev By:\n-\n"},
		{"minutes and duplicates", application.Result{Day: domain.Day{Seconds: 6*3600 + 30*60 + 15, Details: []string{"Fix", "Fix", "  "}}, Email: "  "}, "Time:\n6h 30m\n\nDetail:\n- Fix\n\nHasil:\n-\n\nDev By:\n-\n"},
		{"terminal controls", application.Result{Day: domain.Day{Details: []string{"\x1b[31mFix\x1b[0m"}}, Email: "\x1b[31mdev@example.com\x1b[0m"}, "Time:\n0m\n\nDetail:\n- Fix\n\nHasil:\n-\n\nDev By:\ndev@example.com\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatSummary(tc.result); got != tc.want+"\n### Environment Changes\n\nEnvironment variable check could not be completed.\n" {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}

type summaryStub struct {
	days                []domain.Day
	selected            time.Time
	weekErr, summaryErr error
}

func (s *summaryStub) Tickets(context.Context, time.Time) ([]domain.TicketDay, error) {
	return nil, nil
}
func (s *summaryStub) WeekTickets(context.Context) ([]domain.TicketDay, error) {
	return nil, s.weekErr
}
func (s *summaryStub) TicketDescriptionContext(context.Context, string) (application.TicketAIContext, error) {
	return application.TicketAIContext{}, nil
}
func (s *summaryStub) TicketSummary(ctx context.Context, date time.Time, _ string) (application.Result, error) {
	return s.Summary(ctx, date)
}
func (s *summaryStub) TicketContext(context.Context, time.Time, string) (application.TicketAIContext, error) {
	return application.TicketAIContext{}, nil
}

func (s *summaryStub) Week(context.Context) ([]domain.Day, error) { return s.days, s.weekErr }
func (s *summaryStub) Summary(_ context.Context, date time.Time) (application.Result, error) {
	s.selected = date
	return application.Result{Day: s.days[1], Email: "dev@example.com"}, s.summaryErr
}

func TestSummaryCommandSelectionAndOutput(t *testing.T) {
	dates, _ := domain.WeekDates(time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC), time.UTC)
	s := &summaryStub{}
	for _, date := range dates {
		s.days = append(s.days, domain.Day{Date: date})
	}
	s.days[1].Seconds, s.days[1].HasWorklog, s.days[1].Details = 7200, true, []string{"Fix"}
	closed := false
	cmd := NewSummaryCommand(func(context.Context) (SummaryReader, func() error, error) {
		return s, func() error { closed = true; return nil }, nil
	})
	var out, prompts bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&prompts)
	cmd.SetIn(strings.NewReader("invalid\n0\n8\n2\n"))
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !closed || !s.selected.Equal(dates[1]) {
		t.Fatalf("closed=%v date=%v", closed, s.selected)
	}
	for _, text := range []string{"Mon, 28 Sep 2026    No worklog", "Tue, 29 Sep 2026    2h", "Sun, 04 Oct 2026    No worklog", "Enter a number from 1 to 7."} {
		if !strings.Contains(prompts.String(), text) {
			t.Fatalf("missing %q in %q", text, prompts.String())
		}
	}
	want := "Time:\n2h\n\nDetail:\n- Fix\n\nHasil:\n-\n\nDev By:\ndev@example.com\n"
	if out.String() != want+"\n### Environment Changes\n\nEnvironment variable check could not be completed.\n" {
		t.Fatalf("stdout=%q", out.String())
	}
	if !strings.HasSuffix(prompts.String(), "\nIf wlog is useful for your workflow, consider giving it a ⭐ on GitHub:\nhttps://github.com/taupikpirdian/wlog\n") || strings.Count(prompts.String(), "https://github.com/taupikpirdian/wlog") != 1 {
		t.Fatalf("missing or repeated final CTA: %q", prompts.String())
	}
}

func TestSummaryCommandErrorsAndCleanup(t *testing.T) {
	failure := errors.New("failure")
	for _, tc := range []struct {
		name, input                            string
		openErr, weekErr, summaryErr, closeErr error
		want                                   string
	}{
		{name: "open", openErr: failure, want: "failure"},
		{name: "week", weekErr: failure, want: "failure"},
		{name: "summary", input: "1\n", summaryErr: failure, want: "failure"},
		{name: "close", input: "1\n", closeErr: failure, want: "failure"},
		{name: "EOF", want: "no input"},
		{name: "cancel", input: "q\n", want: "canceled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			closed := false
			s := &summaryStub{days: []domain.Day{{Date: time.Now()}, {Date: time.Now()}}, weekErr: tc.weekErr, summaryErr: tc.summaryErr}
			cmd := NewSummaryCommand(func(context.Context) (SummaryReader, func() error, error) {
				return s, func() error { closed = true; return tc.closeErr }, tc.openErr
			})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetIn(strings.NewReader(tc.input))
			cmd.SetArgs([]string{})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v", err)
			}
			if tc.openErr == nil && !closed {
				t.Fatal("resource not closed")
			}
			if strings.Contains(out.String(), "https://github.com/taupikpirdian/wlog") {
				t.Fatalf("CTA shown after failure: %q", out.String())
			}
		})
	}
}
