package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	"github.com/taupikpirdian/wlog/delivery/cli"
	"github.com/taupikpirdian/wlog/infrastructure/storage"
)

func TestSummaryCommandWithDatabaseAndGitEmail(t *testing.T) {
	directory := t.TempDir()
	global := filepath.Join(directory, "global.gitconfig")
	if err := os.WriteFile(global, []byte("[user]\n email = integration@example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "0")
	// Prevent the workspace repository's local email from overriding this fixture.
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(directory, "worklog.db")
	db, err := storage.OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		`INSERT INTO tickets(id,ticket_key) VALUES(1,'OOT-1')`,
		`INSERT INTO work_sessions(ticket_id,title,started_at,ended_at,status,duration_seconds) VALUES(1,'Fix bug','2026-10-02T09:00:00Z','2026-10-02T10:00:00Z','COMPLETED',3600),(1,'Review code','2026-10-02T11:00:00Z','2026-10-02T12:00:00Z','COMPLETED',3600)`,
		`INSERT INTO work_activities(ticket_id,type,description,created_at) VALUES(1,'NOTE','Fix bug','2026-10-02T09:30:00Z')`,
		`INSERT INTO work_activities(ticket_id,type,commit_message,created_at) VALUES(1,'GIT_COMMIT','Add regression test','2026-10-02T09:45:00Z')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	factory := summaryFactory(fixtureLoader(func(context.Context) (bootstrap.Config, error) { return bootstrap.Config{DatabasePath: path}, nil }), func() time.Time { return now }, time.UTC)
	root := cli.NewRootCommand(nil, "test", nil, nil)
	root.AddCommand(cli.NewSummaryCommand(factory))
	var out, prompts bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&prompts)
	root.SetIn(strings.NewReader("5\n1\nn\n"))
	root.SetArgs([]string{"summary"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	want := "Time:\n2h (1 commit)\n\nGenerated for Logs:\nDetail:\n- Fix bug\n- Add regression test\n- Review code\n\nResult:\n-\n\nDev By:\nintegration@example.com\n"
	if out.String() != want+"\n### Environment Changes\n\nEnvironment variable check could not be completed.\n" {
		t.Fatalf("stdout=%q want=%q", out.String(), want)
	}
	if !strings.Contains(prompts.String(), "Fri, 02 Oct 2026    2h") {
		t.Fatalf("prompts=%q", prompts.String())
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM work_sessions`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("summary mutated sessions: count=%d err=%v", count, err)
	}
}
