package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
)

type diagnosticSkillResolver struct {
	skill               application.TicketSkill
	provider, directory string
}

func (r *diagnosticSkillResolver) Resolve(_ context.Context, provider, directory string, progress application.ProgressHandler) (application.TicketSkill, error) {
	r.provider, r.directory = provider, directory
	progress(application.ProgressEvent{Type: application.ProgressStatus, Message: "Checking installed AI skills"})
	return r.skill, nil
}

func TestSkillsCheckRegisteredAndSharesResolver(t *testing.T) {
	for _, loaded := range []bool{true, false} {
		path := filepath.Join(t.TempDir(), "ticket-generator", "SKILL.md")
		resolver := &diagnosticSkillResolver{skill: application.TicketSkill{Loaded: loaded, Path: path, CheckedPaths: []string{path}, FailureReason: "permission denied", Instructions: "PRIVATE SKILL INSTRUCTIONS"}}
		config := &configMemory{config: bootstrap.Config{AI: bootstrap.AIConfig{Provider: "claude"}}}
		root := NewRootCommand(nil, "test", nil, nil)
		root.AddCommand(NewSkillsCommand(config, resolver))
		var out, progress bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&progress)
		root.SetArgs([]string{"skills", "check", "ticket-generator", "--repository", filepath.Dir(path)})
		err := root.Execute()
		if (err == nil) != loaded || resolver.provider != "claude" || resolver.directory != filepath.Dir(path) || !strings.Contains(out.String(), path) || strings.Contains(out.String(), "PRIVATE SKILL") {
			t.Fatalf("error=%v output=%q resolver=%+v", err, out.String(), resolver)
		}
		if loaded && !strings.Contains(out.String(), "Status: Loaded") {
			t.Fatal("successful reading not reported")
		}
		if !loaded && !strings.Contains(err.Error(), "permission denied") {
			t.Fatal("load failure reason lost")
		}
	}
}
