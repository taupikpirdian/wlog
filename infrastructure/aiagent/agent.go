// Package aiagent adapts noninteractive local AI CLIs to the summary contract.
package aiagent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
)

type Factory struct{ runner AIProcessRunner }

func NewFactory() *Factory { return &Factory{runner: &ProcessRunner{}} }

type commandAgent struct {
	provider string
	config   bootstrap.AIProviderConfig
	runner   AIProcessRunner
}
type CodexAgent struct{ commandAgent }
type ClaudeAgent struct{ commandAgent }
type OpenCodeAgent struct{ commandAgent }
type CustomAgent struct{ commandAgent }

func (f *Factory) Create(config bootstrap.AIConfig) (application.AIAgent, error) {
	if err := application.ValidateAIConfig(config); err != nil {
		return nil, err
	}
	provider := config.Providers[config.Provider]
	if _, err := exec.LookPath(provider.Command); err != nil {
		return nil, fmt.Errorf("AI executable %q is not installed or not on PATH; install/authenticate it or run wl config ai: %w", provider.Command, err)
	}
	runner := f.runner
	if runner == nil {
		runner = &ProcessRunner{}
	}
	agent := commandAgent{provider: config.Provider, config: provider, runner: runner}
	switch config.Provider {
	case "codex":
		return &CodexAgent{agent}, nil
	case "claude":
		return &ClaudeAgent{agent}, nil
	case "opencode":
		return &OpenCodeAgent{agent}, nil
	default:
		return &CustomAgent{agent}, nil
	}
}

const maxResponseBytes = 4 * 1024 * 1024

type limitedBuffer struct {
	bytes.Buffer
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := maxResponseBytes - b.Len()
	if n > remaining {
		b.exceeded = true
		p = p[:remaining]
	}
	_, err := b.Buffer.Write(p)
	return n, err
}

func (a *commandAgent) Capabilities() application.AICapabilities {
	if a.provider != "custom" || a.config.Progress.Mode == "jsonl" {
		return application.AICapabilities{SupportsEventStream: true}
	}
	return application.AICapabilities{SupportsStderrProgress: a.config.Progress.Mode == "stderr"}
}

func (a *commandAgent) Generate(ctx context.Context, request application.AIRequest, progress application.ProgressHandler) (*application.AIResponse, error) {
	prompt, err := application.BuildAIPrompt(request)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(request.WorkingDirectory) {
		return nil, fmt.Errorf("AI working directory must be an explicit absolute path")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	args := append([]string{}, a.config.Args...)
	input := prompt
	outputFile := ""
	var parser eventParser
	var progressMu sync.Mutex
	secrets := application.SecretValues(os.Environ())
	sink := eventSink{provider: a.provider, repository: request.WorkingDirectory, handler: func(event application.ProgressEvent) {
		if progress == nil {
			return
		}
		event.Message = application.RedactOutput(event.Message, secrets)
		event.FilePath = application.RedactOutput(event.FilePath, secrets)
		progressMu.Lock()
		defer progressMu.Unlock()
		progress(event)
	}}
	var env []string
	for _, item := range os.Environ() {
		// Git operations performed by the agent must also respect cmd.Dir.
		if !strings.HasPrefix(item, "GIT_") {
			env = append(env, item)
		}
	}
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	switch a.provider {
	case "codex":
		directory, err := os.MkdirTemp("", "wlog-ai-*")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(directory)
		schema := filepath.Join(directory, "schema.json")
		if err := os.WriteFile(schema, []byte(application.ResponseSchema), 0600); err != nil {
			return nil, err
		}
		outputFile = filepath.Join(directory, "response.json")
		args = append([]string{"exec"}, args...)
		args = append(args, "--sandbox", "read-only", "--ephemeral", "--skip-git-repo-check", "--color", "never", "--json", "--output-schema", schema, "--output-last-message", outputFile, "-")
		parser = &codexEventParser{sink: sink}
	case "claude":
		tools := "Read,Grep,Glob,Bash"
		allowed := "Read,Grep,Glob,Bash(git diff *),Bash(git show *),Bash(git log *),Bash(git cat-file *)"
		if request.Skill.Loaded && request.Skill.Native {
			tools += ",Skill"
			allowed += ",Skill(ticket-generator)"
		}
		args = append(args, "--print", "--output-format", "stream-json", "--verbose", "--json-schema", application.ResponseSchema, "--permission-mode", "plan", "--tools", tools, "--allowedTools", allowed, "--no-session-persistence")
		parser = &claudeEventParser{sink: sink}
	case "opencode":
		args = append([]string{"run"}, args...)
		args = append(args, "--format", "json", "--agent", "plan")
		// v1 plan-agent permissions: source inspection only. No auto-approval.
		permission := `{"agent":{"plan":{"permission":{"*":"deny","read":"allow","glob":"allow","grep":"allow","bash":{"*":"deny","git diff *":"allow","git show *":"allow","git log *":"allow","git cat-file *":"allow"}}}}}`
		if request.Skill.Loaded && request.Skill.Native {
			permission = strings.Replace(permission, `"read":"allow"`, `"read":"allow","skill":{"*":"deny","ticket-generator":"allow"}`, 1)
		}
		filtered := env[:0]
		for _, item := range env {
			if !strings.HasPrefix(item, "OPENCODE_CONFIG_CONTENT=") {
				filtered = append(filtered, item)
			}
		}
		env = append(filtered, "OPENCODE_CONFIG_CONTENT="+permission)
		parser = &openCodeEventParser{sink: sink}
	case "custom":
		for i, arg := range args {
			if strings.Contains(arg, "{{prompt}}") {
				args[i] = strings.ReplaceAll(arg, "{{prompt}}", prompt)
				input = ""
			}
		}
		if a.config.Progress.Mode == "jsonl" {
			parser = &customEventParser{sink: sink}
		}
	}
	process := ProcessRequest{Command: a.config.Command, Args: args, Dir: request.WorkingDirectory, Env: env, Input: input}
	if parser != nil {
		process.StdoutLine = parser.Consume
	}
	if a.provider == "custom" && a.config.Progress.Mode == "stderr" {
		process.StderrLine = func(line []byte) {
			text := strings.TrimSpace(string(line))
			lower := strings.ToLower(text)
			if text == "" || strings.Contains(lower, "reasoning") || strings.Contains(lower, "thinking") || strings.Contains(lower, "chain_of_thought") {
				return
			}
			sink.emit(application.ProgressInfo, text, "", "")
		}
	}
	result, err := a.runner.Run(ctx, process)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("AI process canceled or timed out: %w", ctx.Err())
		}
		return nil, fmt.Errorf("AI process %s failed; check provider authentication and configured arguments: %w (%s)", a.provider, err, application.SafeDiagnostics(result.StderrTail, secrets))
	}
	body := result.Stdout
	if outputFile != "" {
		info, err := os.Stat(outputFile)
		if err != nil {
			return nil, fmt.Errorf("Codex produced no final response: %w", err)
		}
		if info.Size() > maxResponseBytes {
			return nil, fmt.Errorf("AI final response is too large")
		}
		body, err = os.ReadFile(outputFile)
		if err != nil {
			return nil, err
		}
	} else if parser != nil {
		body, err = parser.Result()
		if err != nil {
			return nil, fmt.Errorf("AI provider response failed: %s", application.SafeDiagnostics(err.Error(), secrets))
		}
	}
	return application.ParseAIResponse(body)
}
