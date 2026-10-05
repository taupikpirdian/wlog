package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	application "github.com/taupikpirdian/wlog/application/session"
	domain "github.com/taupikpirdian/wlog/domain/session"
	"github.com/taupikpirdian/wlog/domain/ticket"
)

func sessionDatabase(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "worklog.db")
	db, err := OpenDatabase(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func sessionService(db *sql.DB, now *time.Time) *application.Service {
	pattern, _ := ticket.NewKeyPattern(`[A-Z][A-Z0-9]+-[0-9]+`)
	return application.NewService(NewSQLiteSessionStore(db), pattern, func() time.Time { return *now }, func(context.Context) string { return "" })
}

func TestSessionLifecycleAndRollback(t *testing.T) {
	ctx := context.Background()
	db, _ := sessionDatabase(t)
	at := time.Date(2026, 10, 1, 23, 59, 59, 500000000, time.UTC)
	service := sessionService(db, &at)
	if _, err := db.Exec(`INSERT INTO tickets(ticket_key,title) VALUES ('OOT-1','Master title')`); err != nil {
		t.Fatal(err)
	}
	first, err := service.Start(ctx, "OOT-1", "  First session  ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Title != "First session" || first.Repository != nil {
		t.Fatalf("session: %+v", first)
	}
	at = at.Add(1500 * time.Millisecond)
	if _, err := db.Exec(`CREATE TRIGGER reject_session BEFORE INSERT ON work_sessions WHEN NEW.title='reject' BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Start(ctx, "OOT-2", "reject", &first); err == nil {
		t.Fatal("expected insertion failure")
	}
	active, err := service.Active(ctx)
	if err != nil || active == nil || active.ID != first.ID {
		t.Fatalf("old session not restored: %v %+v", err, active)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM tickets WHERE ticket_key='OOT-2'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("ticket leaked: %d %v", count, err)
	}
	second, err := service.Start(ctx, "OOT-2", "Second session", &first)
	if err != nil {
		t.Fatal(err)
	}
	var ended string
	var duration int64
	if err := db.QueryRow(`SELECT ended_at,duration_seconds FROM work_sessions WHERE id=?`, first.ID).Scan(&ended, &duration); err != nil {
		t.Fatal(err)
	}
	if ended != second.StartedAt.Format(time.RFC3339Nano) || duration != 1 {
		t.Fatalf("pivot/duration: %s %d", ended, duration)
	}
	var title string
	if err := db.QueryRow(`SELECT title FROM tickets WHERE ticket_key='OOT-1'`).Scan(&title); err != nil || title != "Master title" {
		t.Fatalf("master title: %q %v", title, err)
	}
	if _, err := service.Start(ctx, "OOT-3", "Stale confirmation", &first); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale confirmation: %v", err)
	}
	completed, _ := first.Complete(at)
	if err := NewSQLiteSessionStore(db).Complete(ctx, completed); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale stop: %v", err)
	}
	at = at.Add(-time.Second)
	if _, err := service.Stop(ctx); !errors.Is(err, domain.ErrClockBackwards) {
		t.Fatalf("backwards clock: %v", err)
	}
	at = at.Add(2 * time.Second)
	stopped, err := service.Stop(ctx)
	if err != nil || stopped.ID != second.ID || *stopped.DurationSeconds != 1 {
		t.Fatalf("stop: %+v %v", stopped, err)
	}
	if _, err := service.Stop(ctx); !errors.Is(err, domain.ErrNoActive) {
		t.Fatalf("no active: %v", err)
	}
}

func TestConcurrentWritersAcrossConnections(t *testing.T) {
	ctx := context.Background()
	db, path := sessionDatabase(t)
	other, err := OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	stores := []*SQLiteSessionStore{NewSQLiteSessionStore(db), NewSQLiteSessionStore(other)}
	at := time.Now().UTC()
	var wg sync.WaitGroup
	gate := make(chan struct{})
	results := make(chan error, 2)
	for _, store := range stores {
		wg.Add(1)
		go func(store *SQLiteSessionStore) {
			defer wg.Done()
			<-gate
			_, err := store.Start(ctx, domain.Session{TicketKey: "OOT-1", Title: "Concurrent", StartedAt: at, Status: domain.Active}, nil)
			results <- err
		}(store)
	}
	close(gate)
	wg.Wait()
	close(results)
	assertOneWinner(t, results)
	active, err := stores[0].Active(ctx)
	if err != nil || active == nil {
		t.Fatalf("active: %+v %v", active, err)
	}
	completed, _ := active.Complete(at.Add(time.Minute))
	gate = make(chan struct{})
	results = make(chan error, 2)
	for _, store := range stores {
		wg.Add(1)
		go func(store *SQLiteSessionStore) { defer wg.Done(); <-gate; results <- store.Complete(ctx, completed) }(store)
	}
	close(gate)
	wg.Wait()
	close(results)
	assertOneWinner(t, results)
}

func TestValidationAndSameTicketReplacement(t *testing.T) {
	ctx := context.Background()
	db, _ := sessionDatabase(t)
	at := time.Now().UTC()
	service := sessionService(db, &at)
	for _, args := range [][2]string{{"invalid", "Title"}, {"OOT-1 extra", "Title"}, {"OOT-1", "  \t  "}} {
		if _, err := service.Start(ctx, args[0], args[1], nil); err == nil {
			t.Fatalf("accepted invalid input: %v", args)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM tickets`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid input wrote tickets: %d %v", count, err)
	}
	pattern, _ := ticket.NewKeyPattern(`TASK-[0-9]+`)
	service = application.NewService(NewSQLiteSessionStore(db), pattern, func() time.Time { return at }, func(context.Context) string { return "/example/repository" })
	first, err := service.Start(ctx, "TASK-1", "First", nil)
	if err != nil {
		t.Fatal(err)
	}
	var ended, duration sql.NullString
	if err := db.QueryRow(`SELECT ended_at,duration_seconds FROM work_sessions WHERE id=?`, first.ID).Scan(&ended, &duration); err != nil || ended.Valid || duration.Valid {
		t.Fatalf("active session completion fields: %v %v %v", ended, duration, err)
	}
	if first.Repository == nil || *first.Repository != "/example/repository" {
		t.Fatalf("repository: %+v", first)
	}
	at = at.Add(50 * time.Millisecond)
	second, err := service.Start(ctx, "TASK-1", "Same ticket", &first)
	if err != nil || second.ID == first.ID || second.TicketID != first.TicketID {
		t.Fatalf("replacement: %+v %v", second, err)
	}
	var seconds int64
	if err := db.QueryRow(`SELECT duration_seconds FROM work_sessions WHERE id=?`, first.ID).Scan(&seconds); err != nil || seconds != 0 {
		t.Fatalf("subsecond completion: %d %v", seconds, err)
	}
}

func assertOneWinner(t *testing.T, results <-chan error) {
	t.Helper()
	winners, conflicts := 0, 0
	for err := range results {
		if err == nil {
			winners++
		} else if errors.Is(err, domain.ErrConflict) {
			conflicts++
		} else {
			t.Fatalf("unexpected writer error: %v", err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("winners=%d conflicts=%d", winners, conflicts)
	}
}

func TestStartRecordsInitialNoteAtomically(t *testing.T) {
	ctx := context.Background()
	db, _ := sessionDatabase(t)
	at := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	pattern, _ := ticket.NewKeyPattern(`OOT-[0-9]+`)
	service := application.NewService(NewSQLiteSessionStore(db), pattern, func() time.Time { return at }, func(context.Context) string { return "/work/backend" })
	first, err := service.Start(ctx, "OOT-3842", "  meeting be  ", nil)
	if err != nil {
		t.Fatal(err)
	}
	assertInitialSessionNote(t, db, first)
	if _, err := db.Exec(`CREATE TRIGGER reject_initial_note BEFORE INSERT ON work_activities WHEN NEW.description='reject note' BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	at = at.Add(time.Minute)
	if _, err := service.Start(ctx, "OOT-3843", "reject note", &first); err == nil {
		t.Fatal("expected note insertion failure")
	}
	active, err := service.Active(ctx)
	if err != nil || active == nil || active.ID != first.ID {
		t.Fatalf("previous session not restored: %+v %v", active, err)
	}
	for _, table := range []string{"tickets", "work_sessions", "work_activities"} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s leaked rows: %d %v", table, count, err)
		}
	}
	second, err := service.Start(ctx, "OOT-3842", "follow-up", &first)
	if err != nil {
		t.Fatal(err)
	}
	assertInitialSessionNote(t, db, second)
	assertNoteCount(t, db, 2)
}

func assertInitialSessionNote(t *testing.T, db *sql.DB, value domain.Session) {
	t.Helper()
	var ticketID int64
	var description, created string
	var repository sql.NullString
	if err := db.QueryRow(`SELECT ticket_id,description,repository,created_at FROM work_activities WHERE session_id=? AND type='NOTE'`, value.ID).Scan(&ticketID, &description, &repository, &created); err != nil {
		t.Fatal(err)
	}
	if ticketID != value.TicketID || description != value.Title || created != value.StartedAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("initial note: ticket=%d description=%q created=%q session=%+v", ticketID, description, created, value)
	}
	if value.Repository == nil && repository.Valid || value.Repository != nil && (!repository.Valid || repository.String != *value.Repository) {
		t.Fatalf("note repository: %v session=%+v", repository, value)
	}
}
