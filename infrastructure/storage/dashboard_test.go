package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	domain "github.com/taupikpirdian/wlog/domain/dashboard"
)

func dailyFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`INSERT INTO tickets(id,ticket_key) VALUES(1,'OOT-1'),(2,'OOT-2')`,
		`INSERT INTO work_sessions(id,ticket_id,title,started_at,status) VALUES(1,1,'Investigation','2026-10-02T09:00:00Z','ACTIVE')`,
		`INSERT INTO work_sessions(id,ticket_id,title,started_at,ended_at,status,duration_seconds) VALUES(2,2,'Legacy','2026-10-01 23:30:00','2026-10-02 00:30:00','COMPLETED',3600)`,
		`INSERT INTO work_activities(id,ticket_id,session_id,type,description,created_at) VALUES(1,1,1,'NOTE','Check tax','2026-10-02T09:15:00.123456789Z')`,
		`INSERT INTO work_activities(id,ticket_id,type,commit_message,commit_hash,created_at) VALUES(2,2,'GIT_COMMIT','fix tax','abcdef0123','2026-10-02 09:30:00')`,
		`INSERT INTO work_activities(id,type,created_at) VALUES(3,'GIT_COMMIT','2026-10-02T09:45:00+00:00')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDailySnapshotNullableLegacyAndReadOnly(t *testing.T) {
	db, _ := sessionDatabase(t)
	dailyFixture(t, db)
	store := NewSQLiteDashboardStore(db)
	snapshot, err := store.ReadSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Sessions) != 2 || len(snapshot.Activities) != 3 {
		t.Fatalf("snapshot: %+v", snapshot)
	}
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	view, err := domain.Build(snapshot, now, time.UTC)
	if err != nil || view.TotalSeconds != 5400 || len(view.Events) != 5 || len(view.Unsessioned) != 1 || len(view.Unassigned) != 1 {
		t.Fatalf("view: %+v %v", view, err)
	}
	if snapshot.Activities[0].SessionID == nil || snapshot.Activities[1].Hash != "abcdef0123" || snapshot.Activities[2].TicketKey != "" {
		t.Fatalf("activity mapping: %+v", snapshot.Activities)
	}
	for _, query := range []string{`SELECT count(*) FROM work_sessions WHERE status='ACTIVE'`, `SELECT count(*) FROM work_activities WHERE session_id=1`, `SELECT count(*) FROM tickets WHERE ticket_key='OOT-1'`} {
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil || count != 1 {
			t.Fatalf("data changed: %d %v", count, err)
		}
	}
}

func TestDailySnapshotFailures(t *testing.T) {
	for _, tc := range []struct{ name, statement, want string }{
		{"start parse", `UPDATE work_sessions SET started_at='broken' WHERE id=2`, "started_at"},
		{"end parse", `UPDATE work_sessions SET ended_at='broken' WHERE id=2`, "ended_at"},
		{"activity parse", `UPDATE work_activities SET created_at='broken' WHERE id=3`, "created_at"},
		{"sessions query", `DROP TABLE work_sessions`, "work_sessions"},
		{"activities query", `DROP TABLE work_activities`, "work_activities"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := sessionDatabase(t)
			dailyFixture(t, db)
			if _, err := db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(tc.statement); err != nil {
				t.Fatal(err)
			}
			snapshot, err := NewSQLiteDashboardStore(db).ReadSnapshot(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(snapshot.Sessions) != 0 || len(snapshot.Activities) != 0 {
				t.Fatalf("partial/error: %+v %v", snapshot, err)
			}
		})
	}
	db, _ := sessionDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewSQLiteDashboardStore(db).ReadSnapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	db.Close()
	if _, err := NewSQLiteDashboardStore(db).ReadSnapshot(context.Background()); err == nil {
		t.Fatal("closed database accepted")
	}
}

// The barrier delays the second query while another connection commits. The
// already-established transaction must still read the original activity set.
type barrierSnapshot struct {
	tx      *sql.Tx
	between func()
	calls   int
}

func (b *barrierSnapshot) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	b.calls++
	if b.calls == 2 {
		b.between()
	}
	return b.tx.QueryContext(ctx, query, args...)
}

func TestDailySnapshotRemainsConsistentDuringStopAndNote(t *testing.T) {
	ctx := context.Background()
	db, path := sessionDatabase(t)
	dailyFixture(t, db)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		t.Fatal(err)
	}
	writer, err := OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	q := &barrierSnapshot{tx: tx, between: func() {
		other, err := writer.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer other.Rollback()
		if _, err := other.Exec(`UPDATE work_sessions SET status='COMPLETED',ended_at='2026-10-02T10:00:00Z',duration_seconds=3600 WHERE id=1`); err != nil {
			t.Fatal(err)
		}
		if _, err := other.Exec(`INSERT INTO work_activities(ticket_id,session_id,type,description,created_at) VALUES(1,1,'NOTE','concurrent','2026-10-02T09:59:00Z')`); err != nil {
			t.Fatal(err)
		}
		if err := other.Commit(); err != nil {
			t.Fatal(err)
		}
	}}
	old, err := readDailySnapshot(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if old.Sessions[0].Status != "ACTIVE" || len(old.Activities) != 3 {
		t.Fatalf("mixed snapshot: %+v", old)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	next, err := NewSQLiteDashboardStore(db).ReadSnapshot(ctx)
	if err != nil || next.Sessions[0].Status != "COMPLETED" || len(next.Activities) != 4 {
		t.Fatalf("next snapshot: %+v %v", next, err)
	}
}
