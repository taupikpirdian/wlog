package aiskills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	application "github.com/taupikpirdian/wlog/application/summary"
)

func TestSkillAvailabilityReadingAndFallback(t *testing.T) {
	for _, tc := range []struct {
		name, provider    string
		exists, readFails bool
	}{
		{"native available", "codex", true, false},
		{"custom available", "custom", true, false},
		{"not installed", "codex", false, false},
		{"unreadable", "codex", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			repo := t.TempDir()
			path := filepath.Join(repo, ".agents", "skills", "ticket-generator", "SKILL.md")
			body := "---\nname: ticket-generator\n---\nFULL INSTRUCTIONS\nUse observed implementation evidence.\n"
			if tc.exists {
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			reads := 0
			resolver := &Resolver{home: func() (string, error) { return home, nil }, readFile: func(path string) ([]byte, error) {
				reads++
				if tc.readFails {
					return nil, errors.New("permission denied")
				}
				return readSkill(path)
			}}
			var events []application.ProgressEvent
			value, err := resolver.Resolve(context.Background(), tc.provider, repo, func(event application.ProgressEvent) { events = append(events, event) })
			if err != nil {
				t.Fatal(err)
			}
			var messages []string
			for _, event := range events {
				if event.Provider != "" {
					t.Fatal("wlog falsely claimed provider activity")
				}
				messages = append(messages, event.Message)
			}
			output := strings.Join(messages, "\n")
			if !strings.HasPrefix(output, "Checking installed AI skills") {
				t.Fatalf("progress=%s", output)
			}
			if !tc.exists {
				if reads != 0 || value.Loaded || strings.Contains(output, "Reading ticket-generator") || !strings.Contains(output, "skill not found") {
					t.Fatalf("reads=%d skill=%+v progress=%s", reads, value, output)
				}
				return
			}
			if reads != 1 || !strings.Contains(output, "Found skill: ticket-generator") || !strings.Contains(output, "Reading ticket-generator skill instructions") {
				t.Fatalf("reads=%d progress=%s", reads, output)
			}
			if tc.readFails {
				if value.Loaded || !strings.Contains(output, "Failed to load") || !strings.Contains(output, "Using built-in wlog ticket generator") {
					t.Fatalf("skill=%+v progress=%s", value, output)
				}
				return
			}
			if !value.Loaded || value.Path != path {
				t.Fatalf("skill=%+v", value)
			}
			if tc.provider == "custom" && value.Instructions != body {
				t.Fatal("full custom instructions were not loaded")
			}
			if tc.provider == "codex" && (value.Instructions != "" || !value.Native || value.Invocation != "$ticket-generator" || !strings.Contains(output, "will be requested") || !strings.Contains(output, "skill loaded by wlog")) {
				t.Fatalf("native skill falsely marked as agent-loaded: %+v %s", value, output)
			}
		})
	}
}

func TestSkillPathsAreProviderSpecific(t *testing.T) {
	for _, provider := range []string{"claude", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			home := t.TempDir()
			repo := t.TempDir()
			paths, _ := skillPaths(provider, repo, home)
			path := paths[0]
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("Complete methodology"), 0600); err != nil {
				t.Fatal(err)
			}
			resolver := &Resolver{home: func() (string, error) { return home, nil }, readFile: readSkill}
			skill, err := resolver.Resolve(context.Background(), provider, repo, nil)
			if err != nil || !skill.Native || !skill.Loaded {
				t.Fatalf("skill=%+v err=%v", skill, err)
			}
		})
	}
}

func TestReadSkillRejectsEmptyLargeAndNonregularFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
	}{{"empty", nil}, {"large", []byte(strings.Repeat("x", maxSkillBytes+1))}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "SKILL.md")
			if err := os.WriteFile(path, tc.body, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readSkill(path); err == nil {
				t.Fatal("invalid instructions accepted")
			}
		})
	}
	if _, err := readSkill(t.TempDir()); err == nil {
		t.Fatal("directory accepted as instructions")
	}
}

func TestSkillInInvocationDirectoryIsReadOutsideTicketRepository(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "opencode", "custom"} {
		t.Run(provider, func(t *testing.T) {
			cwd, repo, home := t.TempDir(), t.TempDir(), t.TempDir()
			path := filepath.Join(cwd, ".agents", "skills", "ticket-generator", "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("FULL INSTALLED SKILL\nPreserve title and QA Impact."), 0600); err != nil {
				t.Fatal(err)
			}
			reads := 0
			resolver := &Resolver{home: func() (string, error) { return home, nil }, cwd: func() (string, error) { return cwd, nil }, readFile: func(path string) ([]byte, error) { reads++; return readSkill(path) }}
			skill, err := resolver.Resolve(context.Background(), provider, repo, nil)
			if err != nil || !skill.Loaded || skill.Path != path || reads != 1 || len(skill.CheckedPaths) == 0 {
				t.Fatalf("skill=%+v reads=%d error=%v", skill, reads, err)
			}
			if repo == filepath.Dir(path) {
				t.Fatal("test must keep source repository independent from skill location")
			}
		})
	}
}
