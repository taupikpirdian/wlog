package summary_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
	"github.com/taupikpirdian/wlog/domain/dashboard"
	"github.com/taupikpirdian/wlog/domain/session"
	domain "github.com/taupikpirdian/wlog/domain/summary"
)

const validResponse = `{"worklog":{"details":["Implement validation"],"results":[]},"ticket_description":{"background":"Recorded implementation","problem_requirement":"Validate input","scope":["Validation"],"expected_result":"Invalid input is rejected","technical_notes":""}}`

func TestAIResponseValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", validResponse, true},
		{"invalid JSON", "{", false},
		{"missing field", strings.Replace(validResponse, `"background":"Recorded implementation",`, "", 1), false},
		{"empty scope", strings.Replace(validResponse, `["Validation"]`, `[]`, 1), false},
		{"missing results", strings.Replace(validResponse, `,"results":[]`, "", 1), false},
		{"empty details", strings.Replace(validResponse, `["Implement validation"]`, `[]`, 1), false},
		{"invented duration", strings.Replace(validResponse, `"worklog":{`, `"worklog":{"duration":"999h",`, 1), false},
		{"invented ticket", strings.Replace(validResponse, `{"worklog"`, `{"ticket_key":"OTHER-1","worklog"`, 1), false},
		{"extra text", validResponse + " explanation", false},
		{"blank entry", strings.Replace(validResponse, `["Validation"]`, `[" "]`, 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := application.ParseAIResponse([]byte(tc.body))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

type gitFake struct{ calls []string }

func (g *gitFake) ValidateRepository(_ context.Context, path string) (string, error) {
	if path == "/missing" {
		return "", errors.New("repository not found")
	}
	return path, nil
}
func (g *gitFake) Change(_ context.Context, path, hash string) (application.CodeChange, error) {
	g.calls = append(g.calls, path+":"+hash)
	if hash == "bad" {
		return application.CodeChange{}, errors.New("commit not found")
	}
	return application.CodeChange{CommitRange: application.CommitRange{Start: hash + "parent", End: hash}, Diff: "+ actual implementation " + hash}, nil
}

func TestCodeContextMultipleRepositoriesDedupAndDegraded(t *testing.T) {
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	value := application.TicketAIContext{TicketKey: "OOT-1", Summary: application.Result{Day: domain.Day{Date: date}}, AllTicketWorklogs: dashboard.Snapshot{Activities: []dashboard.Activity{
		{Type: "GIT_COMMIT", Repository: "/repoA", Hash: "aaa", At: date.Add(time.Hour), Branch: "feature/OOT-1"},
		{Type: "GIT_COMMIT", Repository: "/repoA", Hash: "aaa", At: date.Add(2 * time.Hour)},
		{Type: "GIT_COMMIT", Repository: "/repoB", Hash: "bbb", At: date.Add(-time.Hour)},
		{Type: "GIT_COMMIT", Repository: "/repoB", Hash: "bad", At: date.Add(time.Hour)},
		{Type: "GIT_COMMIT", Repository: "/missing", Hash: "ccc", At: date.Add(time.Hour)},
	}}}
	got, err := application.CollectCode(context.Background(), value, &gitFake{})
	if err != nil || len(got.Repositories) != 2 || len(got.Repositories[0].Changes) != 1 || !got.Repositories[0].Changes[0].SelectedDate || got.Repositories[1].Changes[0].SelectedDate || len(got.Warnings) != 2 {
		t.Fatalf("context=%+v err=%v", got, err)
	}
	prompt, err := application.BuildAIPrompt(application.AIRequest{Context: got, Repository: &got.Repositories[0]})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"ACTUAL SOURCE CODE CHANGE", "Source-code context is incomplete", "Do not invent", "Do not claim deployment", "selected date + selected ticket", "+ actual implementation aaa"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("prompt missing %q", required)
		}
	}
	if strings.Contains(prompt, "+ actual implementation bbb") {
		t.Fatal("other repository diff leaked into per-repository prompt")
	}
}

type fakeAgent struct {
	requests []application.AIRequest
	err      error
	body     string
}

func (a *fakeAgent) Capabilities() application.AICapabilities { return application.AICapabilities{} }
func (a *fakeAgent) Generate(_ context.Context, r application.AIRequest, _ application.ProgressHandler) (*application.AIResponse, error) {
	a.requests = append(a.requests, r)
	if a.err != nil {
		return nil, a.err
	}
	body := a.body
	if body == "" {
		body = validResponse
	}
	return application.ParseAIResponse([]byte(body))
}

type fakeFactory struct {
	agent *fakeAgent
	err   error
	calls int
}

func (f *fakeFactory) Create(bootstrap.AIConfig) (application.AIAgent, error) {
	f.calls++
	return f.agent, f.err
}

func TestGenerateAIRepositoryDirectoriesAndConsent(t *testing.T) {
	value := application.TicketAIContext{TicketKey: "OOT-1", Repositories: []application.RepositoryAIContext{{Path: "/repoA"}, {Path: "/repoB"}}}
	factory := &fakeFactory{agent: &fakeAgent{}}
	result, err := application.GenerateAI(context.Background(), value, bootstrap.AIConfig{}, factory, false, "/config")
	if err != nil || len(factory.agent.requests) != 2 || factory.agent.requests[0].WorkingDirectory != "/repoA" || factory.agent.requests[1].WorkingDirectory != "/repoB" || result.Context.TicketKey != "OOT-1" {
		t.Fatalf("result=%+v requests=%+v err=%v", result, factory.agent.requests, err)
	}
	value.Repositories = nil
	factory.agent.requests = nil
	if _, err := application.GenerateAI(context.Background(), value, bootstrap.AIConfig{}, factory, false, "/config"); err == nil {
		t.Fatal("missing consent accepted")
	}
	result, err = application.GenerateAI(context.Background(), value, bootstrap.AIConfig{}, factory, true, "/config")
	if err != nil || len(factory.agent.requests) != 1 || !factory.agent.requests[0].WorklogsOnly || factory.agent.requests[0].WorkingDirectory != "/config" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	prompt, _ := application.BuildAIPrompt(factory.agent.requests[0])
	if !strings.Contains(prompt, "No source code is available") || !strings.Contains(prompt, "Do NOT claim you inspected") {
		t.Fatal("degraded guard missing")
	}
}

func TestTicketContextSeparatesSelectedDayAndWholeTicket(t *testing.T) {
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	s := &store{snapshot: dashboard.Snapshot{Sessions: []session.Session{
		{ID: 1, TicketKey: "OOT-1", Title: "Yesterday", Status: session.Completed, StartedAt: date.Add(-2 * time.Hour), EndedAt: timePtr(date.Add(-time.Hour))},
		{ID: 2, TicketKey: "OOT-1", Title: "Today", Status: session.Completed, StartedAt: date.Add(time.Hour), EndedAt: timePtr(date.Add(2 * time.Hour))},
		{ID: 3, TicketKey: "OOT-2", Title: "Other ticket", Status: session.Completed, StartedAt: date.Add(3 * time.Hour), EndedAt: timePtr(date.Add(4 * time.Hour))},
	}}}
	reader := application.NewService(s, &emailReader{value: "dev@example.com"}, func() time.Time { return date.Add(36 * time.Hour) }, time.UTC)
	value, err := reader.TicketContext(context.Background(), date, "OOT-1")
	if err != nil || value.Summary.Day.Seconds != 3600 || len(value.Summary.Day.Details) != 1 || value.Summary.Day.Details[0] != "Today" || len(value.AllTicketWorklogs.Sessions) != 2 {
		t.Fatalf("context=%+v err=%v", value, err)
	}
	prompt, _ := application.BuildAIPrompt(application.AIRequest{Context: value, WorklogsOnly: true})
	if strings.Contains(prompt, "Other ticket") || !strings.Contains(prompt, "Yesterday") {
		t.Fatal("wrong ticket scope")
	}
	if _, err := reader.TicketSummary(context.Background(), date, "OOT-404"); err == nil {
		t.Fatal("missing ticket accepted")
	}
}
func timePtr(t time.Time) *time.Time { return &t }

func TestTicketPromptFollowsSkillWithoutForcingWlogFormat(t *testing.T) {
	for _, native := range []bool{true, false} {
		skill := application.TicketSkill{Name: "ticket-generator", Path: "/skills/ticket-generator/SKILL.md", Loaded: true, Native: native, Invocation: "$ticket-generator"}
		if !native {
			skill.Instructions = "FULL SKILL METHODOLOGY\nReconstruct scope from actual changes."
		}
		prompt, err := application.BuildAIPrompt(application.AIRequest{Context: application.TicketAIContext{TicketOnly: true}, Skill: skill})
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range []string{"Read the ticket-generator skill instructions before analyzing code changes", "applicable section structure, subject to the Jira output rules", "QA Impact", "Do NOT convert", "output_language: id", "Do not invent base_branch/doc_path"} {
			if !strings.Contains(prompt, text) {
				t.Fatalf("missing instruction %q", text)
			}
		}
		for _, excluded := range []string{application.ResponseSchema, "### Background", "### Problem / Requirement", "### Scope", "### Expected Result", "### Technical Notes"} {
			if strings.Contains(prompt, excluded) {
				t.Fatalf("skill prompt forces legacy structure: %q", excluded)
			}
		}
		if native && (!strings.Contains(prompt, "Native skill requested: $ticket-generator") || strings.Contains(prompt, "TICKET-GENERATOR INSTRUCTIONS (read completely")) {
			t.Fatal("native skill should be requested, not pasted")
		}
		if !native && !strings.Contains(prompt, skill.Instructions) {
			t.Fatal("custom agent did not receive loaded methodology")
		}
	}
	prompt, _ := application.BuildAIPrompt(application.AIRequest{Context: application.TicketAIContext{TicketOnly: true}})
	if !strings.Contains(prompt, "NO TICKET-GENERATOR SKILL WAS LOADED") || !strings.Contains(prompt, "built-in") || !strings.Contains(prompt, "Description\nConservative recorded context.") || strings.Contains(prompt, application.ResponseSchema) {
		t.Fatal("missing fallback instructions")
	}
	prompt, _ = application.BuildAIPrompt(application.AIRequest{})
	if strings.Contains(prompt, "TICKET-GENERATOR IS ACTIVE") {
		t.Fatal("summary prompt changed")
	}
}

func TestTicketPromptHasNoSelectedDateScope(t *testing.T) {
	value := application.TicketAIContext{TicketKey: "OOT-1", TicketOnly: true, AllTicketWorklogs: dashboard.Snapshot{Activities: []dashboard.Activity{
		{TicketKey: "OOT-1", Type: "NOTE", Text: "Previous week analysis", At: time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)},
		{TicketKey: "OOT-1", Type: "NOTE", Text: "Current week implementation", At: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)},
	}}}
	prompt, err := application.BuildAIPrompt(application.AIRequest{Context: value})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"There is NO selected date", "not the scope of generation", "Previous week analysis", "Current week implementation"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("missing ticket-wide instruction %q", required)
		}
	}
	for _, excluded := range []string{"selected date + selected ticket", "SelectedDate=true", `"Summary":`, "0001-01-01"} {
		if strings.Contains(prompt, excluded) {
			t.Fatalf("date-scoped summary context leaked into ticket generation: %q", excluded)
		}
	}
}
