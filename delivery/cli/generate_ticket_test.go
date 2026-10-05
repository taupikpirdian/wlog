package cli

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
	"github.com/taupikpirdian/wlog/domain/dashboard"
	domain "github.com/taupikpirdian/wlog/domain/summary"
)

type skillsStub struct {
	calls int
	found bool
}

func (s *skillsStub) Resolve(_ context.Context, _ string, _ string, name string, progress application.ProgressHandler) (application.TicketSkill, error) {
	s.calls++
	progress(application.ProgressEvent{Type: application.ProgressStatus, Message: "Checking installed AI skills"})
	if s.found {
		progress(application.ProgressEvent{Type: application.ProgressInfo, Message: "Found skill: " + name})
		progress(application.ProgressEvent{Type: application.ProgressInfo, Message: "Reading " + name + " skill instructions"})
		progress(application.ProgressEvent{Type: application.ProgressSuccess, Message: name + " skill loaded by wlog"})
		return application.TicketSkill{Name: name, Path: "/skills/" + name + "/SKILL.md", Loaded: true, Native: true, Invocation: "$" + name}, nil
	}
	progress(application.ProgressEvent{Type: application.ProgressWarning, Message: "ticket-generator skill not found"})
	progress(application.ProgressEvent{Type: application.ProgressInfo, Message: "Using built-in wlog ticket-generation instructions"})
	return application.TicketSkill{}, nil
}

func TestGenerateTicketAliasUsesIdenticalFlow(t *testing.T) {
	for _, language := range []application.OutputLanguage{application.LanguageIndonesian, application.LanguageEnglish} {
		for _, found := range []bool{false, true} {
			var outputs, prompts []string
			var requests []application.AIRequest
			for _, name := range []string{"generate-ticket", "gt"} {
				date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
				reader := &enhancedSummaryStub{value: application.TicketAIContext{TicketKey: "OOT-1", Summary: application.Result{Day: domain.Day{Date: date, Seconds: 7200, HasWorklog: true}}, AllTicketWorklogs: dashboard.Snapshot{Activities: []dashboard.Activity{{TicketKey: "OOT-1", Type: "GIT_COMMIT", Repository: "/database/repo", Hash: "aaa", At: date.Add(time.Hour)}}}}}
				config := &configMemory{config: bootstrap.Config{DataDirectory: "/config", AI: bootstrap.AIConfig{Enabled: true, Provider: "codex", Providers: map[string]bootstrap.AIProviderConfig{"codex": {Command: "codex"}}}}}
				agent := &cliAgentFake{}
				skills := &skillsStub{found: found}
				factory := func(context.Context) (SummaryReader, func() error, error) {
					return reader, func() error { return nil }, nil
				}
				root := NewRootCommand(nil, "test", nil, nil)
				root.AddCommand(NewGenerateTicketCommand(factory, SummaryAIOptions{Config: config, Git: &cliGitFake{}, Agents: &cliFactoryFake{agent: agent}, Skills: skills}))
				var out, progress bytes.Buffer
				root.SetOut(&out)
				root.SetErr(&progress)
				choice := "1"
				if language == application.LanguageEnglish {
					choice = "2"
				}
				root.SetIn(strings.NewReader("1\n" + choice + "\n"))
				root.SetArgs([]string{name})
				if err := root.Execute(); err != nil {
					t.Fatal(err)
				}
				if agent.calls != 1 || agent.ticketCalls != 1 || agent.summaryCalls != 0 || skills.calls != 1 || !agent.request.Context.TicketOnly || agent.request.Skill.Loaded != found {
					t.Fatalf("agent=%d skills=%d request=%+v", agent.calls, skills.calls, agent.request)
				}
				if reader.weekCalls != 1 || reader.dailyCalls != 0 || reader.descriptionCalls != 1 || !agent.request.Context.Summary.Day.Date.IsZero() {
					t.Fatalf("generation must use weekly tickets and whole-ticket context: reader=%+v request=%+v", reader, agent.request)
				}
				if strings.Contains(progress.String(), "Select worklog date") || strings.Contains(progress.String(), "Choose date") || strings.Contains(progress.String(), "Additional context for this summary") || !strings.Contains(progress.String(), "Select ticket:") {
					t.Fatalf("unexpected date selection: %q", progress.String())
				}
				if agent.request.Context.OutputLanguage != language || !strings.Contains(progress.String(), "Select output language:") {
					t.Fatalf("language=%q progress=%q", agent.request.Context.OutputLanguage, progress.String())
				}
				if strings.HasPrefix(out.String(), "# ") || strings.Contains(out.String(), "Time:") || strings.Contains(out.String(), "Checking installed") {
					t.Fatalf("wrong Jira output: %q", out.String())
				}
				if found && (out.String() != skillTicketFixture || strings.Contains(out.String(), "### Background") || !strings.Contains(progress.String(), "✓ ticket-generator skill loaded by wlog")) {
					t.Fatalf("skill artifact or progress changed: %q %q", out.String(), progress.String())
				}
				if !found && !strings.Contains(out.String(), "Description\n") {
					t.Fatal("built-in fallback output lost")
				}
				if !strings.Contains(progress.String(), "✓ Jira ticket generated.") {
					t.Fatalf("progress=%q", progress.String())
				}
				if check, start := strings.Index(progress.String(), "Checking installed AI skills"), strings.Index(progress.String(), "Starting codex"); check < 0 || start < check {
					t.Fatalf("skill check must precede AI analysis: %q", progress.String())
				}
				outputs = append(outputs, out.String())
				prompts = append(prompts, progress.String())
				requests = append(requests, agent.request)
			}
			if outputs[0] != outputs[1] || prompts[0] != prompts[1] || !reflect.DeepEqual(requests[0], requests[1]) {
				t.Fatal("alias changed generation behavior")
			}
		}
	}
}

func TestGenerateTicketPreservesSkillOmissionOfQAImpact(t *testing.T) {
	content := "[REFACTOR] Internal rename\n\nDescription\nNo QA impact — internal change, no observable behavior change.\n\nIn Scope\n- Rename internal helper\n"
	reader := &enhancedSummaryStub{value: application.TicketAIContext{AllTicketWorklogs: dashboard.Snapshot{Activities: []dashboard.Activity{{Type: "GIT_COMMIT", Repository: "/database/repo", Hash: "aaa"}}}}}
	config := &configMemory{config: bootstrap.Config{AI: bootstrap.AIConfig{Enabled: true, Provider: "codex"}}}
	agent := &cliAgentFake{ticketContent: &content}
	cmd := NewGenerateTicketCommand(func(context.Context) (SummaryReader, func() error, error) {
		return reader, func() error { return nil }, nil
	}, SummaryAIOptions{Config: config, Git: &cliGitFake{}, Agents: &cliFactoryFake{agent: agent}, Skills: &skillsStub{found: true}})
	var output, progress bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&progress)
	cmd.SetIn(strings.NewReader("1\n1\n"))
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if output.String() != content || strings.Contains(output.String(), "## QA Impact") {
		t.Fatalf("skill artifact modified: %q", output.String())
	}
}

func TestGenerateTicketNoWeeklyWorklogs(t *testing.T) {
	reader := &enhancedSummaryStub{weeklyTickets: []domain.TicketDay{}}
	agent := &cliAgentFake{}
	closed := false
	cmd := NewGenerateTicketCommand(func(context.Context) (SummaryReader, func() error, error) {
		return reader, func() error { closed = true; return nil }, nil
	}, SummaryAIOptions{Agents: &cliFactoryFake{agent: agent}})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "no ticket worklogs this week") {
		t.Fatalf("error=%v", err)
	}
	if !closed || agent.calls != 0 || reader.descriptionCalls != 0 || strings.Contains(output.String(), "Choose date") || strings.Contains(output.String(), "Choose ticket") {
		t.Fatalf("unexpected work for empty week: reader=%+v output=%q", reader, output.String())
	}
}

func TestGenerateTicketHelpIncludesAlias(t *testing.T) {
	cmd := NewGenerateTicketCommand(nil, SummaryAIOptions{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "generate-ticket, gt") || !strings.Contains(out.String(), "Generate Jira ticket from recorded worklogs and code changes") {
		t.Fatalf("help=%q", out.String())
	}
}
