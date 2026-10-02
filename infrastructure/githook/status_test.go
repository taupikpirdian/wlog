//go:build unix

package githook

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	application "github.com/taupikpirdian/wlog/application/hook"
	domain "github.com/taupikpirdian/wlog/domain/hook"
)

func TestStatusTracksInstallationWithoutChangingHooks(t *testing.T) {
	dir := newRepository(t)
	ctx := context.Background()
	store := NewStore()
	inspector := NewInspector(dir)
	service := application.NewStatusService(inspector, store)
	location, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	check := func(want string) {
		t.Helper()
		entries, err := os.ReadDir(filepath.Dir(location.HookPath))
		if err != nil {
			t.Fatal(err)
		}
		before, err := store.Read(ctx, location)
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.Status(ctx)
		if err != nil || result.Status != want || result.Location != location {
			t.Fatalf("result=%+v err=%v want=%s", result, err, want)
		}
		after, err := store.Read(ctx, location)
		if err != nil {
			t.Fatal(err)
		}
		if string(before.Target.Content) != string(after.Target.Content) || before.Target.Mode != after.Target.Mode || string(before.Original.Content) != string(after.Original.Content) || before.Original.Mode != after.Original.Mode {
			t.Fatal("status modified hook contents or permissions")
		}
		remaining, err := os.ReadDir(filepath.Dir(location.HookPath))
		if err != nil || len(entries) != len(remaining) {
			t.Fatalf("status created files: %v %v", remaining, err)
		}
	}
	check(application.NotInstalled)
	// An unrelated executable hook is not a wlog installation.
	if err := os.WriteFile(location.HookPath, []byte("#!/bin/sh\necho existing\n"), 0755); err != nil {
		t.Fatal(err)
	}
	check(application.NotInstalled)
	installer := application.NewService(inspector, store)
	if _, err := installer.Install(ctx); err != nil {
		t.Fatal(err)
	}
	check(domain.Installed)
	if err := os.Chmod(location.HookPath, 0600); err != nil {
		t.Fatal(err)
	}
	check(application.NeedsRepair)
	if _, err := installer.Install(ctx); err != nil {
		t.Fatal(err)
	}
	check(domain.Installed)
	// Modifying the original backup invalidates the managed wrapper digest.
	original := filepath.Join(filepath.Dir(location.HookPath), domain.OriginalName)
	if err := os.WriteFile(original, []byte("#!/bin/sh\necho changed\n"), 0755); err != nil {
		t.Fatal(err)
	}
	check(application.Modified)
}

func TestStatusMissingHookDirectoryStaysMissing(t *testing.T) {
	dir := newRepository(t)
	hooks := filepath.Join(dir, ".git", "hooks")
	if err := os.RemoveAll(hooks); err != nil {
		t.Fatal(err)
	}
	result, err := application.NewStatusService(NewInspector(dir), NewStore()).Status(context.Background())
	if err != nil || result.Status != application.NotInstalled {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(hooks); !os.IsNotExist(err) {
		t.Fatalf("status created hook directory: %v", err)
	}
}

func TestStatusCustomPathAndLinkedWorktree(t *testing.T) {
	dir := newRepository(t)
	ctx := context.Background()
	gitCommand(t, dir, "commit", "--quiet", "--allow-empty", "-m", "initial")
	if _, err := application.NewService(NewInspector(dir), NewStore()).Install(ctx); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(canonicalTemp(t), "linked")
	gitCommand(t, dir, "worktree", "add", "--quiet", "-b", "linked", linked)
	sub := filepath.Join(linked, "sub")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	service := application.NewStatusService(NewInspector(sub), NewStore())
	result, err := service.Status(ctx)
	if err != nil || result.Status != domain.Installed || result.Location.Root != linked || !result.Location.Shared {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	gitCommand(t, dir, "config", "core.hooksPath", "/dev/null")
	result, err = service.Status(ctx)
	if err != nil || result.Status != application.CustomHooks {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestStatusRejectsUnsafeHookAndCancellation(t *testing.T) {
	dir := newRepository(t)
	ctx := context.Background()
	inspector := NewInspector(dir)
	location, err := inspector.Inspect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other")
	if err := os.WriteFile(other, []byte("#!/bin/sh\nwl git\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, location.HookPath); err != nil {
		t.Fatal(err)
	}
	service := application.NewStatusService(inspector, NewStore())
	if _, err := service.Status(ctx); !errors.Is(err, domain.ErrUnsafe) && !errors.Is(err, domain.ErrInvalidLocation) {
		t.Fatalf("accepted unsafe hook: %v", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.Status(ctx); err != context.Canceled {
		t.Fatal(err)
	}
}
