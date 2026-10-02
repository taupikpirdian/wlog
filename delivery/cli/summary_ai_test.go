package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
	"github.com/taupikpirdian/wlog/domain/dashboard"
	domain "github.com/taupikpirdian/wlog/domain/summary"
)

type enhancedSummaryStub struct {
	value                                   application.TicketAIContext
	selected                                string
	weeklyTickets                           []domain.TicketDay
	weekCalls, dailyCalls, descriptionCalls int
}

func (s *enhancedSummaryStub) Week(context.Context) ([]domain.Day, error) {
	dates, _ := domain.WeekDates(s.value.Summary.Day.Date, time.UTC)
	days := make([]domain.Day, len(dates))
	for i, date := range dates {
		days[i] = domain.Day{Date: date}
	}
	days[4] = s.value.Summary.Day
	return days, nil
}
func (s *enhancedSummaryStub) Summary(context.Context, time.Time) (application.Result, error) {
	return s.value.Summary, nil
}
func (s *enhancedSummaryStub) Tickets(context.Context, time.Time) ([]domain.TicketDay, error) {
	s.dailyCalls++
	return []domain.TicketDay{{Key: "OOT-1", Day: s.value.Summary.Day}, {Key: "OOT-2", Day: domain.Day{Seconds: 10800}}}, nil
}
func (s *enhancedSummaryStub) WeekTickets(context.Context) ([]domain.TicketDay, error) {
	s.weekCalls++
	if s.weeklyTickets != nil {
		return s.weeklyTickets, nil
	}
	return []domain.TicketDay{{Key: "OOT-1", Day: s.value.Summary.Day}, {Key: "OOT-2", Day: domain.Day{Seconds: 10800}}}, nil
}
func (s *enhancedSummaryStub) TicketDescriptionContext(_ context.Context, key string) (application.TicketAIContext, error) {
	s.descriptionCalls++
	s.selected = key
	value := s.value
	value.Summary = application.Result{}
	value.TicketOnly = true
	return value, nil
}
func (s *enhancedSummaryStub) TicketSummary(_ context.Context, _ time.Time, key string) (application.Result, error) {
	s.selected = key
	return s.value.Summary, nil
}
func (s *enhancedSummaryStub) TicketContext(_ context.Context, _ time.Time, key string) (application.TicketAIContext, error) {
	s.selected = key
	return s.value, nil
}

type configMemory struct {
	config bootstrap.Config
	saves  int
}

func (c *configMemory) LoadOrCreate(context.Context) (bootstrap.Config, error) { return c.config, nil }
func (c *configMemory) SaveAI(_ context.Context, ai bootstrap.AIConfig) error {
	if err := application.ValidateAIConfig(ai); err != nil {
		return err
	}
	c.config.AI = ai
	c.saves++
	return nil
}

type cliGitFake struct {
	missing bool
	calls   int
}

func (g *cliGitFake) ValidateRepository(_ context.Context, path string) (string, error) {
	g.calls++
	if g.missing {
		return "", errors.New("repository not found")
	}
	return path, nil
}
func (g *cliGitFake) Change(context.Context, string, string) (application.CodeChange, error) {
	return application.CodeChange{CommitRange: application.CommitRange{Start: "aaa", End: "bbb"}, Diff: "+ validation"}, nil
}

type cliAgentFake struct {
	calls   int
	request application.AIRequest
}

func (a *cliAgentFake) Capabilities() application.AICapabilities { return application.AICapabilities{} }
func (a *cliAgentFake) Generate(_ context.Context, request application.AIRequest, progress application.ProgressHandler) (*application.AIResponse, error) {
	a.calls++
	a.request = request
	if progress != nil {
		progress(application.ProgressEvent{Provider: "codex", Type: application.ProgressTool, Message: "Running git diff", RepositoryPath: request.WorkingDirectory})
	}
	return &application.AIResponse{Worklog: application.WorklogText{Details: []string{"AI detail"}, Results: []string{"AI result"}}, TicketDescription: application.TicketDescription{Background: "Background", ProblemRequirement: "Requirement", Scope: []string{"Scope"}, ExpectedResult: "Expected"}}, nil
}

type cliFactoryFake struct {
	agent *cliAgentFake
	calls int
}

func (f *cliFactoryFake) Create(bootstrap.AIConfig) (application.AIAgent, error) {
	f.calls++
	return f.agent, nil
}

func TestEnhancedSummaryFlows(t *testing.T) {
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, input         string
		configured, missing bool
		wantAI, wantSave    bool
	}{
		{"AI No", "5\n1\nn\n", true, false, false, false},
		{"AI Yes", "5\n1\ny\n1\n", true, false, true, false},
		{"AI English", "5\n1\ny\n2\n", true, false, true, false},
		{"configure and resume", "5\n1\ny\ny\n1\n\n1\n", false, false, true, true},
		{"decline config", "5\n1\ny\nn\n", false, false, false, false},
		{"missing code accepted", "5\n1\ny\n1\ny\n", true, true, true, false},
		{"missing code declined", "5\n1\ny\n1\nn\n", true, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &enhancedSummaryStub{value: application.TicketAIContext{TicketKey: "OOT-1", Summary: application.Result{Day: domain.Day{Date: date, Seconds: 9000, Details: []string{"Recorded detail"}, HasWorklog: true}, Email: "database@example.com"}, AllTicketWorklogs: dashboard.Snapshot{Activities: []dashboard.Activity{{Type: "GIT_COMMIT", Repository: "/database/repo", Hash: "bbb", At: date.Add(time.Hour)}}}}}
			config := &configMemory{config: bootstrap.Config{DataDirectory: "/config"}}
			if tc.configured {
				config.config.AI = bootstrap.AIConfig{Enabled: true, Provider: "codex", Providers: map[string]bootstrap.AIProviderConfig{"codex": {Command: "codex"}}}
			}
			agent := &cliAgentFake{}
			factory := &cliFactoryFake{agent: agent}
			git := &cliGitFake{missing: tc.missing}
			cmd := NewSummaryCommand(func(context.Context) (SummaryReader, func() error, error) {
				return reader, func() error { return nil }, nil
			}, SummaryAIOptions{Config: config, Git: git, Agents: factory})
			var out, prompts bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&prompts)
			cmd.SetIn(strings.NewReader(tc.input))
			cmd.SetArgs([]string{})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if (agent.calls > 0) != tc.wantAI || (config.saves > 0) != tc.wantSave || reader.selected != "OOT-1" {
				t.Fatalf("AI=%d saved=%d selected=%s", agent.calls, config.saves, reader.selected)
			}
			if !tc.wantAI && (factory.calls != 0 || !strings.Contains(out.String(), "- Recorded detail") || strings.Contains(out.String(), "### Background")) {
				t.Fatalf("non-AI flow changed: %q", out.String())
			}
			if tc.name == "AI No" && git.calls != 0 {
				t.Fatal("non-AI flow queried code")
			}
			if !strings.Contains(out.String(), "Time:\n2h 30m") || !strings.Contains(out.String(), "Dev By:\ndatabase@example.com") {
				t.Fatalf("factual fields changed: %q", out.String())
			}
			if tc.wantAI && !strings.Contains(out.String(), "### Background\nBackground") {
				t.Fatalf("AI output=%q", out.String())
			}
			if tc.wantAI {
				wantLanguage := application.LanguageIndonesian
				if tc.name == "AI English" {
					wantLanguage = application.LanguageEnglish
				}
				if agent.request.Context.OutputLanguage != wantLanguage || !strings.Contains(prompts.String(), "Select output language:") {
					t.Fatalf("language=%q prompts=%q", agent.request.Context.OutputLanguage, prompts.String())
				}
			}
			if (tc.name == "AI No" || tc.name == "decline config") && strings.Contains(prompts.String(), "Select output language:") {
				t.Fatal("non-AI flow asked for an AI output language")
			}
			if tc.wantAI && (!strings.Contains(prompts.String(), "[Codex] → Running git diff") || !strings.Contains(prompts.String(), "✓ Jira summary generated.")) {
				t.Fatalf("missing progress: %q", prompts.String())
			}
			if strings.Contains(out.String(), "[Codex]") || strings.Contains(out.String(), "Loading ticket context") {
				t.Fatalf("progress leaked into stdout: %q", out.String())
			}
			if strings.Contains(out.String(), "https://github.com/taupikpirdian/wlog") || !strings.HasSuffix(prompts.String(), "https://github.com/taupikpirdian/wlog\n") || strings.Count(prompts.String(), "https://github.com/taupikpirdian/wlog") != 1 {
				t.Fatalf("CTA must appear once at the end of stderr: stdout=%q stderr=%q", out.String(), prompts.String())
			}
			if tc.wantAI && tc.missing && !agent.request.WorklogsOnly {
				t.Fatal("missing-code consent not passed")
			}
			if tc.wantAI && !tc.missing && agent.request.WorkingDirectory != "/database/repo" {
				t.Fatalf("agent cwd=%s", agent.request.WorkingDirectory)
			}
		})
	}
}

func TestConfigAICommandCustomArguments(t *testing.T) {
	config := &configMemory{}
	cmd := NewConfigCommand(config)
	var out bytes.Buffer
	cmd.SetErr(&out)
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("4\nmy-agent\n[\"--prompt\",\"{{prompt}}\"]\n"))
	cmd.SetArgs([]string{"ai"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if config.config.AI.Provider != "custom" || config.config.AI.Providers["custom"].Command != "my-agent" || len(config.config.AI.Providers["custom"].Args) != 2 {
		t.Fatalf("config=%+v", config.config.AI)
	}
}

func TestAIFormatterControlledFactsAndBullets(t *testing.T) {
	result := application.AIResult{Context: application.TicketAIContext{TicketKey: "OOT-1", Summary: application.Result{Day: domain.Day{Date: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC), Seconds: 7200}, Email: "real@example.com"}}, Response: application.AIResponse{Worklog: application.WorklogText{Details: []string{"Change", "Change"}, Results: []string{}}, TicketDescription: application.TicketDescription{Background: "Background", ProblemRequirement: "Requirement", Scope: []string{"Change"}, ExpectedResult: "Expected", TechnicalNotes: ""}}}
	got := formatAISummary(result)
	want := "Time:\n2h\n\nDetail:\n- Change\n\nHasil:\n-\n\nDev By:\nreal@example.com\n\n### Background\nBackground\n\n### Problem / Requirement\nRequirement\n\n### Scope\n- Change\n\n### Expected Result\nExpected\n"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}
