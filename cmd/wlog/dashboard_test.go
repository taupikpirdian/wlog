package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	"github.com/taupikpirdian/wlog/delivery/cli"
	"github.com/taupikpirdian/wlog/infrastructure/storage"
)

type fixtureLoader func(context.Context) (bootstrap.Config, error)

func (f fixtureLoader) LoadOrCreate(ctx context.Context) (bootstrap.Config, error) { return f(ctx) }

func TestDashboardFactoryColdStartupAndWorkLifecycle(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "worklog.db")
	now := time.Date(2026, 10, 2, 10, 0, 0, 0, time.UTC)
	loads := 0
	factory := dashboardFactory(fixtureLoader(func(context.Context) (bootstrap.Config, error) {
		loads++
		return bootstrap.Config{DatabasePath: path}, nil
	}), func() time.Time { return now }, time.UTC)
	execute := func(args ...string) string {
		t.Helper()
		root := cli.NewRootCommand(factory, "test", nil, nil)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	if got := execute(); !strings.Contains(got, "No active session") {
		t.Fatal(got)
	}
	if got := execute("today"); !strings.Contains(got, "No activities today") {
		t.Fatal(got)
	}
	if loads != 2 {
		t.Fatalf("loads=%d", loads)
	}
	db, err := storage.OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`INSERT INTO tickets(id,ticket_key) VALUES(1,'OOT-1')`,
		`INSERT INTO work_sessions(id,ticket_id,title,started_at,status) VALUES(1,1,'Fix tax','2026-10-02T09:00:00Z','ACTIVE')`,
		`INSERT INTO work_activities(ticket_id,session_id,type,description,created_at) VALUES(1,1,'NOTE','Investigate','2026-10-02T09:15:00Z')`,
	} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if got := execute(); !strings.Contains(got, "OOT-1 — Fix tax") || !strings.Contains(got, "Duration: 1h") {
		t.Fatal(got)
	}
	if got := execute("today"); !strings.Contains(got, "09:00  START   OOT-1") || !strings.Contains(got, "09:15  NOTE    OOT-1  Investigate") || strings.Contains(got, "STOP") {
		t.Fatal(got)
	}
	db, err = storage.OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var sessions, activities int
	if err := db.QueryRow(`SELECT count(*) FROM work_sessions`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM work_activities`).Scan(&activities); err != nil {
		t.Fatal(err)
	}
	if sessions != 1 || activities != 1 {
		t.Fatalf("read created work: %d %d", sessions, activities)
	}
}

func TestDashboardFactoryFailures(t *testing.T) {
	failure := errors.New("configuration failure")
	for _, tc := range []struct {
		name, path string
		err        error
	}{
		{name: "config", err: failure}, {name: "database", path: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory := dashboardFactory(fixtureLoader(func(context.Context) (bootstrap.Config, error) {
				return bootstrap.Config{DatabasePath: tc.path}, tc.err
			}), time.Now, time.UTC)
			_, close, err := factory(context.Background())
			if err == nil || close != nil {
				t.Fatalf("factory err %v has close %t", err, close != nil)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatal(err)
			}
		})
	}
}

// Benchmark includes opening/migrating the existing database, snapshot, domain
// projection, rendering and close. Fixture setup is excluded from the timer.
func BenchmarkDailyCommands(b *testing.B) {
	ctx := context.Background()
	path := filepath.Join(b.TempDir(), "worklog.db")
	now := time.Date(2026, 10, 2, 13, 30, 0, 0, time.UTC)
	db, err := storage.OpenDatabase(ctx, path)
	if err != nil {
		b.Fatal(err)
	}
	statements := []string{
		`INSERT INTO tickets(id,ticket_key) VALUES(1,'OOT-1')`,
		`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<10000)
 INSERT INTO work_sessions(ticket_id,title,started_at,ended_at,status,duration_seconds)
 SELECT 1,'Fixture',CASE WHEN x<=100 THEN '2026-10-02T09:00:00Z' ELSE '2026-09-01T09:00:00Z' END,
 CASE WHEN x<=100 THEN '2026-10-02T12:00:00Z' ELSE '2026-09-01T12:00:00Z' END,'COMPLETED',10800 FROM n`,
		`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<50000)
 INSERT INTO work_activities(ticket_id,session_id,type,description,created_at)
 SELECT 1,1,'NOTE','Evidence fixture',CASE WHEN x<=500 THEN '2026-10-02T10:00:00Z' ELSE '2026-09-01T10:00:00Z' END FROM n`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			b.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		b.Fatal(err)
	}
	factory := dashboardFactory(fixtureLoader(func(context.Context) (bootstrap.Config, error) { return bootstrap.Config{DatabasePath: path}, nil }), func() time.Time { return now }, time.UTC)
	for _, command := range []string{"dashboard", "today"} {
		b.Run(command, func(b *testing.B) {
			durations := make([]time.Duration, 0, b.N)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				start := time.Now()
				root := cli.NewRootCommand(factory, "bench", nil, nil)
				root.SetOut(io.Discard)
				if command == "today" {
					root.SetArgs([]string{"today"})
				} else {
					root.SetArgs([]string{})
				}
				if err := root.Execute(); err != nil {
					b.Fatal(err)
				}
				durations = append(durations, time.Since(start))
			}
			b.StopTimer()
			var max time.Duration
			for _, duration := range durations {
				if duration > max {
					max = duration
				}
			}
			b.ReportMetric(float64(max.Microseconds())/1000, "max-ms")
			sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
			b.ReportMetric(float64(durations[len(durations)/2].Microseconds())/1000, "median-ms")
		})
	}
}
