package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	application "github.com/taupikpirdian/wlog/application/hook"
	domain "github.com/taupikpirdian/wlog/domain/hook"
)

type fakeHookInstaller struct {
	result application.Result
	err    error
}

func (f fakeHookInstaller) Install(context.Context) (application.Result, error) {
	return f.result, f.err
}

func TestInstallHooksAliasesAndOutcomes(t *testing.T) {
	for _, alias := range []string{"install-hooks", "install-hook"} {
		for _, status := range []string{domain.Installed, domain.AlreadyInstalled, domain.Repaired, domain.Updated} {
			t.Run(alias+"/"+status, func(t *testing.T) {
				root := NewRootCommand(nil, "test", nil, nil)
				root.AddCommand(NewInstallHooksCommand(func(context.Context) (HookInstaller, error) {
					return fakeHookInstaller{result: application.Result{Status: status, HasOriginal: true, Location: domain.Location{Root: "/fixture", HookPath: "/fixture/.git/hooks/post-commit", Shared: true}}}, nil
				}))
				var out bytes.Buffer
				root.SetOut(&out)
				root.SetErr(&out)
				root.SetArgs([]string{alias})
				if err := root.Execute(); err != nil {
					t.Fatal(err)
				}
				for _, want := range []string{"Repository: /fixture", "Hook: /fixture/.git/hooks/post-commit", domain.OriginalName, "manual integration", "all worktrees", "PATH"} {
					if !strings.Contains(out.String(), want) {
						t.Fatalf("output=%q missing=%q", out.String(), want)
					}
				}
			})
		}
	}
}

func TestInstallHooksLazyDependencies(t *testing.T) {
	for _, args := range [][]string{{"install-hooks", "--help"}, {"help", "install-hook"}, {"install-hooks", "unexpected"}, {"install-hook", "--unknown"}, {"--help"}, {"--version"}} {
		root := NewRootCommand(nil, "test", nil, nil)
		root.AddCommand(NewInstallHooksCommand(func(context.Context) (HookInstaller, error) { t.Fatal("opened dependencies"); return nil, nil }))
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		err := root.Execute()
		invalid := len(args) == 2 && (args[1] == "unexpected" || args[1] == "--unknown")
		if (err != nil) != invalid {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

type hookFailedWriter struct{ cause error }

func (w hookFailedWriter) Write([]byte) (int, error) { return 0, w.cause }

func TestInstallHooksErrors(t *testing.T) {
	cause := errors.New("fixture failure")
	for _, tc := range []struct {
		name         string
		factory      HookFactory
		writeFailure bool
		want         error
		message      string
	}{
		{name: "missing factory", message: "unavailable"},
		{name: "factory error", factory: func(context.Context) (HookInstaller, error) { return nil, cause }, want: cause},
		{name: "service error", factory: func(context.Context) (HookInstaller, error) { return fakeHookInstaller{err: cause}, nil }, want: cause},
		{name: "unknown outcome", factory: func(context.Context) (HookInstaller, error) { return fakeHookInstaller{}, nil }, message: "unknown outcome"},
		{name: "output failure", factory: func(context.Context) (HookInstaller, error) {
			return fakeHookInstaller{result: application.Result{Status: domain.Installed}}, nil
		}, writeFailure: true, want: cause, message: "may already be installed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := NewRootCommand(nil, "test", nil, nil)
			root.AddCommand(NewInstallHooksCommand(tc.factory))
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			if tc.writeFailure {
				root.SetOut(hookFailedWriter{cause})
			}
			root.SetArgs([]string{"install-hooks"})
			err := root.Execute()
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
