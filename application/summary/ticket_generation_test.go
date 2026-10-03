package summary_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
)

type ticketAgentStub struct {
	requests     []application.AIRequest
	result       *application.GeneratedTicket
	err          error
	summaryCalls int
}

func (a *ticketAgentStub) Capabilities() application.AICapabilities {
	return application.AICapabilities{}
}
func (a *ticketAgentStub) Generate(context.Context, application.AIRequest, application.ProgressHandler) (*application.AIResponse, error) {
	a.summaryCalls++
	return nil, errors.New("ticket generation must not use summary JSON")
}
func (a *ticketAgentStub) GenerateTicket(_ context.Context, request application.AIRequest, _ application.ProgressHandler) (*application.GeneratedTicket, error) {
	a.requests = append(a.requests, request)
	return a.result, a.err
}

type ticketFactoryStub struct {
	agent *ticketAgentStub
	calls int
}

func (f *ticketFactoryStub) Create(bootstrap.AIConfig) (application.AIAgent, error) {
	f.calls++
	return f.agent, nil
}

type ticketSkillsStub struct {
	paths      []string
	loadedPath string
	err        error
}

func (s *ticketSkillsStub) Resolve(_ context.Context, _ string, path string, progress application.ProgressHandler) (application.TicketSkill, error) {
	s.paths = append(s.paths, path)
	if s.err != nil {
		return application.TicketSkill{}, s.err
	}
	if path != s.loadedPath {
		return application.TicketSkill{}, nil
	}
	return application.TicketSkill{Loaded: true, Native: true, Path: path + "/.agents/skills/ticket-generator/SKILL.md", Invocation: "$ticket-generator"}, nil
}

func TestGenerateOneSkillTicketFromAllRepositories(t *testing.T) {
	content := "\n# [FEATURE] Ticket title\n\n## Description\nCaptured changes.\n\n## QA Impact\n| No | Area | What to check | Expected | Owner |\n| 1 | Auth | Empty input | Rejected | QA Engineer |\n\n"
	value := application.TicketAIContext{TicketKey: "OOT-1", OutputLanguage: application.LanguageEnglish, Repositories: []application.RepositoryAIContext{
		{Path: "/repoA", Changes: []application.CodeChange{{CommitRange: application.CommitRange{Start: "aaa", End: "bbb"}, Diff: "+first change"}}},
		{Path: "/repoB", Changes: []application.CodeChange{{CommitRange: application.CommitRange{Start: "ccc", End: "ddd"}, Diff: "+second change"}}},
	}}
	factory := &ticketFactoryStub{agent: &ticketAgentStub{result: &application.GeneratedTicket{Content: content}}}
	skills := &ticketSkillsStub{loadedPath: "/repoB"}
	result, err := application.GenerateTicketAI(context.Background(), value, bootstrap.AIConfig{Provider: "codex"}, factory, false, "/cwd-is-not-a-repo", skills, nil)
	if err != nil || result.Content != content || len(factory.agent.requests) != 1 || factory.agent.summaryCalls != 0 {
		t.Fatalf("result=%+v error=%v requests=%v", result, err, factory.agent.requests)
	}
	request := factory.agent.requests[0]
	if !reflect.DeepEqual(skills.paths, []string{"/repoA", "/repoB"}) || request.WorkingDirectory != "/repoB" || !request.Skill.Loaded || !request.Context.TicketOnly || len(request.Context.Repositories) != 2 {
		t.Fatalf("skills=%+v request=%+v", skills, request)
	}
	prompt, err := application.BuildTicketPrompt(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"first change", "second change", "aaa", "ddd", "Generate ONE coherent ticket", "TICKET-GENERATOR IS ACTIVE", "output_language: en"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("missing context/instruction %q", required)
		}
	}
	if strings.Contains(prompt, application.ResponseSchema) || strings.Contains(prompt, "### Background") {
		t.Fatal("skill ticket uses a custom output template")
	}
}

func TestTicketSkillFailureFallsBackExplicitly(t *testing.T) {
	factory := &ticketFactoryStub{agent: &ticketAgentStub{result: &application.GeneratedTicket{Content: "# Fallback title\n\n### Background\nRecorded context\n"}}}
	skills := &ticketSkillsStub{err: errors.New("permission denied")}
	var events []application.ProgressEvent
	result, err := application.GenerateTicketAI(context.Background(), application.TicketAIContext{}, bootstrap.AIConfig{Provider: "codex"}, factory, true, "/recorded-data", skills, func(e application.ProgressEvent) { events = append(events, e) })
	if err != nil || !strings.Contains(result.Content, "Fallback title") || factory.agent.requests[0].Skill.Loaded {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	prompt, err := application.BuildTicketPrompt(factory.agent.requests[0])
	if err != nil || !strings.Contains(prompt, "NO TICKET-GENERATOR SKILL WAS LOADED") || !strings.Contains(prompt, "### Background") || !strings.Contains(prompt, "Do not state that implementation details were verified") {
		t.Fatalf("fallback prompt=%s error=%v", prompt, err)
	}
	if len(events) < 3 || events[0].Type != application.ProgressWarning || events[0].Message != "Failed to load ticket-generator skill" || events[1].Message != "Using built-in wlog ticket generator" {
		t.Fatalf("silent skill failure: %+v", events)
	}
}

func TestGeneratedTicketValidationPreservesNonemptyContent(t *testing.T) {
	for _, ticket := range []*application.GeneratedTicket{nil, {}, {Content: " \n\t"}} {
		if err := application.ValidateGeneratedTicket(ticket); err == nil {
			t.Fatal("empty result accepted")
		}
	}
	content := "  # Custom skill title\n\n## Custom section\n| a | b |\n|---|---|\n| 1 | 2 |\n\n"
	ticket := &application.GeneratedTicket{Content: content}
	if err := application.ValidateGeneratedTicket(ticket); err != nil || ticket.Content != content {
		t.Fatalf("artifact modified or rejected: %+v error=%v", ticket, err)
	}
}

func TestTicketGenerationConsentAndEmptyResponse(t *testing.T) {
	factory := &ticketFactoryStub{agent: &ticketAgentStub{result: &application.GeneratedTicket{}}}
	if _, err := application.GenerateTicketAI(context.Background(), application.TicketAIContext{}, bootstrap.AIConfig{}, factory, false, "/data", nil, nil); err == nil || factory.calls != 0 {
		t.Fatalf("missing source consent accepted: error=%v calls=%d", err, factory.calls)
	}
	if _, err := application.GenerateTicketAI(context.Background(), application.TicketAIContext{}, bootstrap.AIConfig{}, factory, true, "/data", nil, nil); err == nil || !strings.Contains(err.Error(), "empty Jira ticket") {
		t.Fatalf("empty result accepted: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := application.GenerateTicketAI(ctx, application.TicketAIContext{}, bootstrap.AIConfig{}, factory, true, "/data", nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled generation continued: %v", err)
	}
}
