package aiagent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
)

const markdownTicket = "# [FEATURE] Validate authorization input\n\n## Description\nValidate captured input.\n\n## Goal\nReject invalid input.\n\n## In Scope\n- Validation\n\n## Out of Scope\n- Deployment\n\n## QA Impact\n| No | Area | What to check | Expected | Owner |\n|----|------|---------------|----------|-------|\n| 1 | Auth | Empty input | Rejected | QA Engineer |\n\n## Acceptance Criteria\n- Empty input is rejected\n"

func TestTicketMarkdownTransportPreservesSkillArtifact(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "opencode", "custom"} {
		for _, mode := range []string{"none", "jsonl"} {
			if provider != "custom" && mode == "jsonl" {
				continue
			}
			t.Run(provider+"/"+mode, func(t *testing.T) {
				directory, secondRepo := t.TempDir(), t.TempDir()
				recordPath := filepath.Join(t.TempDir(), "record.json")
				t.Setenv("WLOG_AI_HELPER", "1")
				t.Setenv("WLOG_AI_PROVIDER", provider)
				t.Setenv("WLOG_AI_ARTIFACT", "ticket")
				t.Setenv("WLOG_AI_PROGRESS_MODE", mode)
				t.Setenv("WLOG_AI_RESPONSE", markdownTicket)
				t.Setenv("WLOG_AI_RECORD", recordPath)
				t.Setenv("GIT_DIR", "/incorrect/repository")
				config := bootstrap.AIConfig{Provider: provider, Providers: map[string]bootstrap.AIProviderConfig{provider: {Command: fakeExecutable(t), Progress: bootstrap.AIProgressConfig{Mode: mode}}}}
				agent, err := NewFactory().Create(config)
				if err != nil {
					t.Fatal(err)
				}
				request := application.AIRequest{WorkingDirectory: directory, Context: application.TicketAIContext{TicketKey: "OOT-1", OutputLanguage: application.LanguageEnglish, Repositories: []application.RepositoryAIContext{
					{Path: directory, Changes: []application.CodeChange{{CommitRange: application.CommitRange{Start: "aaa", End: "bbb"}, Diff: "+first captured change"}}},
					{Path: secondRepo, Changes: []application.CodeChange{{CommitRange: application.CommitRange{Start: "ccc", End: "ddd"}, Diff: "+second captured change"}}},
				}}, Skill: application.TicketSkill{Loaded: true, Native: provider != "custom", Invocation: "ticket-generator", Path: filepath.Join(directory, "SKILL.md"), Instructions: "SKILL CONTENT\nFollow title and conditional QA Impact format."}}
				var events []application.ProgressEvent
				result, err := agent.(application.TicketAIAgent).GenerateTicket(context.Background(), request, func(e application.ProgressEvent) { events = append(events, e) })
				if err != nil || result.Content != markdownTicket {
					t.Fatalf("result=%+v error=%v", result, err)
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
				if record.Directory != canonical || record.GitDirectory != "" || !strings.Contains(record.Input, secondRepo) || !strings.Contains(record.Input, "second captured change") || !strings.Contains(record.Input, "output_language: en") {
					t.Fatalf("wrong authoritative process context: %+v", record)
				}
				for _, excluded := range []string{"### Background", "### Problem / Requirement", application.ResponseSchema} {
					if strings.Contains(record.Input, excluded) {
						t.Fatalf("skill prompt contains custom format: %s", excluded)
					}
				}
				for _, arg := range record.Args {
					if arg == "--output-schema" || arg == "--json-schema" {
						t.Fatal("Markdown generation was forced into structured summary output")
					}
				}
				if provider == "claude" && !strings.Contains(strings.Join(record.Args, " "), "--add-dir "+secondRepo) {
					t.Fatal("Claude lost access to the second recorded repository")
				}
				if provider == "opencode" {
					var config struct {
						Agent map[string]struct {
							Permission struct {
								External map[string]string `json:"external_directory"`
								Bash     map[string]string `json:"bash"`
								Default  string            `json:"*"`
							} `json:"permission"`
						} `json:"agent"`
					}
					if err := json.Unmarshal([]byte(record.OpenCodeConfig), &config); err != nil {
						t.Fatal(err)
					}
					permissions := config.Agent["plan"].Permission
					if permissions.Default != "deny" || permissions.External[filepath.ToSlash(secondRepo)+"/**"] != "allow" || permissions.Bash["git -C * diff *"] != "allow" {
						t.Fatalf("wrong repository inspection permissions: %s", record.OpenCodeConfig)
					}
				}
				for _, event := range events {
					if strings.Contains(event.Message, "QA Impact") || strings.Contains(event.Message, "FEATURE") || strings.Contains(event.Message, "PRIVATE THINKING") {
						t.Fatalf("final Markdown or private reasoning leaked into progress: %+v", event)
					}
				}
			})
		}
	}
}

func TestTicketProvidersRejectEmptyMarkdown(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "opencode", "custom"} {
		t.Run(provider, func(t *testing.T) {
			t.Setenv("WLOG_AI_HELPER", "1")
			t.Setenv("WLOG_AI_PROVIDER", provider)
			t.Setenv("WLOG_AI_ARTIFACT", "ticket")
			t.Setenv("WLOG_AI_RESPONSE", " \n\t")
			agent, err := NewFactory().Create(bootstrap.AIConfig{Provider: provider, Providers: map[string]bootstrap.AIProviderConfig{provider: {Command: fakeExecutable(t)}}})
			if err != nil {
				t.Fatal(err)
			}
			_, err = agent.(application.TicketAIAgent).GenerateTicket(context.Background(), application.AIRequest{WorkingDirectory: t.TempDir(), WorklogsOnly: true}, nil)
			if err == nil || !strings.Contains(err.Error(), "empty Jira ticket") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
