package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	application "github.com/taupikpirdian/wlog/application/activity"
	"github.com/taupikpirdian/wlog/domain/activity"
	"github.com/taupikpirdian/wlog/domain/session"
	"github.com/taupikpirdian/wlog/domain/ticket"
)

type staticCommitReader struct{ value activity.Commit }

func (r staticCommitReader) Read(context.Context, application.CaptureOptions) (activity.Commit, error) {
	return r.value, nil
}

func commitService(db *sql.DB, hash, message, branch string) *application.CaptureService {
	pattern, _ := ticket.NewKeyPattern(`[A-Z][A-Z0-9]+-[0-9]+`)
	value := activity.Commit{Repository: "/fixture/repo", Hash: hash, Message: &message, Branch: &branch, Metadata: activity.CommitMetadata{Status: map[string]string{"timestamp": "unavailable"}}}
	return application.NewCaptureService(staticCommitReader{value}, NewSQLiteActivityStore(db), pattern, application.CaptureOptions{}, func() time.Time { return time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC) })
}

func TestCaptureMatchingMismatchFallbackAndUnassigned(t *testing.T) {
	ctx := context.Background()
	db, _ := sessionDatabase(t)
	at := time.Now().UTC()
	sessions := sessionService(db, &at)
	active, err := sessions.Start(ctx, "OOT-1", "First", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE tickets SET title='Master title' WHERE id=?`, active.TicketID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		hash, message, branch, key, source string
		attached                           bool
	}{
		{"hash1", "Subject\nOOT-1 first key\nOOT-2 second", "OOT-3", "OOT-1", "MESSAGE", true},
		{"hash2", "OOT-2 mismatch", "OOT-1", "OOT-2", "MESSAGE", false},
		{"hash3", "No key", "OOT-2", "OOT-1", "ACTIVE_SESSION", true},
	} {
		result, err := commitService(db, tc.hash, tc.message, tc.branch).Capture(ctx)
		if err != nil || result.Attribution.TicketKey != tc.key || result.Attribution.Source != tc.source || (result.Attribution.SessionID != nil) != tc.attached {
			t.Fatalf("result=%+v error=%v", result, err)
		}
	}
	if _, err := sessions.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ hash, message, branch, key, source string }{
		{"hash4", "OOT-2 without session", "OOT-3", "OOT-2", "MESSAGE"},
		{"hash5", "No key", "feature/OOT-3", "OOT-3", "BRANCH"},
		{"hash6", "No key", "main", "", "UNASSIGNED"},
	} {
		result, err := commitService(db, tc.hash, tc.message, tc.branch).Capture(ctx)
		if err != nil || result.Attribution.TicketKey != tc.key || result.Attribution.Source != tc.source || result.Attribution.SessionID != nil {
			t.Fatalf("result=%+v error=%v", result, err)
		}
		var ticketID, sessionID sql.NullInt64
		var metadata string
		var stats, files sql.NullString
		if err := db.QueryRow(`SELECT ticket_id,session_id,metadata,insertions,changed_files FROM work_activities WHERE id=?`, result.ID).Scan(&ticketID, &sessionID, &metadata, &stats, &files); err != nil {
			t.Fatal(err)
		}
		if ticketID.Valid != (tc.key != "") || sessionID.Valid || stats.Valid || files.Valid {
			t.Fatalf("nullability: ticket=%v session=%v stats=%v files=%v", ticketID, sessionID, stats, files)
		}
		var parsed activity.CommitMetadata
		if err := json.Unmarshal([]byte(metadata), &parsed); err != nil || parsed.TimeSource != "capture_fallback" || parsed.TicketSource != tc.source {
			t.Fatalf("metadata=%s error=%v", metadata, err)
		}
	}
	var title string
	if err := db.QueryRow(`SELECT title FROM tickets WHERE id=?`, active.TicketID).Scan(&title); err != nil || title != "Master title" {
		t.Fatalf("master title=%q error=%v", title, err)
	}
	assertNoteCount(t, db, 7)
}

func TestCaptureDuplicateRollbackAndIdentity(t *testing.T) {
	ctx := context.Background()
	db, _ := sessionDatabase(t)
	first, err := commitService(db, "hash", "OOT-1 original", "main").Capture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := commitService(db, "hash", "OOT-999 changed", "main").Capture(ctx)
	if err != nil || !duplicate.AlreadyCaptured || duplicate.ID != first.ID {
		t.Fatalf("duplicate=%+v error=%v", duplicate, err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM tickets WHERE ticket_key='OOT-999'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("duplicate created ticket: %d %v", count, err)
	}
	if _, err := db.Exec(`CREATE TRIGGER reject_commit BEFORE INSERT ON work_activities WHEN NEW.type='GIT_COMMIT' BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := commitService(db, "newhash", "OOT-2 reject", "main").Capture(ctx); err == nil {
		t.Fatal("expected insert failure")
	}
	if err := db.QueryRow(`SELECT count(*) FROM tickets WHERE ticket_key='OOT-2'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback leaked ticket: %d %v", count, err)
	}
	if _, err := db.Exec(`DROP TRIGGER reject_commit`); err != nil {
		t.Fatal(err)
	}
	value := activity.Commit{Repository: "/different/checkout", Hash: "hash", CreatedAt: time.Now().UTC()}
	result, err := NewSQLiteActivityStore(db).Capture(ctx, value, func(*session.Session) activity.Attribution { return activity.ResolveCommit("", "", nil) })
	if err != nil || result.AlreadyCaptured {
		t.Fatalf("different repository=%+v error=%v", result, err)
	}
	assertNoteCount(t, db, 2)
}

func TestConcurrentCaptureAndActiveAtWrite(t *testing.T) {
	ctx := context.Background()
	db, path := sessionDatabase(t)
	other, err := OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	gate := make(chan struct{})
	results := make(chan activity.CaptureResult, 2)
	failures := make(chan error, 2)
	var wg sync.WaitGroup
	for _, conn := range []*sql.DB{db, other} {
		wg.Add(1)
		go func(conn *sql.DB) {
			defer wg.Done()
			<-gate
			result, err := commitService(conn, "samehash", "OOT-1 commit", "main").Capture(ctx)
			results <- result
			failures <- err
		}(conn)
	}
	close(gate)
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	duplicates := 0
	for result := range results {
		if result.AlreadyCaptured {
			duplicates++
		}
	}
	if duplicates != 1 {
		t.Fatalf("duplicates=%d", duplicates)
	}
	assertNoteCount(t, db, 1)
	// Capture reads Git before persistence. Change active session during that read.
	at := time.Now().UTC()
	sessions := sessionService(other, &at)
	if _, err := sessions.Start(ctx, "OOT-2", "Initial", nil); err != nil {
		t.Fatal(err)
	}
	pattern, _ := ticket.NewKeyPattern(`[A-Z][A-Z0-9]+-[0-9]+`)
	reader := commitReaderFunc(func(context.Context, application.CaptureOptions) (activity.Commit, error) {
		if _, err := sessions.Stop(ctx); err != nil {
			return activity.Commit{}, err
		}
		if _, err := sessions.Start(ctx, "OOT-3", "Current", nil); err != nil {
			return activity.Commit{}, err
		}
		message := "no key"
		return activity.Commit{Repository: "/fixture/repo", Hash: "changedactive", Message: &message}, nil
	})
	service := application.NewCaptureService(reader, NewSQLiteActivityStore(db), pattern, application.CaptureOptions{}, time.Now)
	result, err := service.Capture(ctx)
	if err != nil || result.Attribution.TicketKey != "OOT-3" || result.Attribution.SessionID == nil {
		t.Fatalf("active at write=%+v error=%v", result, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := commitService(db, "cancelled", "OOT-1", "main").Capture(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
}

type commitReaderFunc func(context.Context, application.CaptureOptions) (activity.Commit, error)

func (f commitReaderFunc) Read(ctx context.Context, options application.CaptureOptions) (activity.Commit, error) {
	return f(ctx, options)
}
