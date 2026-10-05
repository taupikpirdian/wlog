package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
	"github.com/taupikpirdian/wlog/delivery/cli"
	"github.com/taupikpirdian/wlog/infrastructure/gitevidence"
	"github.com/taupikpirdian/wlog/infrastructure/storage"
)

type integrationConfig struct{ value bootstrap.Config }

func (c *integrationConfig) LoadOrCreate(context.Context) (bootstrap.Config, error) {
	return c.value, nil
}
func (c *integrationConfig) SaveAI(_ context.Context, ai bootstrap.AIConfig) error {
	c.value.AI = ai
	return nil
}

type integrationAgent struct{ requests []application.AIRequest }

func (a *integrationAgent) GenerateTicket(_ context.Context, request application.AIRequest, progress application.ProgressHandler) (*application.GeneratedTicket, error) {
	a.requests = append(a.requests, request)
	for _, repo := range request.Context.Repositories {
		progress(application.ProgressEvent{Provider: "custom", Type: application.ProgressTool, RepositoryPath: repo.Path, Message: "Inspecting supplied diff"})
	}
	return &application.GeneratedTicket{Content: "# Recorded ticket title\n\n### Background\nRecorded changes\n\n### Scope\n- Update value\n"}, nil
}

func (a *integrationAgent) Capabilities() application.AICapabilities {
	return application.AICapabilities{SupportsEventStream: true}
}
func (a *integrationAgent) Generate(_ context.Context, request application.AIRequest, progress application.ProgressHandler) (*application.AIResponse, error) {
	a.requests = append(a.requests, request)
	progress(application.ProgressEvent{Provider: "custom", Type: application.ProgressTool, RepositoryPath: request.WorkingDirectory, Message: "Inspecting supplied diff"})
	return &application.AIResponse{Worklog: application.WorklogText{Details: []string{"Observed change"}, Results: []string{}, EnvironmentVariables: []string{"CAPTURED_ENV", "EXISTING_ENV", "HEAD_ONLY_ENV"}}, TicketDescription: application.TicketDescription{Background: "Recorded changes", ProblemRequirement: "Observed requirement", Scope: []string{"Update value"}, ExpectedResult: "New value"}}, nil
}

type integrationAgents struct{ agent *integrationAgent }

func (f *integrationAgents) Create(bootstrap.AIConfig) (application.AIAgent, error) {
	return f.agent, nil
}

func aiRepository(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=Fixture", "-c", "user.email=fixture@example.com", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
		cmd.Dir = directory
		body, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, body, err)
		}
		return strings.TrimSpace(string(body))
	}
	run("init", "--quiet")
	file := filepath.Join(directory, "value.go")
	envFile := filepath.Join(directory, ".env.example")
	if err := os.WriteFile(file, []byte("package fixture\nconst Value = 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("EXISTING_ENV=old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "value.go", ".env.example")
	run("commit", "--quiet", "-m", "Initial")
	if err := os.WriteFile(file, []byte("package fixture\nconst Value = 2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("EXISTING_ENV=new\nCAPTURED_ENV=private-fixture-value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "value.go", ".env.example")
	run("commit", "--quiet", "-m", "Captured ticket change")
	hash := run("rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("package fixture\nconst Value = 999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("EXISTING_ENV=new\nCAPTURED_ENV=private-fixture-value\nHEAD_ONLY_ENV=placeholder\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", "value.go", ".env.example")
	run("commit", "--quiet", "-m", "Uncaptured HEAD")
	return directory, hash
}

func TestAISummaryReadsDatabaseRepositoriesOutsideCurrentDirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git unavailable")
	}
	repoA, hashA := aiRepository(t)
	repoB, hashB := aiRepository(t)
	directory := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	global := filepath.Join(directory, "global.gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n email = developer@example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "0")
	path := filepath.Join(directory, "worklog.db")
	db, err := storage.OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO tickets(id,ticket_key) VALUES(1,'OOT-1')`); err != nil {
		t.Fatal(err)
	}
	for i, item := range []struct{ path, hash, start, end string }{{repoA, hashA, "2026-10-02T09:00:00Z", "2026-10-02T10:00:00Z"}, {repoB, hashB, "2026-10-02T11:00:00Z", "2026-10-02T12:00:00Z"}} {
		id := i + 1
		if _, err := db.Exec(`INSERT INTO work_sessions(id,ticket_id,title,repository,started_at,ended_at,status,duration_seconds) VALUES(?,1,'Recorded work',?,?,?,'COMPLETED',3600)`, id, item.path, item.start, item.end); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO work_activities(ticket_id,session_id,type,repository,branch,commit_hash,commit_message,created_at) VALUES(1,?,'GIT_COMMIT',?,'feature/OOT-1',?,'Recorded change',?)`, id, item.path, item.hash, item.end); err != nil {
			t.Fatal(err)
		}
	}
	config := &integrationConfig{value: bootstrap.Config{DatabasePath: path, DataDirectory: directory, AI: bootstrap.AIConfig{Enabled: true, Provider: "custom", Providers: map[string]bootstrap.AIProviderConfig{"custom": {Command: "fixture"}}}}}
	agent := &integrationAgent{}
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	factory := summaryFactory(config, func() time.Time { return now }, time.UTC)
	root := cli.NewRootCommand(nil, "test", nil, nil)
	root.AddCommand(cli.NewSummaryCommand(factory, cli.SummaryAIOptions{Config: config, Git: gitevidence.NewService(), Agents: &integrationAgents{agent: agent}}))
	var out, progress bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&progress)
	root.SetIn(strings.NewReader("5\n1\ny\n2\n"))
	root.SetArgs([]string{"summary"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(agent.requests) != 3 {
		t.Fatalf("requests=%+v", agent.requests)
	}
	seen := map[string]bool{}
	for _, request := range agent.requests {
		if request.Context.Summary.EnvironmentChanges.Status != "checked" || len(request.Context.Summary.EnvironmentChanges.NewVariables) != 1 || request.Context.Summary.EnvironmentChanges.NewVariables[0] != "CAPTURED_ENV" {
			t.Fatalf("missing authoritative detector result: %+v", request.Context.Summary.EnvironmentChanges)
		}
		if request.Context.OutputLanguage != application.LanguageEnglish {
			t.Fatalf("selected language lost between repositories: %q", request.Context.OutputLanguage)
		}
		if request.Context.TicketOnly {
			if len(request.Context.Repositories) != 2 || request.Repository != nil {
				t.Fatalf("ticket must use all repositories: %+v", request)
			}
			continue
		}
		canonical, _ := filepath.EvalSymlinks(request.WorkingDirectory)
		seen[canonical] = true
		if request.WorkingDirectory == directory || len(request.Repository.Changes) != 1 || !strings.Contains(request.Repository.Changes[0].Diff, "+const Value = 2") || strings.Contains(request.Repository.Changes[0].Diff, "999") {
			t.Fatalf("wrong database context=%+v", request)
		}
		if request.Context.AllTicketWorklogs.Activities[0].Branch != "feature/OOT-1" {
			t.Fatal("stored branch context lost")
		}
	}
	canonicalA, _ := filepath.EvalSymlinks(repoA)
	canonicalB, _ := filepath.EvalSymlinks(repoB)
	if !seen[canonicalA] || !seen[canonicalB] {
		t.Fatalf("repositories=%v", seen)
	}
	if !strings.Contains(progress.String(), "Repository 1/2") || !strings.Contains(progress.String(), "Repository 2/2") || !strings.Contains(progress.String(), "[Custom] → Inspecting supplied diff") {
		t.Fatalf("progress=%q", progress.String())
	}
	if !strings.Contains(out.String(), "Time:\n2h (2 commits)\n\nGenerated for Logs:") || !strings.Contains(out.String(), "Dev By:\ndeveloper@example.com") || strings.Contains(out.String(), "[Custom]") {
		t.Fatalf("final output=%q", out.String())
	}
	if !strings.Contains(out.String(), "New environment variables:\n- CAPTURED_ENV\n") || strings.Count(out.String(), "- CAPTURED_ENV\n") != 1 {
		t.Fatalf("new captured environment missing or duplicated: %q", out.String())
	}
	for _, excluded := range []string{"EXISTING_ENV", "HEAD_ONLY_ENV", "private-fixture-value"} {
		if strings.Contains(out.String(), excluded) {
			t.Fatalf("non-new/uncaptured environment or value leaked into summary: %q", out.String())
		}
	}

	// The exact same captured ranges are checked when AI is declined.
	out.Reset()
	progress.Reset()
	plain := cli.NewRootCommand(nil, "test", nil, nil)
	plain.AddCommand(cli.NewSummaryCommand(factory))
	plain.SetOut(&out)
	plain.SetErr(&progress)
	plain.SetIn(strings.NewReader("5\n1\nn\n"))
	plain.SetArgs([]string{"summary"})
	if err := plain.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "New environment variables:\n- CAPTURED_ENV\n") || strings.Contains(out.String(), "HEAD_ONLY_ENV") || strings.Contains(out.String(), "private-fixture-value") || len(agent.requests) != 3 {
		t.Fatalf("non-AI environment check=%q", out.String())
	}

	// The short command selects this Friday ticket directly from the week;
	// it must not require worklogs on an implicitly selected Monday.
	agent.requests = nil
	out.Reset()
	progress.Reset()
	root = cli.NewRootCommand(nil, "test", nil, nil)
	root.AddCommand(cli.NewGenerateTicketCommand(factory, cli.SummaryAIOptions{Config: config, Git: gitevidence.NewService(), Agents: &integrationAgents{agent: agent}}))
	root.SetOut(&out)
	root.SetErr(&progress)
	root.SetIn(strings.NewReader("1\n2\n"))
	root.SetArgs([]string{"gt"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(agent.requests) != 1 || strings.Contains(progress.String(), "Choose date") || !strings.Contains(progress.String(), "OOT-1") || !strings.Contains(progress.String(), "2h") || !strings.HasPrefix(out.String(), "# Recorded ticket title\n") {
		t.Fatalf("weekly ticket generation: requests=%+v output=%q progress=%q", agent.requests, out.String(), progress.String())
	}
	for _, request := range agent.requests {
		if !request.Context.TicketOnly || !request.Context.Summary.Day.Date.IsZero() || request.Context.OutputLanguage != application.LanguageEnglish || len(request.Context.AllTicketWorklogs.Sessions) != 2 || len(request.Context.Repositories) != 2 {
			t.Fatalf("incorrect ticket-wide context: %+v", request)
		}
		for _, repo := range request.Context.Repositories {
			if repo.Path == directory || len(repo.Changes) != 1 || !strings.Contains(repo.Changes[0].Diff, "+const Value = 2") || strings.Contains(repo.Changes[0].Diff, "999") {
				t.Fatalf("incorrect recorded source evidence: %+v", repo)
			}
		}
	}
}
