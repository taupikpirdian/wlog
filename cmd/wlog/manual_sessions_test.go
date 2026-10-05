package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	applicationactivity "github.com/taupikpirdian/wlog/application/activity"
	"github.com/taupikpirdian/wlog/application/bootstrap"
	applicationsession "github.com/taupikpirdian/wlog/application/session"
	"github.com/taupikpirdian/wlog/delivery/cli"
	"github.com/taupikpirdian/wlog/domain/ticket"
	"github.com/taupikpirdian/wlog/infrastructure/storage"
)

func TestManualCompositionLifecycleOnTemporaryDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "manual.db")
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)
	loader := fixtureLoader(func(context.Context) (bootstrap.Config, error) {
		return bootstrap.Config{DatabasePath: path, Ticket: bootstrap.TicketConfig{Pattern: `OOT-[0-9]+`}}, nil
	})
	clock := func() time.Time { return now }
	repository := func(context.Context) string { return "" }
	normal := func(ctx context.Context) (cli.SessionService, func() error, error) {
		db, err := storage.OpenDatabase(ctx, path)
		if err != nil {
			return nil, nil, err
		}
		pattern, _ := ticket.NewKeyPattern(`OOT-[0-9]+`)
		return applicationsession.NewService(storage.NewSQLiteSessionStore(db), pattern, clock, repository), db.Close, nil
	}
	notes := func(ctx context.Context) (cli.NoteService, func() error, error) {
		db, err := storage.OpenDatabase(ctx, path)
		if err != nil {
			return nil, nil, err
		}
		return applicationactivity.NewService(storage.NewSQLiteActivityStore(db), clock), db.Close, nil
	}
	execute := func(args ...string) string {
		t.Helper()
		root := cli.NewRootCommand(dashboardFactory(loader, clock, time.Local), "test", normal, notes)
		cli.AddManualSessionCommands(root, manualSessionFactory(loader, clock, time.Local, repository))
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out.String()
	}
	if got := execute("session", "OOT-1", "--from", "09:00", "--to", "11:00", "--title", "Fix tax"); !strings.Contains(got, "Duration: 2h") {
		t.Fatal(got)
	}
	if got := execute("s", "OOT-2", "Continue", "--since", "11:00"); !strings.Contains(got, "Started: 11:00") {
		t.Fatal(got)
	}
	if got := execute("note", "Investigate"); !strings.Contains(got, "Note added to OOT-2") {
		t.Fatal(got)
	}
	if got := execute(); !strings.Contains(got, "Duration: 1h") || !strings.Contains(got, "Total        3h") {
		t.Fatal(got)
	}
	if got := execute("today"); !strings.Contains(got, "09:00  START   OOT-1") || !strings.Contains(got, "11:00  STOP    OOT-1") || !strings.Contains(got, "11:00  NOTE    OOT-2  Continue") || !strings.Contains(got, "12:00  NOTE    OOT-2  Investigate") {
		t.Fatal(got)
	}
	now = now.Add(time.Hour)
	execute("stop")
	if got := execute(); !strings.Contains(got, "No active session") || !strings.Contains(got, "Total        4h") {
		t.Fatal(got)
	}
	execute("s", "OOT-3842", "meeting be")
	if got := execute("today"); !strings.Contains(got, "13:00  NOTE    OOT-3842  meeting be") {
		t.Fatal(got)
	}
	db, err := storage.OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM work_sessions WHERE status='COMPLETED'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("completed %d %v", count, err)
	}
}

func TestManualFactoryErrors(t *testing.T) {
	failure := errors.New("loader failed")
	for _, tc := range []struct {
		name, pattern, path string
		err                 error
	}{
		{name: "loader", err: failure}, {name: "pattern", pattern: "["}, {name: "database", pattern: `OOT-[0-9]+`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory := manualSessionFactory(fixtureLoader(func(context.Context) (bootstrap.Config, error) {
				return bootstrap.Config{DatabasePath: tc.path, Ticket: bootstrap.TicketConfig{Pattern: tc.pattern}}, tc.err
			}), time.Now, time.Local, func(context.Context) string { return "" })
			_, close, err := factory(context.Background())
			if err == nil || close != nil {
				t.Fatalf("err %v close present %t", err, close != nil)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatal(err)
			}
		})
	}
}
