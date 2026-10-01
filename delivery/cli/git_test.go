package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/taupikpirdian/wlog/domain/activity"
)

type fakeCapture struct {
	result activity.CaptureResult
	err    error
}

func (s fakeCapture) Capture(context.Context) (activity.CaptureResult, error) { return s.result, s.err }

func TestGitOutputAndErrors(t *testing.T) {
	id := int64(12)
	for _, tc := range []struct {
		name            string
		result          activity.CaptureResult
		cause           error
		output, warning string
	}{
		{"matching", activity.CaptureResult{Attribution: activity.Attribution{TicketKey: "OOT-1", SessionID: &id}}, nil, "✓ Commit captured under OOT-1.\n", ""},
		{"without session", activity.CaptureResult{Attribution: activity.Attribution{TicketKey: "OOT-1"}}, nil, "✓ Commit captured under OOT-1 (UNSESSIONED).\n", ""},
		{"mismatch", activity.CaptureResult{Attribution: activity.Attribution{TicketKey: "OOT-2", ActiveTicket: "OOT-1"}, Warnings: []string{"ticket_mismatch"}}, nil, "✓ Commit captured under OOT-2 (UNSESSIONED).\n", "Active session : OOT-1"},
		{"unassigned", activity.CaptureResult{Warnings: []string{"unassigned"}}, nil, "✓ Commit captured (UNASSIGNED).\n", "No ticket detected"},
		{"duplicate", activity.CaptureResult{AlreadyCaptured: true}, nil, "✓ Commit already captured.\n", ""},
		{"partial", activity.CaptureResult{Warnings: []string{"message_unavailable"}}, nil, "✓ Commit captured (UNASSIGNED).\n", "message_unavailable"},
		{"failure", activity.CaptureResult{}, errors.New("storage failed"), "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			closed := false
			root := NewRootCommand(nil, "test", nil, nil, func(context.Context) (GitCaptureService, func() error, error) {
				return fakeCapture{tc.result, tc.cause}, func() error { closed = true; return nil }, nil
			})
			var out, stderr bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&stderr)
			root.SetArgs([]string{"git"})
			err := root.Execute()
			if !errors.Is(err, tc.cause) || !closed || out.String() != tc.output || (tc.warning != "" && !strings.Contains(stderr.String(), tc.warning)) {
				t.Fatalf("error=%v out=%q stderr=%q closed=%v", err, out.String(), stderr.String(), closed)
			}
		})
	}
}

func TestGitHelpAndArgsAreLazy(t *testing.T) {
	for _, args := range [][]string{{"git", "--help"}, {"help", "git"}, {"git", "unexpected"}, {"--help"}, {"--version"}} {
		root := NewRootCommand(nil, "test", nil, nil, func(context.Context) (GitCaptureService, func() error, error) {
			t.Fatal("opened dependency")
			return nil, nil, nil
		})
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		err := root.Execute()
		if (err != nil) != (len(args) == 2 && args[1] == "unexpected") {
			t.Fatalf("args=%v error=%v", args, err)
		}
	}
}
