package gitcontext

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCurrentRepositoryAndFallbacks(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(previous) })
	if got := Current(ctx); got != "" {
		t.Fatalf("outside repository: %q", got)
	}
	git, err := exec.LookPath("git")
	if err == nil {
		if output, err := exec.Command(git, "init", "--quiet", dir).CombinedOutput(); err != nil {
			t.Fatalf("git init: %s %v", output, err)
		}
		nested := filepath.Join(dir, "nested")
		if err := os.Mkdir(nested, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(nested); err != nil {
			t.Fatal(err)
		}
		want, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := Current(ctx); got != want {
			t.Fatalf("repository root: %q want %q", got, want)
		}
	}
	t.Setenv("PATH", t.TempDir())
	if got := Current(ctx); got != "" {
		t.Fatalf("git unavailable: %q", got)
	}
}
