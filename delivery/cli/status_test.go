package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	application "github.com/taupikpirdian/wlog/application/hook"
	dashboard "github.com/taupikpirdian/wlog/domain/dashboard"
	hook "github.com/taupikpirdian/wlog/domain/hook"
)

type hookStatusFunc func(context.Context) (application.Result, error)

func (f hookStatusFunc) Status(ctx context.Context) (application.Result, error) { return f(ctx) }

func TestStatusShowsDashboardAndCurrentRepositoryHook(t *testing.T) {
	for _, tc := range []struct{ state, want string }{
		{hook.Installed, "Git hook: installed (automatic commit capture)"},
		{application.NotInstalled, "Run wl install-hooks in this repository to enable"},
		{application.NeedsRepair, "repair executable permissions"},
		{application.Modified, "automatic capture could not be verified"},
		{application.CustomHooks, "custom core.hooksPath"},
	} {
		t.Run(tc.state, func(t *testing.T) {
			closes := 0
			factory := func(context.Context) (DashboardReader, func() error, error) {
				return dailyReaderFunc(func(context.Context) (dashboard.View, error) { return dailyView(t), nil }), func() error { closes++; return nil }, nil
			}
			root := NewRootCommand(factory, "test", nil, nil)
			root.AddCommand(NewStatusCommand(factory, func(context.Context) (HookStatusReader, error) {
				return hookStatusFunc(func(context.Context) (application.Result, error) {
					return application.Result{Status: tc.state, Location: hook.Location{Root: "/work/repo\n\x1b[31m", HookPath: "/work/repo/.git/hooks/post-commit", Shared: true}}, nil
				}), nil
			}))
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetArgs([]string{"status"})
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"DEV WORKLOG", "OOT-3668 — Support QA", tc.want, "Repository: /work/repo ", "Hook: /work/repo/.git/hooks/post-commit", "Scope: all worktrees"} {
				if !strings.Contains(out.String(), want) {
					t.Fatalf("missing %q: %s", want, out.String())
				}
			}
			if closes != 1 || strings.ContainsRune(out.String(), '\x1b') {
				t.Fatalf("closes=%d output=%q", closes, out.String())
			}
		})
	}
}

func TestStatusHelpAndInvalidArgsRemainLazy(t *testing.T) {
	for _, args := range [][]string{{"status", "--help"}, {"help", "status"}, {"status", "unexpected"}, {"status", "--unknown"}} {
		factory := func(context.Context) (DashboardReader, func() error, error) {
			t.Fatal("opened dashboard")
			return nil, nil, nil
		}
		root := NewRootCommand(factory, "test", nil, nil)
		root.AddCommand(NewStatusCommand(factory, func(context.Context) (HookStatusReader, error) { t.Fatal("inspected hooks"); return nil, nil }))
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs(args)
		err := root.Execute()
		if (args[1] == "unexpected" || args[1] == "--unknown") != (err != nil) {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestStatusUnavailableHooksStillShowsWork(t *testing.T) {
	factory := func(context.Context) (DashboardReader, func() error, error) {
		return dailyReaderFunc(func(context.Context) (dashboard.View, error) { return dailyView(t), nil }), func() error { return nil }, nil
	}
	root := NewRootCommand(factory, "test", nil, nil)
	root.AddCommand(NewStatusCommand(factory, func(context.Context) (HookStatusReader, error) {
		return hookStatusFunc(func(context.Context) (application.Result, error) {
			return application.Result{}, errors.New("not a Git worktree\n\x1b[31m")
		}), nil
	}))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"status"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "DEV WORKLOG") || !strings.Contains(out.String(), "Git hook: unavailable (not a Git worktree )") || strings.ContainsRune(out.String(), '\x1b') {
		t.Fatal(out.String())
	}
}
