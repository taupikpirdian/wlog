package gitevidence

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureRepo(t *testing.T) (string, string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git unavailable")
	}
	directory := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.com", "-c", "core.hooksPath=/dev/null", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = directory
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, output, err)
		}
		return strings.TrimSpace(string(output))
	}
	run("init", "--quiet")
	file := filepath.Join(directory, "service.go")
	if err := os.WriteFile(file, []byte("package example\nconst Value = 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "service.go")
	run("commit", "--quiet", "-m", "initial")
	start := run("rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("package example\nconst Value = 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "service.go")
	run("commit", "--quiet", "-m", "ticket change")
	end := run("rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("package example\nconst Value = 999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "service.go")
	run("commit", "--quiet", "-m", "unrelated HEAD")
	return directory, start, end
}

func TestDiffUsesRecordedRepositoryAndCapturedHash(t *testing.T) {
	directory, start, end := fixtureRepo(t)
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if current == directory {
		t.Fatal("fixture must differ from cwd")
	}
	// Prove an inherited environment cannot redirect this service to another repo.
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "missing.git"))
	service := NewService()
	canonical, err := service.ValidateRepository(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	change, err := service.Change(context.Background(), canonical, end)
	if err != nil || change.Start != start || change.End != end {
		t.Fatalf("change=%+v err=%v", change, err)
	}
	if !strings.Contains(change.Diff, "-const Value = 1") || !strings.Contains(change.Diff, "+const Value = 2") || strings.Contains(change.Diff, "999") {
		t.Fatalf("wrong range diff: %s", change.Diff)
	}
	root, err := service.Change(context.Background(), canonical, start)
	if err != nil || root.Start != "" || !strings.Contains(root.Diff, "+const Value = 1") {
		t.Fatalf("root=%+v err=%v", root, err)
	}
}

func TestMissingRepositoryAndCommits(t *testing.T) {
	service := NewService()
	for _, path := range []string{filepath.Join(t.TempDir(), "missing"), t.TempDir(), "relative/path"} {
		if _, err := service.ValidateRepository(context.Background(), path); err == nil {
			t.Fatalf("invalid path accepted: %s", path)
		}
	}
	directory, _, _ := fixtureRepo(t)
	for _, hash := range []string{"deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdead", "", "--help", "HEAD"} {
		if _, err := service.Change(context.Background(), directory, hash); err == nil {
			t.Fatalf("invalid commit accepted: %s", hash)
		}
	}
}
