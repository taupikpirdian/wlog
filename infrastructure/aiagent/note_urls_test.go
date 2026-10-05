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
	"github.com/taupikpirdian/wlog/domain/dashboard"
)

func TestProvidersEnableNoteURLReadingOnlyWhenPresent(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "opencode", "custom"} {
		for _, ticketOnly := range []bool{false, true} {
			for _, hasURL := range []bool{false, true} {
				t.Run(provider+"/"+map[bool]string{false: "summary", true: "ticket"}[ticketOnly]+"/"+map[bool]string{false: "no-url", true: "url"}[hasURL], func(t *testing.T) {
					directory := t.TempDir()
					recordPath := filepath.Join(directory, "record.json")
					t.Setenv("WLOG_AI_HELPER", "1")
					t.Setenv("WLOG_AI_PROVIDER", provider)
					t.Setenv("WLOG_AI_RECORD", recordPath)
					body := fixtureResponse
					if ticketOnly {
						t.Setenv("WLOG_AI_ARTIFACT", "ticket")
						body = "Ticket title\n\nDescription\nRecorded findings.\n"
					}
					t.Setenv("WLOG_AI_RESPONSE", body)
					agent, err := NewFactory().Create(bootstrap.AIConfig{Provider: provider, Providers: map[string]bootstrap.AIProviderConfig{provider: {Command: fakeExecutable(t)}}})
					if err != nil {
						t.Fatal(err)
					}
					value := application.TicketAIContext{TicketOnly: ticketOnly}
					if hasURL {
						if ticketOnly {
							value.AdditionalContext = "Analysis https://notes.example/report"
						} else {
							value.AllTicketWorklogs.Activities = []dashboard.Activity{{Type: "NOTE", Text: "Analysis https://notes.example/report"}}
						}
					}
					request := application.AIRequest{WorkingDirectory: directory, Context: value, WorklogsOnly: true}
					if ticketOnly {
						_, err = agent.(application.TicketAIAgent).GenerateTicket(context.Background(), request, nil)
					} else {
						_, err = agent.Generate(context.Background(), request, nil)
					}
					if err != nil {
						t.Fatal(err)
					}
					data, err := os.ReadFile(recordPath)
					if err != nil {
						t.Fatal(err)
					}
					var record processRecord
					if err := json.Unmarshal(data, &record); err != nil {
						t.Fatal(err)
					}
					if strings.Contains(record.Input, "open, read and analyze every URL") != hasURL {
						t.Fatal("note URL instructions did not reach the provider")
					}
					args := strings.Join(record.Args, " ")
					switch provider {
					case "codex":
						if strings.Contains(args, `web_search="live"`) != hasURL || !strings.Contains(args, "--sandbox read-only") {
							t.Fatalf("Codex web access or sandbox=%q", args)
						}
					case "claude":
						if strings.Contains(args, "WebFetch(domain:notes.example)") != hasURL || !strings.Contains(args, "--permission-mode plan") || strings.Contains(args, "Bash(curl") {
							t.Fatalf("Claude web access or permissions=%q", args)
						}
					case "opencode":
						var config struct {
							Agent map[string]struct {
								Permission struct {
									Default  string            `json:"*"`
									WebFetch map[string]string `json:"webfetch"`
									Bash     map[string]string `json:"bash"`
								} `json:"permission"`
							} `json:"agent"`
						}
						if err := json.Unmarshal([]byte(record.OpenCodeConfig), &config); err != nil {
							t.Fatal(err)
						}
						permission := config.Agent["plan"].Permission
						if (permission.WebFetch["https://notes.example/report"] == "allow") != hasURL || permission.Default != "deny" || permission.Bash["*"] != "deny" || (hasURL && permission.WebFetch["*"] != "deny") {
							t.Fatalf("OpenCode web access or permissions=%s", record.OpenCodeConfig)
						}
					}
				})
			}
		}
	}
}
