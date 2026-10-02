package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	application "github.com/taupikpirdian/wlog/application/summary"
)

type ProgressRenderer struct {
	out                    io.Writer
	mu                     sync.Mutex
	err                    error
	secrets                []string
	lastStatus, repository string
}

func NewProgressRenderer(out io.Writer) *ProgressRenderer {
	return &ProgressRenderer{out: out, secrets: application.SecretValues(os.Environ())}
}
func (r *ProgressRenderer) clean(text string) string {
	return safeText(application.RedactOutput(text, r.secrets))
}
func (r *ProgressRenderer) Render(event application.ProgressEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return
	}
	if event.RepositoryPath != "" && event.RepositoryPath != r.repository {
		r.repository = event.RepositoryPath
		_, r.err = fmt.Fprintf(r.out, "\nRepository:\n  %s\n", r.clean(event.RepositoryPath))
		if r.err != nil {
			return
		}
	}
	message := r.clean(event.Message)
	if event.Type == application.ProgressFile {
		message = "Reading " + r.clean(event.FilePath)
	}
	if len([]rune(message)) > 500 {
		message = string([]rune(message)[:500]) + "…"
	}
	if strings.TrimSpace(message) == "" {
		return
	}
	statusKey := event.Provider + ":" + event.RepositoryPath + ":" + message
	if event.Type == application.ProgressStatus && statusKey == r.lastStatus {
		return
	}
	if event.Type == application.ProgressStatus {
		r.lastStatus = statusKey
	} else {
		r.lastStatus = ""
	}
	prefix := "→ "
	if event.Provider != "" {
		prefix = "[" + providerLabel(event.Provider) + "] "
		if event.Type == application.ProgressTool || event.Type == application.ProgressFile {
			prefix += "→ "
		}
	}
	if event.Type == application.ProgressWarning {
		prefix += "⚠ "
	}
	_, r.err = fmt.Fprintln(r.out, prefix+message)
}
func providerLabel(provider string) string {
	switch provider {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	case "opencode":
		return "OpenCode"
	case "custom":
		return "Custom"
	default:
		return safeText(provider)
	}
}
func (r *ProgressRenderer) Warning(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return
	}
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		lines = append(lines, r.clean(line))
	}
	_, r.err = fmt.Fprintln(r.out, "\n"+strings.Join(lines, "\n"))
}
func (r *ProgressRenderer) Err() error { r.mu.Lock(); defer r.mu.Unlock(); return r.err }
