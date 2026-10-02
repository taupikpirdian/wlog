package gitcontext

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEmailReader(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is unavailable")
	}
	for _, tc := range []struct{ name, local, global, want string }{
		{"local priority", "local@example.com", "global@example.com", "local@example.com"},
		{"global fallback", "", "global@example.com", "global@example.com"},
		{"empty local fallback", " ", "global@example.com", "global@example.com"},
		{"no email", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(directory, "global.gitconfig"))
			t.Setenv("GIT_CONFIG_COUNT", "0")
			clearGitPaths(t)
			run := func(args ...string) {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = directory
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %s %v", args, output, err)
				}
			}
			run("init", "--quiet")
			if tc.global != "" {
				run("config", "--global", "user.email", tc.global)
			}
			if tc.local != "" {
				run("config", "--local", "user.email", tc.local)
			}
			got, err := NewEmailReader(directory).Email(context.Background())
			if err != nil || got != tc.want {
				t.Fatalf("email=%q err=%v", got, err)
			}
		})
	}
}

func TestEmailReaderOutsideRepositoryAndCancellation(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is unavailable")
	}
	directory := t.TempDir()
	global := filepath.Join(directory, "global.gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n email = outside@example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "0")
	clearGitPaths(t)
	got, err := NewEmailReader(directory).Email(context.Background())
	if err != nil || got != "outside@example.com" {
		t.Fatalf("email=%q err=%v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewEmailReader(directory).Email(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func clearGitPaths(t *testing.T) {
	t.Helper()
	for _, key := range []string{"GIT_CONFIG", "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR"} {
		t.Setenv(key, "") // Register cleanup before removing a potentially inherited override.
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
}
