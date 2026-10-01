package gitcapture

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	application "github.com/taupikpirdian/wlog/application/activity"
)

func fixtureGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	output, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func repositoryFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git unavailable")
	}
	dir := t.TempDir()
	fixtureGit(t, dir, "init", "--quiet")
	fixtureGit(t, dir, "config", "user.name", "Fixture")
	fixtureGit(t, dir, "config", "user.email", "fixture@example.invalid")
	fixtureGit(t, dir, "config", "core.hooksPath", "/dev/null")
	fixtureGit(t, dir, "switch", "-c", "feature/OOT-2-test")
	return dir
}

func TestReadRootCommitPrivacyAndSpecialPaths(t *testing.T) {
	dir := repositoryFixture(t)
	files := map[string][]byte{"file\twith\nspace.txt": []byte("one\ntwo\n"), "binary.dat": {0, 1, 2, 3}, ".env": []byte("DO_NOT_CAPTURE_SECRET\n"), "nested/.env.local": []byte("ANOTHER_SECRET\n")}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	for path, body := range files {
		if err := os.WriteFile(filepath.Join(dir, path), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	fixtureGit(t, dir, "add", ".")
	fixtureGit(t, dir, "commit", "--quiet", "-m", "Subject", "-m", "OOT-1 body key")
	// Uncommitted text must not affect evidence/statistics.
	if err := os.WriteFile(filepath.Join(dir, "file\twith\nspace.txt"), []byte("uncommitted\n"), 0600); err != nil {
		t.Fatal(err)
	}
	value, err := NewReader(dir).Read(context.Background(), application.CaptureOptions{ChangedFiles: true, DiffStat: true})
	if err != nil {
		t.Fatal(err)
	}
	if value.Message == nil || !strings.Contains(*value.Message, "OOT-1 body key") || value.Branch == nil || *value.Branch != "feature/OOT-2-test" {
		t.Fatalf("metadata=%+v", value)
	}
	if len(value.Files) != 4 || value.Insertions == nil || *value.Insertions != 2 || *value.Deletions != 0 || len(value.Metadata.BinaryFiles) != 1 || value.Metadata.DiffBasis != "empty_tree" {
		t.Fatalf("stats=%+v", value)
	}
	if len(value.Metadata.Warnings) != 0 {
		t.Fatalf("warnings=%v", value.Metadata.Warnings)
	}
	fixtureGit(t, dir, "checkout", "--detach", "--quiet")
	value, err = NewReader(dir).Read(context.Background(), application.CaptureOptions{})
	if err != nil || value.Branch != nil || value.Metadata.Status["branch"] != "detached" || value.Files != nil || value.Insertions != nil {
		t.Fatalf("detached/disabled=%+v error=%v", value, err)
	}
}

func TestReadRenameEmptyAndMerge(t *testing.T) {
	dir := repositoryFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "old.txt"), []byte("line\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, dir, "add", ".")
	fixtureGit(t, dir, "commit", "--quiet", "-m", "OOT-1 root")
	fixtureGit(t, dir, "mv", "old.txt", "new.txt")
	fixtureGit(t, dir, "commit", "--quiet", "-m", "OOT-1 rename")
	options := application.CaptureOptions{ChangedFiles: true, DiffStat: true}
	value, err := NewReader(dir).Read(context.Background(), options)
	if err != nil || len(value.Files) != 2 || *value.Insertions != 1 || *value.Deletions != 1 {
		t.Fatalf("rename=%+v error=%v", value, err)
	}
	fixtureGit(t, dir, "commit", "--quiet", "--allow-empty", "-m", "empty")
	value, err = NewReader(dir).Read(context.Background(), options)
	if err != nil || value.Files == nil || len(value.Files) != 0 || *value.Insertions != 0 || *value.Deletions != 0 {
		t.Fatalf("empty=%+v error=%v", value, err)
	}
	fixtureGit(t, dir, "switch", "-c", "side")
	if err := os.WriteFile(filepath.Join(dir, "side.txt"), []byte("side\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, dir, "add", ".")
	fixtureGit(t, dir, "commit", "--quiet", "-m", "side")
	fixtureGit(t, dir, "switch", "feature/OOT-2-test")
	fixtureGit(t, dir, "merge", "--no-ff", "--no-edit", "side")
	value, err = NewReader(dir).Read(context.Background(), options)
	if err != nil || len(value.Files) != 1 || value.Files[0] != "side.txt" || *value.Insertions != 1 || *value.Deletions != 0 || value.Metadata.DiffBasis != "first_parent" {
		t.Fatalf("merge=%+v error=%v", value, err)
	}
}

func TestReaderPartialFailureAndPinnedHash(t *testing.T) {
	dir := repositoryFixture(t)
	fixtureGit(t, dir, "commit", "--quiet", "--allow-empty", "-m", "OOT-1 original")
	pinned := fixtureGit(t, dir, "rev-parse", "HEAD")
	headReads := 0
	reader := NewReaderWithRunner(dir, func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--format=format:%cI") || strings.Contains(joined, "--numstat") {
			return nil, errors.New("optional failure")
		}
		if joined == "rev-parse --verify HEAD^{commit}" {
			headReads++
			if headReads == 2 {
				fixtureGit(t, dir, "commit", "--quiet", "--allow-empty", "-m", "OOT-2 new HEAD")
			}
		}
		return runGit(ctx, dir, args...)
	})
	value, err := reader.Read(context.Background(), application.CaptureOptions{ChangedFiles: true, DiffStat: true})
	if err != nil || value.Hash != pinned || value.Message == nil || !strings.Contains(*value.Message, "original") || value.Branch != nil || !value.CreatedAt.IsZero() || value.Insertions != nil || value.Files == nil {
		t.Fatalf("snapshot=%+v error=%v", value, err)
	}
	if value.Metadata.Status["branch"] != "unstable" || value.Metadata.Status["timestamp"] != "unavailable" || value.Metadata.Status["diff_stat"] != "unavailable" {
		t.Fatalf("status=%v", value.Metadata.Status)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewReader(dir).Read(cancelled, application.CaptureOptions{}); err == nil {
		t.Fatal("accepted cancelled read")
	}
}

func TestReaderEssentialFailures(t *testing.T) {
	dir := repositoryFixture(t)
	if _, err := NewReader(dir).Read(context.Background(), application.CaptureOptions{}); err == nil {
		t.Fatal("accepted unborn HEAD")
	}
	if _, err := NewReader(t.TempDir()).Read(context.Background(), application.CaptureOptions{}); err == nil {
		t.Fatal("accepted outside repository")
	}
	bare := t.TempDir()
	fixtureGit(t, bare, "init", "--bare", "--quiet")
	if _, err := NewReader(bare).Read(context.Background(), application.CaptureOptions{}); err == nil {
		t.Fatal("accepted bare repository")
	}
	reader := NewReaderWithRunner(dir, func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("unsupported Git offline capability")
	})
	if _, err := reader.Read(context.Background(), application.CaptureOptions{}); err == nil {
		t.Fatal("accepted missing Git capability")
	}
}

func TestParsersRejectMalformedData(t *testing.T) {
	for _, input := range []string{"unterminated", "1\t-\tfile\x00", "-1\t0\tfile\x00", "1\t2\t\x00", "9223372036854775807\t0\ta\x001\t0\tb\x00"} {
		if _, _, _, err := parseNumstat([]byte(input)); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
	if _, err := parsePaths([]byte("unterminated")); err == nil {
		t.Fatal("accepted incomplete paths")
	}
}
