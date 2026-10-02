package aiagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
)

const fixtureResponse = `{"worklog":{"details":["Changed code"],"results":[]},"ticket_description":{"background":"Background","problem_requirement":"Requirement","scope":["Change"],"expected_result":"Expected","technical_notes":""}}`

type processRecord struct {
	Directory, Input, GitDirectory string
	Args                           []string
}

// Fake local CLI validates all provider transport formats without paid AI calls.
func TestAgentHelper(t *testing.T) {
	if os.Getenv("WLOG_AI_HELPER") != "1" {
		return
	}
	input, _ := io.ReadAll(os.Stdin)
	directory, _ := os.Getwd()
	record, _ := json.Marshal(processRecord{Directory: directory, Input: string(input), GitDirectory: os.Getenv("GIT_DIR"), Args: os.Args})
	_ = os.WriteFile(os.Getenv("WLOG_AI_RECORD"), record, 0600)
	for _, line := range strings.Split(os.Getenv("WLOG_AI_STDERR"), "\n") {
		if line != "" {
			fmt.Fprintln(os.Stderr, line)
		}
	}
	if gate := os.Getenv("WLOG_AI_GATE"); gate != "" {
		for {
			if _, err := os.Stat(gate); err == nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if os.Getenv("WLOG_AI_FAIL") == "1" {
		fmt.Fprint(os.Stderr, "authentication failed")
		os.Exit(2)
	}
	body := os.Getenv("WLOG_AI_RESPONSE")
	switch os.Getenv("WLOG_AI_PROVIDER") {
	case "codex":
		fmt.Println(`{"type":"turn.started"}`)
		fmt.Println(`{"type":"item.started","item":{"type":"command_execution","command":"git diff","status":"in_progress"}}`)
		for i, arg := range os.Args {
			if arg == "--output-last-message" && i+1 < len(os.Args) {
				_ = os.WriteFile(os.Args[i+1], []byte(body), 0600)
			}
		}
		fmt.Println("diagnostic output is not the final response")
	case "claude":
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "result", "is_error": false, "structured_output": json.RawMessage(body)})
	case "opencode":
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "step_start"})
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "text", "part": map[string]string{"text": body[:len(body)/2]}})
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "text", "part": map[string]string{"text": body[len(body)/2:]}})
	default:
		if os.Getenv("WLOG_AI_PROGRESS_MODE") == "jsonl" {
			fmt.Println(`{"type":"status","message":"Repository inspected"}`)
			fmt.Println(`{"type":"tool","name":"git","command":"git diff aaa..bbb"}`)
			fmt.Println(`{"type":"reasoning","message":"PRIVATE THINKING"}`)
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "result", "data": json.RawMessage(body)})
		} else {
			fmt.Print(body)
		}
	}
	os.Exit(0)
}

func fakeExecutable(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture requires POSIX; application tests cover Windows-independent behavior")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "fake-agent")
	quoted := "'" + strings.ReplaceAll(executable, "'", "'\\''") + "'"
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexec "+quoted+" -test.run=TestAgentHelper -- \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestProviderProcessFormatsAndExplicitRepositoryDirectory(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "opencode", "custom"} {
		t.Run(provider, func(t *testing.T) {
			executable := fakeExecutable(t)
			directory := t.TempDir()
			recordPath := filepath.Join(t.TempDir(), "record.json")
			t.Setenv("WLOG_AI_HELPER", "1")
			t.Setenv("WLOG_AI_PROVIDER", provider)
			t.Setenv("WLOG_AI_RESPONSE", fixtureResponse)
			t.Setenv("WLOG_AI_RECORD", recordPath)
			t.Setenv("GIT_DIR", "/incorrect-repository")
			config := bootstrap.AIConfig{Provider: provider, Providers: map[string]bootstrap.AIProviderConfig{provider: {Command: executable}}}
			agent, err := NewFactory().Create(config)
			if err != nil {
				t.Fatal(err)
			}
			response, err := agent.Generate(context.Background(), application.AIRequest{WorkingDirectory: directory, WorklogsOnly: true, Context: application.TicketAIContext{OutputLanguage: application.LanguageEnglish}}, nil)
			if err != nil || response.Worklog.Details[0] != "Changed code" {
				t.Fatalf("response=%+v err=%v", response, err)
			}
			body, err := os.ReadFile(recordPath)
			if err != nil {
				t.Fatal(err)
			}
			var record processRecord
			if err := json.Unmarshal(body, &record); err != nil {
				t.Fatal(err)
			}
			canonical, _ := filepath.EvalSymlinks(directory)
			if record.Directory != canonical || record.GitDirectory != "" || !strings.Contains(record.Input, "ACTUAL SOURCE CODE CHANGE") {
				t.Fatalf("record=%+v", record)
			}
			if !strings.Contains(record.Input, "output_language: en") || !strings.Contains(record.Input, "prose in English") || strings.Contains(record.Input, "output_language: id") {
				t.Fatal("selected language was not sent to the provider")
			}
		})
	}
}

func TestCustomPromptArgumentsAndProcessFailures(t *testing.T) {
	executable := fakeExecutable(t)
	directory := t.TempDir()
	recordPath := filepath.Join(t.TempDir(), "record.json")
	t.Setenv("WLOG_AI_HELPER", "1")
	t.Setenv("WLOG_AI_PROVIDER", "custom")
	t.Setenv("WLOG_AI_RESPONSE", fixtureResponse)
	t.Setenv("WLOG_AI_RECORD", recordPath)
	config := bootstrap.AIConfig{Provider: "custom", Providers: map[string]bootstrap.AIProviderConfig{"custom": {Command: executable, Args: []string{"--prompt", "{{prompt}}"}}}}
	agent, err := NewFactory().Create(config)
	if err != nil {
		t.Fatal(err)
	}
	request := application.AIRequest{WorkingDirectory: directory, WorklogsOnly: true}
	if _, err := agent.Generate(context.Background(), request, nil); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(recordPath)
	var record processRecord
	_ = json.Unmarshal(body, &record)
	if record.Input != "" || !strings.Contains(strings.Join(record.Args, " "), "ACTUAL SOURCE CODE CHANGE") {
		t.Fatalf("template failed: %+v", record)
	}
	t.Setenv("WLOG_AI_RESPONSE", "{broken")
	if _, err := agent.Generate(context.Background(), request, nil); err == nil || !strings.Contains(err.Error(), "AI response invalid") {
		t.Fatalf("err=%v", err)
	}
	t.Setenv("WLOG_AI_FAIL", "1")
	if _, err := agent.Generate(context.Background(), request, nil); err == nil || !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("err=%v", err)
	}
	config.Providers["custom"] = bootstrap.AIProviderConfig{Command: filepath.Join(directory, "missing-agent")}
	if _, err := NewFactory().Create(config); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("err=%v", err)
	}
}
