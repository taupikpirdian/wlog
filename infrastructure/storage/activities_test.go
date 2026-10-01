package storage

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	application "github.com/taupikpirdian/wlog/application/activity"
	"github.com/taupikpirdian/wlog/domain/activity"
	"github.com/taupikpirdian/wlog/domain/session"
)

func TestNotesPersistContentAndPreserveSession(t *testing.T) {
	ctx := context.Background()
	db, _ := sessionDatabase(t)
	at := time.Date(2026, 10, 1, 23, 59, 59, 0, time.UTC)
	sessions := sessionService(db, &at)
	first, err := sessions.Start(ctx, "OOT-3668", "Support QA", nil)
	if err != nil {
		t.Fatal(err)
	}
	store := NewSQLiteActivityStore(db)
	service := application.NewService(store, func() time.Time { return at })
	at = at.Add(2 * time.Second)
	text := "  QA — OOT-9999\n'quoted'; DROP TABLE tickets;  "
	for i := 0; i < 2; i++ {
		got, err := service.AddNote(ctx, text)
		if err != nil || got.ID == 0 || got.TicketID != first.TicketID || got.SessionID != first.ID || got.Repository != nil {
			t.Fatalf("note=%+v error=%v", got, err)
		}
		var description, timestamp, kind string
		var repository, branch, hash, message, files, metadata sql.NullString
		var insertions, deletions int
		if err := db.QueryRow(`SELECT type,description,created_at,repository,branch,commit_hash,commit_message,changed_files,metadata,insertions,deletions FROM work_activities WHERE id=?`, got.ID).Scan(&kind, &description, &timestamp, &repository, &branch, &hash, &message, &files, &metadata, &insertions, &deletions); err != nil {
			t.Fatal(err)
		}
		if kind != "NOTE" || description != "QA — OOT-9999\n'quoted'; DROP TABLE tickets;" || timestamp != at.Format(time.RFC3339Nano) {
			t.Fatalf("stored=%q %q %q", kind, description, timestamp)
		}
		if repository.Valid || branch.Valid || hash.Valid || message.Valid || files.Valid || metadata.Valid || insertions != 0 || deletions != 0 {
			t.Fatal("unexpected Git metadata")
		}
	}
	assertNoteCount(t, db, 2)
	var status, title, start string
	var end sql.NullString
	var duration sql.NullInt64
	if err := db.QueryRow(`SELECT status,title,started_at,ended_at,duration_seconds FROM work_sessions WHERE id=?`, first.ID).Scan(&status, &title, &start, &end, &duration); err != nil {
		t.Fatal(err)
	}
	if status != session.Active || title != first.Title || start != first.StartedAt.Format(time.RFC3339Nano) || end.Valid || duration.Valid {
		t.Fatal("note changed session")
	}
	if _, err := sessions.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddNote(ctx, "After stop"); !errors.Is(err, session.ErrNoActive) {
		t.Fatalf("no-active error=%v", err)
	}
	assertNoteCount(t, db, 2)
}

func TestNoteSnapshotConflictsAndRepository(t *testing.T) {
	ctx := context.Background()
	db, path := sessionDatabase(t)
	other, err := OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	at := time.Now().UTC()
	sessions := sessionService(other, &at)
	first, err := sessions.Start(ctx, "OOT-1", "First", nil)
	if err != nil {
		t.Fatal(err)
	}
	store := NewSQLiteActivityStore(db)
	stale, err := store.Active(ctx)
	if err != nil {
		t.Fatal(err)
	}
	value, _ := activity.NewNote("Stale", *stale, at)
	if _, err := sessions.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddNote(ctx, value, *stale); !errors.Is(err, session.ErrConflict) {
		t.Fatalf("stopped snapshot=%v", err)
	}
	second, err := sessions.Start(ctx, "OOT-2", "Second", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddNote(ctx, value, first); !errors.Is(err, session.ErrConflict) {
		t.Fatalf("switched snapshot=%v", err)
	}
	assertNoteCount(t, db, 0)
	if _, err := db.Exec(`UPDATE work_sessions SET repository='/session/repo' WHERE id=?`, second.ID); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store, func() time.Time { return at })
	got, err := service.AddNote(ctx, "Current")
	if err != nil || got.SessionID != second.ID || got.TicketID != second.TicketID || got.Repository == nil || *got.Repository != "/session/repo" {
		t.Fatalf("current note=%+v error=%v", got, err)
	}
}

func TestNoteValidationRollbackAndCancellation(t *testing.T) {
	ctx := context.Background()
	db, _ := sessionDatabase(t)
	at := time.Now().UTC()
	sessions := sessionService(db, &at)
	service := application.NewService(NewSQLiteActivityStore(db), func() time.Time { return at })
	if _, err := service.AddNote(ctx, "Without session"); !errors.Is(err, session.ErrNoActive) {
		t.Fatalf("no-active=%v", err)
	}
	if _, err := service.AddNote(ctx, " \t "); !errors.Is(err, activity.ErrEmptyDescription) {
		t.Fatalf("empty=%v", err)
	}
	active, err := sessions.Start(ctx, "OOT-1", "First", nil)
	if err != nil {
		t.Fatal(err)
	}
	at = at.Add(-time.Second)
	if _, err := service.AddNote(ctx, "Earlier"); !errors.Is(err, activity.ErrClockBeforeSession) {
		t.Fatalf("clock=%v", err)
	}
	at = active.StartedAt
	if _, err := db.Exec(`CREATE TRIGGER reject_note BEFORE INSERT ON work_activities WHEN NEW.type='NOTE' BEGIN SELECT RAISE(ABORT,'injected note failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AddNote(ctx, "Reject"); err == nil {
		t.Fatal("expected insert failure")
	}
	assertNoteCount(t, db, 0)
	if _, err := db.Exec(`DROP TRIGGER reject_note`); err != nil {
		t.Fatal(err)
	}
	// Cancel after INSERT but before COMMIT to exercise transaction cleanup.
	cancelled, cancel := context.WithCancel(ctx)
	err = writeTransaction(cancelled, db, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(cancelled, `INSERT INTO work_activities(ticket_id,session_id,type,description) VALUES (?,?,'NOTE','cancelled')`, active.TicketID, active.ID)
		cancel()
		return err
	})
	if err == nil {
		t.Fatal("expected cancelled commit")
	}
	assertNoteCount(t, db, 0)
	if _, err := service.AddNote(ctx, "At start"); err != nil {
		t.Fatalf("database unusable after rollback: %v", err)
	}
	found, err := sessions.Active(ctx)
	if err != nil || found == nil || found.ID != active.ID {
		t.Fatalf("session changed=%+v %v", found, err)
	}
}

func TestConcurrentNotes(t *testing.T) {
	ctx := context.Background()
	db, path := sessionDatabase(t)
	other, err := OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	at := time.Now().UTC()
	if _, err := sessionService(db, &at).Start(ctx, "OOT-1", "First", nil); err != nil {
		t.Fatal(err)
	}
	gate := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, conn := range []*sql.DB{db, other} {
		wg.Add(1)
		go func(conn *sql.DB) {
			defer wg.Done()
			<-gate
			_, err := application.NewService(NewSQLiteActivityStore(conn), func() time.Time { return at }).AddNote(ctx, "Same note")
			results <- err
		}(conn)
	}
	close(gate)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertNoteCount(t, db, 2)
}

func assertNoteCount(t *testing.T, db *sql.DB, expected int) {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM work_activities`).Scan(&count); err != nil || count != expected {
		t.Fatalf("activity count=%d want=%d error=%v", count, expected, err)
	}
}
