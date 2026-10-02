//go:build unix

package githook

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	domain "github.com/taupikpirdian/wlog/domain/hook"
)

func hookLocation(t *testing.T) domain.Location {
	t.Helper()
	dir := filepath.Join(canonicalTemp(t), "repo with spaces and 'quotes'")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return domain.Location{Root: dir, HookPath: filepath.Join(dir, "hooks", "post-commit")}
}

func writeFixture(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != body || info.Mode().Perm() != mode {
		t.Fatalf("file %s content=%q mode=%v", path, content, info.Mode())
	}
}

func assertClean(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == domain.LockName || strings.HasPrefix(entry.Name(), ".wlog-hook-") {
			t.Fatalf("leftover %s", entry.Name())
		}
	}
}

func TestStoreInstallRepeatAndRepair(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "preserve"}[existing], func(t *testing.T) {
			location := hookLocation(t)
			original := "#!/bin/sh\nprintf 'original output\\n'\nexit 7\n"
			if existing {
				writeFixture(t, location.HookPath, original, 0751)
			}
			plan, err := NewStore().Apply(context.Background(), location, domain.PlanInstall)
			if err != nil || plan.Status != domain.Installed || plan.HasOriginal != existing {
				t.Fatalf("plan=%+v err=%v", plan, err)
			}
			if existing {
				assertFile(t, filepath.Join(filepath.Dir(location.HookPath), domain.OriginalName), original, 0751)
			}
			before, _ := readFile(location.HookPath)
			plan, err = NewStore().Apply(context.Background(), location, domain.PlanInstall)
			after, _ := readFile(location.HookPath)
			if err != nil || plan.Status != domain.AlreadyInstalled || !sameFile(before, after) {
				t.Fatalf("rerun=%+v err=%v", plan, err)
			}
			if err := os.Chmod(location.HookPath, 0600); err != nil {
				t.Fatal(err)
			}
			plan, err = NewStore().Apply(context.Background(), location, domain.PlanInstall)
			if err != nil || plan.Status != domain.Repaired {
				t.Fatalf("repair=%+v err=%v", plan, err)
			}
			assertFile(t, location.HookPath, string(before.value.Content), 0700)
			assertClean(t, filepath.Dir(location.HookPath))
		})
	}
}

func TestWrapperPreservesOriginalAndIgnoresCaptureFailure(t *testing.T) {
	for _, tc := range []struct {
		name, original string
		mode           os.FileMode
		missing        bool
		status         int
		output         string
	}{
		{"no original", "", 0, false, 0, ""},
		{"missing wl", "", 0, true, 0, ""},
		{"original failure", "#!/bin/sh\nprintf 'original output\\n'\nexit 7\n", 0755, false, 7, "original output\n"},
		{"set e", "#!/bin/sh\nset -e\nfalse\nprintf 'not reached'\n", 0755, false, 1, ""},
		{"nonexecutable", "#!/bin/sh\nprintf 'not executed'\nexit 9\n", 0640, false, 0, ""},
		{"arguments stdin and environment", "#!/bin/sh\nprintf 'arg=%s ' \"$1\"\nread value\nprintf 'stdin=%s\\n' \"$value\"\ncd /\nexport WLOG_TEST_MUTATION=changed\n", 0755, false, 0, "arg=argument with spaces stdin=input text\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			location := hookLocation(t)
			if tc.original != "" {
				writeFixture(t, location.HookPath, tc.original, tc.mode)
			}
			if _, err := NewStore().Apply(context.Background(), location, domain.PlanInstall); err != nil {
				t.Fatal(err)
			}
			bin := canonicalTemp(t)
			spy := filepath.Join(bin, "calls")
			if !tc.missing {
				writeFixture(t, filepath.Join(bin, "wl"), "#!/bin/sh\nprintf '%s|%s|%s\\n' \"$PWD\" \"$1\" \"${WLOG_TEST_MUTATION-unset}\" >> \"$WLOG_TEST_SPY\"\necho suppressed stdout\necho suppressed stderr >&2\nexit 42\n", 0755)
			}
			cmd := exec.Command(location.HookPath, "argument with spaces")
			cmd.Dir = location.Root
			cmd.Env = append(os.Environ(), "PATH="+bin, "WLOG_TEST_SPY="+spy, "WLOG_TEST_MUTATION=unchanged")
			cmd.Stdin = strings.NewReader("input text\n")
			out, err := cmd.CombinedOutput()
			status := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				status = exit.ExitCode()
			}
			if status != tc.status || string(out) != tc.output {
				t.Fatalf("status=%d out=%q err=%v", status, out, err)
			}
			if !tc.missing {
				assertFile(t, spy, location.Root+"|git|unchanged\n", 0644)
			}
			assertClean(t, filepath.Dir(location.HookPath))
		})
	}
}

func TestStoreConflictsAndUnsafePaths(t *testing.T) {
	for _, tc := range []struct {
		name, file, kind string
		cause            error
	}{
		{"target symlink", "post-commit", "symlink", domain.ErrUnsafe},
		{"backup symlink", domain.OriginalName, "symlink", domain.ErrUnsafe},
		{"lock symlink", domain.LockName, "symlink", domain.ErrUnsafe},
		{"target directory", "post-commit", "directory", domain.ErrUnsafe},
		{"backup collision", domain.OriginalName, "regular", domain.ErrConflict},
		{"busy", domain.LockName, "regular", domain.ErrBusy},
		{"modified wrapper", "post-commit", "marker", domain.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			location := hookLocation(t)
			dir := filepath.Dir(location.HookPath)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, tc.file)
			victim := filepath.Join(location.Root, "victim")
			writeFixture(t, victim, "untouched", 0600)
			switch tc.kind {
			case "symlink":
				if err := os.Symlink(victim, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "marker":
				writeFixture(t, path, "# wlog-managed-post-commit modified", 0755)
			default:
				writeFixture(t, path, "existing", 0600)
			}
			_, err := NewStore().Apply(context.Background(), location, domain.PlanInstall)
			if !errors.Is(err, tc.cause) {
				t.Fatalf("error=%v want %v", err, tc.cause)
			}
			assertFile(t, victim, "untouched", 0600)
		})
	}
}

func TestStorePrepublicationFailureAndExternalChanges(t *testing.T) {
	for _, change := range []string{"failure", "cancel", "target", "backup", "lock", "staged", "busy"} {
		t.Run(change, func(t *testing.T) {
			location := hookLocation(t)
			original := "#!/bin/sh\nexit 0\n"
			writeFixture(t, location.HookPath, original, 0755)
			dir := filepath.Dir(location.HookPath)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("injected failure")
			store := NewStore()
			var changedPath string
			store.checkpoint = func(stage string) error {
				if stage != "before-publish" {
					return nil
				}
				switch change {
				case "cancel":
					cancel()
					return nil
				case "target":
					changedPath = location.HookPath
				case "backup":
					changedPath = filepath.Join(dir, domain.OriginalName)
				case "lock":
					changedPath = filepath.Join(dir, domain.LockName)
				case "staged":
					paths, _ := filepath.Glob(filepath.Join(dir, ".wlog-hook-*"))
					for _, path := range paths {
						data, _ := os.ReadFile(path)
						if bytes.Contains(data, []byte("wlog-managed-post-commit")) {
							changedPath = path
						}
					}
				case "busy":
					if _, err := NewStore().Apply(context.Background(), location, domain.PlanInstall); !errors.Is(err, domain.ErrBusy) {
						t.Fatalf("concurrent=%v", err)
					}
				}
				if changedPath != "" {
					writeFixture(t, changedPath, "external change", 0600)
				}
				if change == "target" || change == "backup" || change == "staged" {
					return nil
				}
				return failure
			}
			_, err := store.Apply(ctx, location, domain.PlanInstall)
			if err == nil {
				t.Fatal("expected failure")
			}
			if change == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if changedPath != "" {
				assertFile(t, changedPath, "external change", 0600)
			}
			if change != "target" {
				assertFile(t, location.HookPath, original, 0755)
			}
			if change != "backup" {
				if _, err := os.Lstat(filepath.Join(dir, domain.OriginalName)); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("backup not cleaned: %v", err)
				}
			}
			// A changed backup shares its inode with the staging copy; neither is owned anymore.
			if change != "lock" && change != "staged" && change != "backup" {
				assertClean(t, dir)
			}
			if change == "backup" || change == "lock" || change == "staged" {
				if !strings.Contains(err.Error(), "inspect recovery path") {
					t.Fatalf("missing recovery guidance: %v", err)
				}
			}
		})
	}
}

func TestStorePostpublicationFailureRerun(t *testing.T) {
	location := hookLocation(t)
	writeFixture(t, location.HookPath, "#!/bin/sh\nexit 0\n", 0755)
	failure := errors.New("directory sync failed")
	store := NewStore()
	store.checkpoint = func(stage string) error {
		if stage == "published" {
			return failure
		}
		return nil
	}
	_, err := store.Apply(context.Background(), location, domain.PlanInstall)
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "may already be installed") {
		t.Fatal(err)
	}
	plan, err := NewStore().Apply(context.Background(), location, domain.PlanInstall)
	if err != nil || plan.Status != domain.AlreadyInstalled {
		t.Fatalf("rerun=%+v err=%v", plan, err)
	}
	assertClean(t, filepath.Dir(location.HookPath))
}

func TestStoreBackupPublishNeverClobbersConcurrentFile(t *testing.T) {
	location := hookLocation(t)
	original := "#!/bin/sh\nexit 0\n"
	writeFixture(t, location.HookPath, original, 0755)
	backup := filepath.Join(filepath.Dir(location.HookPath), domain.OriginalName)
	_, err := NewStore().Apply(context.Background(), location, func(snapshot domain.Snapshot) (domain.Plan, error) {
		plan, err := domain.PlanInstall(snapshot)
		writeFixture(t, backup, "another installer or editor", 0600)
		return plan, err
	})
	if err == nil {
		t.Fatal("overwrote concurrent backup")
	}
	assertFile(t, location.HookPath, original, 0755)
	assertFile(t, backup, "another installer or editor", 0600)
	assertClean(t, filepath.Dir(location.HookPath))
}

func TestStoreRejectsSymlinkDirectoryAndCancelledContext(t *testing.T) {
	location := hookLocation(t)
	victim := canonicalTemp(t)
	if err := os.Symlink(victim, filepath.Dir(location.HookPath)); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore().Apply(context.Background(), location, domain.PlanInstall); !errors.Is(err, domain.ErrUnsafe) {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(victim)
	if err != nil || len(entries) != 0 {
		t.Fatalf("victim changed: %v %v", entries, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewStore().Apply(ctx, location, domain.PlanInstall); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestRealCommitsFromSharedHookIgnoreCaptureFailure(t *testing.T) {
	dir := newRepository(t)
	gitCommand(t, dir, "commit", "--quiet", "--allow-empty", "-m", "initial")
	linked := filepath.Join(canonicalTemp(t), "linked")
	gitCommand(t, dir, "worktree", "add", "--quiet", "-b", "linked", linked)
	location, err := NewInspector(linked).Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore().Apply(context.Background(), location, domain.PlanInstall); err != nil {
		t.Fatal(err)
	}
	bin := canonicalTemp(t)
	spy := filepath.Join(bin, "calls")
	writeFixture(t, filepath.Join(bin, "wl"), "#!/bin/sh\nprintf '%s|%s\\n' \"$PWD\" \"$1\" >> \"$WLOG_TEST_SPY\"\necho capture failed >&2\nexit 42\n", 0755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("WLOG_TEST_SPY", spy)
	for _, worktree := range []string{dir, linked} {
		out := gitCommand(t, worktree, "commit", "--allow-empty", "-m", "commit survives capture failure")
		if strings.Contains(out, "capture failed") {
			t.Fatalf("capture leaked: %s", out)
		}
	}
	assertFile(t, spy, dir+"|git\n"+linked+"|git\n", 0644)
}
