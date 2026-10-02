//go:build unix

package githook

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func canonicalTemp(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func gitCommand(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func newRepository(t *testing.T) string {
	t.Helper()
	dir := canonicalTemp(t)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "no-global-config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	gitCommand(t, dir, "init", "--quiet")
	gitCommand(t, dir, "config", "user.name", "Hook Fixture")
	gitCommand(t, dir, "config", "user.email", "fixture@example.invalid")
	gitCommand(t, dir, "config", "commit.gpgsign", "false")
	return dir
}

func TestInspectorUnbornAndLinkedWorktrees(t *testing.T) {
	dir := newRepository(t)
	sub := filepath.Join(dir, "subdirectory")
	if err := os.Mkdir(sub, 0700); err != nil {
		t.Fatal(err)
	}
	location, err := NewInspector(sub).Inspect(context.Background())
	if err != nil || location.Root != dir || location.HookPath != filepath.Join(dir, ".git", "hooks", "post-commit") || location.Shared {
		t.Fatalf("location=%+v err=%v", location, err)
	}
	gitCommand(t, dir, "commit", "--quiet", "--allow-empty", "-m", "initial")
	linked := filepath.Join(canonicalTemp(t), "linked worktree's path")
	gitCommand(t, dir, "worktree", "add", "--quiet", "-b", "linked", linked)
	other, err := NewInspector(linked).Inspect(context.Background())
	if err != nil || other.Root != linked || other.HookPath != location.HookPath || !other.Shared {
		t.Fatalf("location=%+v err=%v", other, err)
	}
}

func TestInspectorUnsupportedAndUnavailable(t *testing.T) {
	for _, custom := range []string{"", "relative-hooks", "/dev/null", "/absolute/hooks"} {
		t.Run("custom="+custom, func(t *testing.T) {
			dir := newRepository(t)
			gitCommand(t, dir, "config", "core.hooksPath", custom)
			location, err := NewInspector(dir).Inspect(context.Background())
			if err != nil || !location.Custom {
				t.Fatalf("location=%+v err=%v", location, err)
			}
		})
	}
	t.Run("nonrepository", func(t *testing.T) {
		if _, err := NewInspector(canonicalTemp(t)).Inspect(context.Background()); err == nil {
			t.Fatal("accepted nonrepository")
		}
	})
	t.Run("bare", func(t *testing.T) {
		dir := newRepository(t)
		bare := canonicalTemp(t)
		gitCommand(t, dir, "init", "--bare", "--quiet", bare)
		if _, err := NewInspector(bare).Inspect(context.Background()); err == nil {
			t.Fatal("accepted bare repository")
		}
	})
	t.Run("missing git", func(t *testing.T) {
		dir := newRepository(t)
		t.Setenv("PATH", canonicalTemp(t))
		if _, err := NewInspector(dir).Inspect(context.Background()); err == nil {
			t.Fatal("accepted missing git")
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := NewInspector(canonicalTemp(t)).Inspect(ctx); err == nil {
			t.Fatal("accepted cancelled context")
		}
	})
}
