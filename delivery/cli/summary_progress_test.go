package cli

import (
	"bytes"
	"strings"
	"testing"

	application "github.com/taupikpirdian/wlog/application/summary"
)

func TestProgressRendererSeparatesSourcesAndMasksSecrets(t *testing.T) {
	t.Setenv("PROVIDER_API_KEY", "private12345")
	var out bytes.Buffer
	renderer := NewProgressRenderer(&out)
	renderer.Render(application.ProgressEvent{Type: application.ProgressStatus, Message: "Loading worklogs"})
	renderer.Render(application.ProgressEvent{Provider: "codex", Type: application.ProgressStatus, Message: "Session started", RepositoryPath: "/recorded/repo"})
	renderer.Render(application.ProgressEvent{Provider: "codex", Type: application.ProgressStatus, Message: "Session started", RepositoryPath: "/recorded/repo"})
	renderer.Render(application.ProgressEvent{Provider: "codex", Type: application.ProgressFile, FilePath: "service.go"})
	renderer.Render(application.ProgressEvent{Provider: "codex", Type: application.ProgressTool, Message: "Running git diff; API_KEY=private12345 Authorization: Bearer abcdef"})
	if renderer.Err() != nil {
		t.Fatal(renderer.Err())
	}
	got := out.String()
	for _, part := range []string{"→ Loading worklogs", "Repository:\n  /recorded/repo", "[Codex] Session started", "[Codex] → Reading service.go", "[REDACTED]"} {
		if !strings.Contains(got, part) {
			t.Fatalf("missing %q: %q", part, got)
		}
	}
	if strings.Count(got, "[Codex] Session started") != 1 || strings.Contains(got, "private12345") || strings.Contains(got, "abcdef") {
		t.Fatalf("unsafe or duplicate output: %q", got)
	}
}
