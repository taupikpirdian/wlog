package storage

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	application "github.com/taupikpirdian/wlog/application/session"
	domain "github.com/taupikpirdian/wlog/domain/session"
	"github.com/taupikpirdian/wlog/domain/ticket"
)

func manualService(db *sql.DB, now *time.Time) application.ManualUsecase {
	pattern, _ := ticket.NewKeyPattern(`OOT-[0-9]+`)
	return application.NewManualService(NewSQLiteSessionStore(db), pattern, func() time.Time { return *now }, time.UTC, func(context.Context) string { return "" })
}

func TestManualPersistenceProjectionAndEvidencePreservation(t *testing.T) {
	ctx := context.Background()
	db, _ := sessionDatabase(t)
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	svc := manualService(db, &now)
	for _, statement := range []string{
		`INSERT INTO tickets(id,ticket_key,title) VALUES(1,'OOT-1','Master title')`,
		`INSERT INTO work_activities(ticket_id,type,commit_message,created_at) VALUES(1,'GIT_COMMIT','Existing evidence','2026-10-02 09:30:00')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	value, err := svc.CreateCompleted(ctx, "OOT-1", " Fix tax ", "09:00", "11:00")
	if err != nil || value.ID == 0 || value.TicketID != 1 || value.Title != "Fix tax" || *value.DurationSeconds != 7200 {
		t.Fatalf("manual: %+v %v", value, err)
	}
	var title, created, updated string
	var sid sql.NullInt64
	if err := db.QueryRow(`SELECT title FROM tickets WHERE id=1`).Scan(&title); err != nil || title != "Master title" {
		t.Fatalf("master changed %s %v", title, err)
	}
	if err := db.QueryRow(`SELECT created_at,updated_at FROM work_sessions WHERE id=?`, value.ID).Scan(&created, &updated); err != nil || created != now.Format(time.RFC3339Nano) || updated != created {
		t.Fatalf("audit: %s %s %v", created, updated, err)
	}
	if err := db.QueryRow(`SELECT session_id FROM work_activities`).Scan(&sid); err != nil || sid.Valid {
		t.Fatalf("evidence moved: %v %v", sid, err)
	}
	active, err := svc.StartSince(ctx, "OOT-2", "Continued", "11:00")
	if err != nil || active.EndedAt != nil || active.DurationSeconds != nil {
		t.Fatalf("adjacent since: %+v %v", active, err)
	}
	assertInitialSessionNote(t, db, active)
	_, err = svc.CreateCompleted(ctx, "OOT-3", "Earlier", "08:00", "09:00")
	if err != nil {
		t.Fatal(err)
	}
	still, err := NewSQLiteSessionStore(db).Active(ctx)
	if err != nil || still.ID != active.ID || still.Status != domain.Active {
		t.Fatalf("active changed: %+v %v", still, err)
	}
	snapshot, err := NewSQLiteDashboardStore(db).ReadSnapshot(ctx)
	if err != nil || len(snapshot.Sessions) != 3 || len(snapshot.Activities) != 2 {
		t.Fatalf("projection sources: %+v %v", snapshot, err)
	}
}

func TestManualRollbackConflictsAndLegacyTimes(t *testing.T) {
	ctx := context.Background()
	db, _ := sessionDatabase(t)
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	svc := manualService(db, &now)
	for _, sql := range []string{
		`INSERT INTO tickets(id,ticket_key) VALUES(1,'OOT-1')`,
		`INSERT INTO work_sessions(ticket_id,title,started_at,ended_at,status,duration_seconds) VALUES(1,'Legacy','2026-10-02 09:00:00','2026-10-02 10:00:00','COMPLETED',3600)`,
		`CREATE TRIGGER reject_manual BEFORE INSERT ON work_sessions WHEN NEW.title='reject' BEGIN SELECT RAISE(ABORT,'fixture reject'); END`,
	} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		key, title, from, to string
		want                 error
	}{
		{"OOT-2", "overlap", "09:30", "11:00", domain.ErrOverlap},
		{"OOT-3", "reject", "10:00", "11:00", nil},
	} {
		_, err := svc.CreateCompleted(ctx, tc.key, tc.title, tc.from, tc.to)
		if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
			t.Fatalf("expected rejection: %v", err)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM tickets`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("ticket leaked: %d %v", count, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM work_sessions`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("session leaked: %d %v", count, err)
	}
	active, err := svc.StartSince(ctx, "OOT-2", "active", "10:00")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StartSince(ctx, "OOT-3", "other active", "11:00"); !errors.Is(err, domain.ErrActiveManual) {
		t.Fatal(err)
	}
	if _, err := svc.CreateCompleted(ctx, "OOT-3", "active overlap", "11:00", "12:00"); !errors.Is(err, domain.ErrOverlap) {
		t.Fatal(err)
	}
	completed, err := active.Complete(now)
	if err != nil {
		t.Fatal(err)
	}
	if err := NewSQLiteSessionStore(db).Complete(ctx, completed); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := svc.CreateCompleted(ctx, "OOT-4", "cancel", "08:00", "09:00"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE work_sessions SET started_at='invalid' WHERE title='Legacy'`); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateCompleted(context.Background(), "OOT-4", "parse", "08:00", "09:00"); err == nil {
		t.Fatal("corrupt timestamp ignored")
	}
}

func TestConcurrentManualWriters(t *testing.T) {
	for _, modes := range [][2]bool{{false, false}, {true, true}, {false, true}} {
		t.Run(map[bool]string{true: "active", false: "completed"}[modes[0]]+"-"+map[bool]string{true: "active", false: "completed"}[modes[1]], func(t *testing.T) {
			ctx := context.Background()
			db, path := sessionDatabase(t)
			other, err := OpenDatabase(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
			services := []application.ManualUsecase{manualService(db, &now), manualService(other, &now)}
			var wg sync.WaitGroup
			gate := make(chan struct{})
			results := make(chan error, 2)
			for i, svc := range services {
				wg.Add(1)
				go func(i int, svc application.ManualUsecase) {
					defer wg.Done()
					<-gate
					var err error
					if modes[i] {
						_, err = svc.StartSince(ctx, "OOT-1", "work", "09:00")
					} else {
						_, err = svc.CreateCompleted(ctx, "OOT-1", "work", "09:00", "11:00")
					}
					results <- err
				}(i, svc)
			}
			close(gate)
			wg.Wait()
			close(results)
			success, conflicts := 0, 0
			for err := range results {
				if err == nil {
					success++
				} else if errors.Is(err, domain.ErrOverlap) || errors.Is(err, domain.ErrActiveManual) {
					conflicts++
				} else {
					t.Fatal(err)
				}
			}
			if success != 1 || conflicts != 1 {
				t.Fatalf("winners %d conflicts %d", success, conflicts)
			}
			var count int
			if err := db.QueryRow(`SELECT count(*) FROM work_sessions`).Scan(&count); err != nil || count != 1 {
				t.Fatalf("count %d %v", count, err)
			}
		})
	}
}

func TestBackdatedAndOrdinaryStartRaceHasOneActiveSession(t *testing.T) {
	ctx := context.Background()
	db, path := sessionDatabase(t)
	other, err := OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	gate := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-gate
		_, err := manualService(db, &now).StartSince(ctx, "OOT-1", "Backdated", "09:00")
		results <- err
	}()
	go func() {
		<-gate
		_, err := NewSQLiteSessionStore(other).Start(ctx, domain.Session{TicketKey: "OOT-2", Title: "Ordinary", StartedAt: now, Status: domain.Active}, nil)
		results <- err
	}()
	close(gate)
	winners, conflicts := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			winners++
		} else if errors.Is(err, domain.ErrActiveManual) || errors.Is(err, domain.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if winners != 1 || conflicts != 1 {
		t.Fatalf("winners=%d conflicts=%d", winners, conflicts)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM work_sessions WHERE status='ACTIVE'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("active count=%d err=%v", count, err)
	}
}

func TestBackdatedStartRollsBackWhenInitialNoteFails(t *testing.T) {
	ctx := context.Background()
	db, _ := sessionDatabase(t)
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`CREATE TRIGGER reject_initial_note BEFORE INSERT ON work_activities BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := manualService(db, &now).StartSince(ctx, "OOT-3842", "meeting be", "09:00"); err == nil {
		t.Fatal("expected note insertion failure")
	}
	for _, table := range []string{"tickets", "work_sessions", "work_activities"} {
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s leaked rows: %d %v", table, count, err)
		}
	}
}
