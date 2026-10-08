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
	calls                     int
	summaryCalls, ticketCalls int
	ticketContent             *string
	request                   application.AIRequest
}

func (a *cliAgentFake) Capabilities() application.AICapabilities { return application.AICapabilities{} }
func (a *cliAgentFake) Generate(_ context.Context, request application.AIRequest, progress application.ProgressHandler) (*application.AIResponse, error) {
	a.calls++
	a.summaryCalls++
	a.request = request
	if progress != nil {
		progress(application.ProgressEvent{Provider: "codex", Type: application.ProgressTool, Message: "Running git diff", RepositoryPath: request.WorkingDirectory})
	}
	return &application.AIResponse{Worklog: application.WorklogText{Details: []string{"AI detail"}, Results: []string{"AI result"}}, TicketDescription: application.TicketDescription{Background: "Background", ProblemRequirement: "Requirement", Scope: []string{"Scope"}, ExpectedResult: "Expected"}}, nil
}

const skillTicketFixture = "[FEATURE] Ticket title\n\nDescription\nRecorded implementation.\n\nGoal\nExpected behavior.\n\nIn Scope\n- Validation\n\nQA Impact\n|| No || Area || Yang dicek || Expected || Dikerjakan oleh ||\n| 1 | Validation | Input kosong | Ditolak | QA Engineer |\n\nAcceptance Criteria\n- Reject invalid input\n"

func (a *cliAgentFake) GenerateTicket(_ context.Context, request application.AIRequest, progress application.ProgressHandler) (*application.GeneratedTicket, error) {
	a.calls++
	a.ticketCalls++
	a.request = request
	if progress != nil {
		progress(application.ProgressEvent{Provider: "codex", Type: application.ProgressTool, Message: "Running git diff", RepositoryPath: request.WorkingDirectory})
	}
	content := "Ticket title\n\nDescription\nRecorded context\n\nScope\n- Validation\n"
	if request.Skill.Loaded {
		content = skillTicketFixture
	}
	if a.ticketContent != nil {
		content = *a.ticketContent
	}
	return &application.GeneratedTicket{Content: content}, nil
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
		{"AI Yes", "5\n1\ny\n1\n\n", true, false, true, false},
		{"AI English", "5\n1\ny\n2\n\n", true, false, true, false},
		{"configure and resume", "5\n1\ny\ny\n1\n\n1\n\n", false, false, true, true},
		{"decline config", "5\n1\ny\nn\n", false, false, false, false},
		{"missing code accepted", "5\n1\ny\n1\n\ny\n", true, true, true, false},
		{"missing code declined", "5\n1\ny\n1\n\nn\n", true, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &enhancedSummaryStub{value: application.TicketAIContext{TicketKey: "OOT-1", Summary: application.Result{Day: domain.Day{Date: date, Seconds: 9000, CommitCount: 1, Details: []string{"Recorded detail"}, HasWorklog: true}, Email: "database@example.com"}, AllTicketWorklogs: dashboard.Snapshot{Activities: []dashboard.Activity{{Type: "GIT_COMMIT", Repository: "/database/repo", Hash: "bbb", At: date.Add(time.Hour)}}}}}
			config := &configMemory{config: bootstrap.Config{DataDirectory: "/config"}}
			if tc.configured {
				config.config.AI = bootstrap.AIConfig{Enabled: true, Provider: "codex", Providers: map[string]bootstrap.AIProviderConfig{"codex": {Command: "codex"}}}
			}
			agent := &cliAgentFake{}
			factory := &cliFactoryFake{agent: agent}
			git := &cliGitFake{missing: tc.missing}
			skills := &skillsStub{found: true}
			cmd := NewSummaryCommand(func(context.Context) (SummaryReader, func() error, error) {
				return reader, func() error { return nil }, nil
			}, SummaryAIOptions{Config: config, Git: git, Agents: factory, Skills: skills})
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
			if !strings.Contains(out.String(), "Time:\n2h 30m (1 commit)\n\nGenerated for Logs:") || !strings.Contains(out.String(), "Dev By:\ndatabase@example.com") {
				t.Fatalf("factual fields changed: %q", out.String())
			}
			if tc.wantAI && (!strings.Contains(out.String(), "Generated for Details Ticket:\n\n"+skillTicketFixture) || agent.summaryCalls != 1 || agent.ticketCalls != 1 || skills.calls != 1 || agent.request.Skill.Name != application.RootCauseSummarySkill) {
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
			if (tc.name == "AI No" || tc.name == "decline config") && (strings.Contains(prompts.String(), "Select output language:") || strings.Contains(prompts.String(), "Additional context for this summary")) {
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

func TestSummaryTicketFallbackAndEmptyArtifact(t *testing.T) {
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	for _, empty := range []bool{false, true} {
		reader := &enhancedSummaryStub{value: application.TicketAIContext{
			TicketKey:         "OOT-1",
			Summary:           application.Result{Day: domain.Day{Date: date, HasWorklog: true}},
			AllTicketWorklogs: dashboard.Snapshot{Activities: []dashboard.Activity{{Type: "GIT_COMMIT", Repository: "/repo", Hash: "bbb", At: date}}},
		}}
		config := &configMemory{config: bootstrap.Config{AI: bootstrap.AIConfig{Enabled: true, Provider: "codex"}}}
		agent := &cliAgentFake{}
		if empty {
			content := " "
			agent.ticketContent = &content
		}
		skills := &skillsStub{}
		cmd := NewSummaryCommand(func(context.Context) (SummaryReader, func() error, error) {
			return reader, func() error { return nil }, nil
		}, SummaryAIOptions{Config: config, Git: &cliGitFake{}, Agents: &cliFactoryFake{agent: agent}, Skills: skills})
		var out, progress bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&progress)
		cmd.SetIn(strings.NewReader("5\n1\ny\n2\n\n"))
		cmd.SetArgs([]string{})
		err := cmd.Execute()
		if empty {
			if err == nil || !strings.Contains(err.Error(), "empty Jira ticket") || strings.Contains(out.String(), "Generated for Logs:") || strings.Contains(progress.String(), "✓ Jira summary generated.") {
				t.Fatalf("empty ticket reported success: error=%v output=%q progress=%q", err, out.String(), progress.String())
			}
		} else if err != nil || !strings.Contains(out.String(), "Generated for Details Ticket:\n\nTicket title\n") || !strings.Contains(progress.String(), "Using built-in wlog ticket-generation instructions") {
			t.Fatalf("fallback: error=%v output=%q progress=%q", err, out.String(), progress.String())
		}
		if skills.calls != 1 || agent.ticketCalls != 1 || agent.summaryCalls != 1 || !agent.request.Context.TicketOnly {
			t.Fatalf("wrong generation pipeline: agent=%+v skills=%+v", agent, skills)
		}
	}
}

func TestAIFormatterControlledFactsAndBullets(t *testing.T) {
	result := application.AIResult{
		Context:  application.TicketAIContext{Summary: application.Result{Day: domain.Day{Seconds: 7200, CommitCount: 3}, Email: "real@example.com"}},
		Response: application.AIResponse{Worklog: application.WorklogText{Details: []string{"Change", "Change"}}},
		Ticket:   application.GeneratedTicket{Content: skillTicketFixture},
	}
	got := formatAISummary(result)
	want := "Time:\n2h (3 commits)\n\nGenerated for Logs:\nDetail:\n- Change\n\nResult:\n-\n\nDev By:\nreal@example.com\n\nGenerated for Details Ticket:\n\n" + skillTicketFixture
	if got != want+"\n\nEnvironment Changes\n\nEnvironment variable check could not be completed.\n" {
		t.Fatalf("got=%q want=%q", got, want)
	}
}

func TestSkillEnvironmentSelection(t *testing.T) {
	for _, tc := range []struct {
		name, summarySkill, ticketSkill, want string
		ticketOnly, invalid                   bool
	}{
		{name: "summary default", want: "root-cause-summary"},
		{name: "summary override", summarySkill: " ticket-generator ", ticketSkill: "invalid-other-command", want: "ticket-generator"},
		{name: "ticket default", summarySkill: "invalid-other-command", want: "ticket-generator", ticketOnly: true},
		{name: "ticket override", ticketSkill: "root-cause-summary", want: "root-cause-summary", ticketOnly: true},
		{name: "invalid summary", summarySkill: "../invalid", invalid: true},
		{name: "invalid ticket", ticketSkill: "unknown", ticketOnly: true, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("WLOG_SUMMARY_SKILL", tc.summarySkill)
			t.Setenv("WLOG_TICKET_SKILL", tc.ticketSkill)
			date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			reader := &enhancedSummaryStub{value: application.TicketAIContext{Summary: application.Result{Day: domain.Day{Date: date}}, AllTicketWorklogs: dashboard.Snapshot{Activities: []dashboard.Activity{{Type: "GIT_COMMIT", Repository: "/repo", Hash: "aaa", At: date}}}}}
			config := &configMemory{config: bootstrap.Config{AI: bootstrap.AIConfig{Enabled: true, Provider: "codex"}}}
			agent := &cliAgentFake{}
			skills := &skillsStub{found: true}
			open := func(context.Context) (SummaryReader, func() error, error) {
				return reader, func() error { return nil }, nil
			}
			options := SummaryAIOptions{Config: config, Git: &cliGitFake{}, Agents: &cliFactoryFake{agent: agent}, Skills: skills}
			cmd := NewSummaryCommand(open, options)
			input := "5\n1\ny\n1\n\n"
			if tc.ticketOnly {
				cmd = NewGenerateTicketCommand(open, options)
				input = "1\n1\n"
			}
			var out, progress bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&progress)
			cmd.SetIn(strings.NewReader(input))
			cmd.SetArgs([]string{})
			err := cmd.Execute()
			if tc.invalid {
				if err == nil || !strings.Contains(err.Error(), "unsupported AI skill") || agent.calls != 0 || skills.calls != 0 {
					t.Fatalf("err=%v agent=%+v skills=%+v", err, agent, skills)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if agent.request.Skill.Name != tc.want || agent.request.Skill.Invocation != "$"+tc.want || agent.request.SummaryDetails == tc.ticketOnly {
				t.Fatalf("wrong skill or task: %+v", agent.request)
			}
			prompt, err := application.BuildTicketPrompt(agent.request)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(prompt, "Native skill requested: $"+tc.want) {
				t.Fatal("selected skill missing in prompt")
			}
		})
	}
}
