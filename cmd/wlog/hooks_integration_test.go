//go:build unix

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	applicationactivity "github.com/taupikpirdian/wlog/application/activity"
	applicationhook "github.com/taupikpirdian/wlog/application/hook"
	"github.com/taupikpirdian/wlog/delivery/cli"
	domainhook "github.com/taupikpirdian/wlog/domain/hook"
	"github.com/taupikpirdian/wlog/domain/ticket"
	"github.com/taupikpirdian/wlog/infrastructure/gitcapture"
	"github.com/taupikpirdian/wlog/infrastructure/githook"
	"github.com/taupikpirdian/wlog/infrastructure/storage"
)

// The hook executes this test binary as a CLI using an isolated database.
// It exercises the real Git reader and capture service without touching ~/.worklog.
func TestMain(m *testing.M) {
	if path := os.Getenv("WLOG_HOOK_FIXTURE_DATABASE"); path != "" {
		root := cli.NewRootCommand(nil, "test", nil, nil, func(ctx context.Context) (cli.GitCaptureService, func() error, error) {
			db, err := storage.OpenDatabase(ctx, path)
			if err != nil {
				return nil, nil, err
			}
			pattern, err := ticket.NewKeyPattern(`[A-Z][A-Z0-9]+-[0-9]+`)
			if err != nil {
				_ = db.Close()
				return nil, nil, err
			}
			dir, err := os.Getwd()
			if err != nil {
				_ = db.Close()
				return nil, nil, err
			}
			return applicationactivity.NewCaptureService(gitcapture.NewReader(dir), storage.NewSQLiteActivityStore(db), pattern, applicationactivity.CaptureOptions{ChangedFiles: true, DiffStat: true}, time.Now), db.Close, nil
		})
		if err := root.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestCommitsCapturedFromTerminalAndEditorPATH(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"terminal", "editor"} {
		t.Run(mode, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			fixtureDatabase := filepath.Join(t.TempDir(), "worklog.db")
			t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "no-global-config"))
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			run := func(args ...string) string {
				t.Helper()
				cmd := exec.Command(git, append([]string{"-C", dir}, args...)...)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v %s", args, err, out)
				}
				return string(out)
			}
			run("init", "--quiet")
			run("config", "user.name", "Hook Fixture")
			run("config", "user.email", "fixture@example.invalid")
			run("config", "commit.gpgsign", "false")
			ctx := context.Background()
			inspector := githook.NewInspector(dir)
			store := githook.NewStore()
			// Upgrade a legacy hook and preserve an existing executable hook.
			original := []byte("#!/bin/sh\nprintf original > hook-original-ran\n")
			hookPath := filepath.Join(dir, ".git", "hooks", "post-commit")
			if err := os.WriteFile(hookPath, original, 0755); err != nil {
				t.Fatal(err)
			}
			if _, err := applicationhook.NewService(inspector, store).Install(ctx); err != nil {
				t.Fatal(err)
			}
			installer := applicationhook.NewServiceWithExecutable(inspector, store, executable)
			result, err := installer.Install(ctx)
			if err != nil || result.Status != domainhook.Updated {
				t.Fatalf("upgrade=%+v err=%v", result, err)
			}
			if result, err = installer.Install(ctx); err != nil || result.Status != domainhook.AlreadyInstalled {
				t.Fatalf("repeat=%+v err=%v", result, err)
			}
			backup, err := os.ReadFile(filepath.Join(filepath.Dir(hookPath), domainhook.OriginalName))
			if err != nil || string(backup) != string(original) {
				t.Fatalf("backup=%q err=%v", backup, err)
			}
			t.Setenv("WLOG_HOOK_FIXTURE_DATABASE", fixtureDatabase)
			localBin := filepath.Join(dir, ".local", "bin")
			if err := os.MkdirAll(localBin, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(executable, filepath.Join(localBin, "wl")); err != nil {
				t.Fatal(err)
			}
			// A GUI Git process typically inherits a PATH without the user's wl directory.
			path := filepath.Dir(git) + ":/usr/bin:/bin"
			if mode == "terminal" {
				path = localBin + ":" + path
			}
			t.Setenv("PATH", path)
			if err := os.WriteFile(filepath.Join(dir, "change.txt"), []byte("fixture\n"), 0600); err != nil {
				t.Fatal(err)
			}
			run("add", "change.txt")
			run("commit", "--quiet", "-m", "OOT-123 capture from "+mode)
			if _, err := os.Stat(filepath.Join(dir, "hook-original-ran")); err != nil {
				t.Fatalf("original hook did not run: %v", err)
			}
			db, err := storage.OpenDatabase(ctx, fixtureDatabase)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var count int
			var key, repository string
			if err := db.QueryRow(`SELECT count(*),t.ticket_key,a.repository FROM work_activities a JOIN tickets t ON t.id=a.ticket_id WHERE a.type='GIT_COMMIT'`).Scan(&count, &key, &repository); err != nil {
				t.Fatal(err)
			}
			if count != 1 || key != "OOT-123" || repository != dir {
				t.Fatalf("capture count=%d ticket=%s repository=%s", count, key, repository)
			}
		})
	}
}
