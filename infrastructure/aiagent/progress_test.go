package aiagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
)

func TestCustomStderrStreamsBeforeProviderFinishes(t *testing.T) {
	executable := fakeExecutable(t)
	directory := t.TempDir()
	gate := filepath.Join(directory, "continue")
	t.Setenv("WLOG_AI_HELPER", "1")
	t.Setenv("WLOG_AI_PROVIDER", "custom")
	t.Setenv("WLOG_AI_RESPONSE", fixtureResponse)
	t.Setenv("WLOG_AI_RECORD", filepath.Join(directory, "record"))
	t.Setenv("WLOG_AI_GATE", gate)
	t.Setenv("WLOG_AI_STDERR", "Inspecting repository\nReading service.go\nGenerating summary")
	config := bootstrap.AIConfig{Provider: "custom", Providers: map[string]bootstrap.AIProviderConfig{"custom": {Command: executable, Progress: bootstrap.AIProgressConfig{Mode: "stderr"}}}}
	agent, err := NewFactory().Create(config)
	if err != nil {
		t.Fatal(err)
	}
	if !agent.Capabilities().SupportsStderrProgress {
		t.Fatal("stderr capability missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var events []application.ProgressEvent
	response, err := agent.Generate(ctx, application.AIRequest{WorkingDirectory: directory, WorklogsOnly: true}, func(event application.ProgressEvent) {
		events = append(events, event)
		// The child will not write its result or exit until progress has reached us.
		if len(events) == 3 {
			if err := os.WriteFile(gate, []byte("continue"), 0600); err != nil {
				cancel()
			}
		}
	})
	if err != nil || response == nil || len(events) != 3 {
		t.Fatalf("events=%v response=%+v err=%v", events, response, err)
	}
	for _, event := range events {
		if strings.Contains(event.Message, "worklog") {
			t.Fatal("final JSON leaked to progress")
		}
	}
}

func TestCustomModesSeparateResultFromProgress(t *testing.T) {
	for _, mode := range []string{"none", "stderr", "jsonl"} {
		t.Run(mode, func(t *testing.T) {
			executable := fakeExecutable(t)
			directory := t.TempDir()
			t.Setenv("WLOG_AI_HELPER", "1")
			t.Setenv("WLOG_AI_PROVIDER", "custom")
			t.Setenv("WLOG_AI_RESPONSE", fixtureResponse)
			t.Setenv("WLOG_AI_RECORD", filepath.Join(directory, "record"))
			t.Setenv("WLOG_AI_STDERR", "Loading source\nReading tests")
			if mode == "jsonl" {
				t.Setenv("WLOG_AI_PROGRESS_MODE", "jsonl")
			}
			config := bootstrap.AIConfig{Provider: "custom", Providers: map[string]bootstrap.AIProviderConfig{"custom": {Command: executable, Progress: bootstrap.AIProgressConfig{Mode: mode}}}}
			agent, err := NewFactory().Create(config)
			if err != nil {
				t.Fatal(err)
			}
			var events []application.ProgressEvent
			response, err := agent.Generate(context.Background(), application.AIRequest{WorkingDirectory: directory, WorklogsOnly: true}, func(event application.ProgressEvent) { events = append(events, event) })
			if err != nil || response == nil {
				t.Fatalf("err=%v", err)
			}
			if mode == "none" && len(events) != 0 {
				t.Fatalf("fake progress: %+v", events)
			}
			if mode != "none" && len(events) != 2 {
				t.Fatalf("events=%+v", events)
			}
			for _, event := range events {
				if strings.Contains(event.Message, "PRIVATE") || strings.Contains(event.Message, "worklog") {
					t.Fatalf("unsafe event: %+v", event)
				}
			}
		})
	}
}

func TestProgressSurvivesExitFailureAndDiagnosticsAreRedacted(t *testing.T) {
	executable := fakeExecutable(t)
	directory := t.TempDir()
	t.Setenv("WLOG_AI_HELPER", "1")
	t.Setenv("WLOG_AI_PROVIDER", "custom")
	t.Setenv("WLOG_AI_RECORD", filepath.Join(directory, "record"))
	t.Setenv("WLOG_AI_STDERR", "Reading service.go\nwarning: API_KEY=credential123")
	t.Setenv("WLOG_AI_FAIL", "1")
	config := bootstrap.AIConfig{Provider: "custom", Providers: map[string]bootstrap.AIProviderConfig{"custom": {Command: executable, Progress: bootstrap.AIProgressConfig{Mode: "stderr"}}}}
	agent, err := NewFactory().Create(config)
	if err != nil {
		t.Fatal(err)
	}
	var events []application.ProgressEvent
	_, err = agent.Generate(context.Background(), application.AIRequest{WorkingDirectory: directory, WorklogsOnly: true}, func(event application.ProgressEvent) { events = append(events, event) })
	if err == nil || !strings.Contains(err.Error(), "authentication failed") || len(events) < 2 {
		t.Fatalf("events=%v err=%v", events, err)
	}
	if strings.Contains(err.Error(), "credential123") {
		t.Fatalf("credential leaked in error: %v", err)
	}
	for _, event := range events {
		if strings.Contains(event.Message, "credential123") {
			t.Fatalf("credential leaked: %+v", event)
		}
	}
}

func TestRunnerCancellationClosesStreamingReaders(t *testing.T) {
	executable := fakeExecutable(t)
	directory := t.TempDir()
	t.Setenv("WLOG_AI_HELPER", "1")
	t.Setenv("WLOG_AI_PROVIDER", "custom")
	t.Setenv("WLOG_AI_RECORD", filepath.Join(directory, "record"))
	t.Setenv("WLOG_AI_STDERR", "Reading source")
	t.Setenv("WLOG_AI_GATE", filepath.Join(directory, "never-release"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	progress := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		_, err := (&ProcessRunner{}).Run(ctx, ProcessRequest{Command: executable, Dir: directory, Env: os.Environ(), StderrLine: func([]byte) {
			select {
			case progress <- struct{}{}:
			default:
			}
		}})
		done <- err
	}()
	select {
	case <-progress:
	case <-time.After(3 * time.Second):
		t.Fatal("no live progress received")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("process or streaming goroutines leaked")
	}
}

func TestParsersOnlyEmitObservableEvents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		lines  []string
		create func(eventSink) eventParser
		want   int
	}{
		{"codex", []string{`{"type":"turn.started"}`, `{"type":"item.started","item":{"type":"command_execution","command":"git diff aaa..bbb"}}`, `{"type":"item.completed","item":{"type":"reasoning","text":"PRIVATE"}}`, `{"type":"item.completed","item":{"type":"agent_message","text":"FINAL"}}`, `invalid json`}, func(s eventSink) eventParser { return &codexEventParser{sink: s} }, 2},
		{"claude", []string{`{"type":"system","subtype":"init"}`, `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"service.go"}},{"type":"thinking","thinking":"PRIVATE"},{"type":"text","text":"FINAL"}]}}`, `bad JSON`}, func(s eventSink) eventParser { return &claudeEventParser{sink: s} }, 2},
		{"opencode", []string{`{"type":"step_start"}`, `{"type":"tool_use","part":{"tool":"read","state":{"status":"completed","input":{"filePath":"service.go"}}}}`, `{"type":"reasoning","part":{"text":"PRIVATE"}}`, `{"type":"unknown","part":{"text":"PRIVATE"}}`}, func(s eventSink) eventParser { return &openCodeEventParser{sink: s} }, 2},
		{"custom", []string{`{"type":"tool","name":"git","command":"git diff aaa..bbb"}`, `{"type":"file","path":"service.go"}`, `{"type":"reasoning","message":"PRIVATE"}`, `invalid`}, func(s eventSink) eventParser { return &customEventParser{sink: s} }, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []application.ProgressEvent
			parser := tc.create(eventSink{provider: tc.name, repository: "/repo", handler: func(event application.ProgressEvent) { events = append(events, event) }})
			for _, line := range tc.lines {
				if err := parser.Consume([]byte(line)); err != nil {
					t.Fatal(err)
				}
			}
			if len(events) != tc.want {
				t.Fatalf("events=%+v", events)
			}
			for _, event := range events {
				if strings.Contains(event.Message, "PRIVATE") || strings.Contains(event.Message, "FINAL") {
					t.Fatalf("unsafe event=%+v", event)
				}
			}
		})
	}
}

func TestLongEventAndBoundedStderrTail(t *testing.T) {
	var tail tailBuffer
	for i := 0; i < 100; i++ {
		tail.Write([]byte(strings.Repeat("x", 1024)))
	}
	tail.Write([]byte("error: latest"))
	if len(tail.bytes) > maxStderrBytes || !strings.HasSuffix(string(tail.bytes), "error: latest") {
		t.Fatal("stderr tail is not bounded or lost recent diagnostics")
	}
	line := strings.Repeat("x", 128*1024)
	var mu sync.Mutex
	count := 0
	if err := scanLines(strings.NewReader(line+"\n"), func(body []byte) error {
		mu.Lock()
		defer mu.Unlock()
		count++
		if len(body) != len(line) {
			t.Fatal("line truncated")
		}
		return nil
	}); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}

func TestNativeSkillLoadingEventsAreStreamed(t *testing.T) {
	for _, tc := range []struct {
		name, line string
		create     func(eventSink) eventParser
	}{
		{"claude", `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"ticket-generator"}}]}}`, func(s eventSink) eventParser { return &claudeEventParser{sink: s} }},
		{"opencode", `{"type":"tool_use","part":{"tool":"skill","state":{"status":"running","input":{"name":"ticket-generator"}}}}`, func(s eventSink) eventParser { return &openCodeEventParser{sink: s} }},
		{"codex", `{"type":"item.started","item":{"type":"command_execution","command":"cat /skills/ticket-generator/SKILL.md"}}`, func(s eventSink) eventParser { return &codexEventParser{sink: s} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []application.ProgressEvent
			parser := tc.create(eventSink{provider: tc.name, handler: func(event application.ProgressEvent) { events = append(events, event) }})
			if err := parser.Consume([]byte(tc.line)); err != nil {
				t.Fatal(err)
			}
			if len(events) != 1 || events[0].Provider != tc.name || !strings.Contains(events[0].Message, "ticket-generator") {
				t.Fatalf("events=%+v", events)
			}
		})
	}
}
