package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	application "github.com/taupikpirdian/wlog/application/activity"
	"github.com/taupikpirdian/wlog/domain/activity"
	"github.com/taupikpirdian/wlog/domain/session"
	"github.com/taupikpirdian/wlog/infrastructure/storage"
)

func TestNoteCommandsWithSQLite(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "worklog.db")
	db, err := storage.OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	at := time.Now().UTC()
	active, err := storage.NewSQLiteSessionStore(db).Start(ctx, session.Session{TicketKey: "OOT-3668", Title: "Support QA", StartedAt: at, Status: session.Active}, nil)
	if err != nil {
		t.Fatal(err)
	}
	opens, closes := 0, 0
	factory := func(ctx context.Context) (NoteService, func() error, error) {
		opens++
		conn, err := storage.OpenDatabase(ctx, path)
		if err != nil {
			return nil, nil, err
		}
		return application.NewService(storage.NewSQLiteActivityStore(conn), func() time.Time { return at }), func() error { closes++; return conn.Close() }, nil
	}
	for _, args := range [][]string{{"n", "Check Splunk logs"}, {"note", "Found response mismatch"}, {"n", "Support QA retest"}} {
		root := NewRootCommand(nil, "test", nil, factory)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetIn(strings.NewReader(""))
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if out.String() != "✓ Note added to OOT-3668\n" {
			t.Fatalf("output=%q", out.String())
		}
	}
	if opens != 3 || closes != opens {
		t.Fatalf("opens=%d closes=%d", opens, closes)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM work_activities WHERE ticket_id=? AND session_id=? AND type='NOTE'`, active.TicketID, active.ID).Scan(&count); err != nil || count != 4 {
		t.Fatalf("notes=%d error=%v", count, err)
	}
	failedOutput := NewRootCommand(nil, "test", nil, factory)
	failedOutput.SetOut(noteBrokenWriter{})
	failedOutput.SetArgs([]string{"n", "Committed despite output error"})
	if err := failedOutput.Execute(); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("output error=%v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM work_activities`).Scan(&count); err != nil || count != 5 {
		t.Fatalf("output failure lost committed note: count=%d error=%v", count, err)
	}
	completed, _ := active.Complete(at)
	if err := storage.NewSQLiteSessionStore(db).Complete(ctx, completed); err != nil {
		t.Fatal(err)
	}
	root := NewRootCommand(nil, "test", nil, factory)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"n", "No session"})
	err = root.Execute()
	if !errors.Is(err, session.ErrNoActive) || !strings.Contains(err.Error(), "wl s <ticket>") || strings.Contains(out.String(), "✓") {
		t.Fatalf("error=%v output=%q", err, out.String())
	}
}

type noteBrokenWriter struct{}

func (noteBrokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestNoteHelpAndInvalidInputAreLazy(t *testing.T) {
	for _, tc := range []struct {
		args []string
		fail bool
	}{
		{[]string{"note", "--help"}, false},
		{[]string{"help", "note"}, false},
		{[]string{"--help"}, false},
		{[]string{"--version"}, false},
		{[]string{"n"}, true},
		{[]string{"n", "one", "two"}, true},
		{[]string{"n", ""}, true},
		{[]string{"note", " \t\n "}, true},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			root := NewRootCommand(nil, "test", nil, func(context.Context) (NoteService, func() error, error) {
				t.Fatal("opened storage")
				return nil, nil, nil
			})
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs(tc.args)
			err := root.Execute()
			if (err != nil) != tc.fail {
				t.Fatalf("error=%v", err)
			}
			if tc.fail && !strings.Contains(out.String(), "Usage:") {
				t.Fatalf("missing usage: %q", out.String())
			}
		})
	}
}

type failingNotes struct{ err error }

func (s failingNotes) AddNote(context.Context, string) (activity.Note, error) {
	return activity.Note{}, s.err
}

func TestNoteErrorsDoNotPrintSuccess(t *testing.T) {
	for _, cause := range []error{session.ErrConflict, activity.ErrClockBeforeSession, errors.New("database failed")} {
		closed := false
		root := NewRootCommand(nil, "test", nil, func(context.Context) (NoteService, func() error, error) {
			return failingNotes{cause}, func() error { closed = true; return nil }, nil
		})
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{"n", "Valid"})
		if err := root.Execute(); !errors.Is(err, cause) || strings.Contains(out.String(), "✓") || !closed {
			t.Fatalf("error=%v output=%q closed=%v", err, out.String(), closed)
		}
	}
}
