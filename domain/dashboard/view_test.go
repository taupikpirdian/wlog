package dashboard_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	domain "github.com/taupikpirdian/wlog/domain/dashboard"
	"github.com/taupikpirdian/wlog/domain/session"
)

func instant(value string) time.Time {
	at, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		panic(err)
	}
	return at
}
func finished(id int64, key, start, end string) session.Session {
	at := instant(end)
	return session.Session{ID: id, TicketKey: key, StartedAt: instant(start), EndedAt: &at, Status: session.Completed}
}

func TestDailyViewClipsAndAggregatesWithoutChangingSources(t *testing.T) {
	loc := time.FixedZone("Jakarta", 7*3600)
	now := instant("2026-10-02T10:56:00+07:00")
	active := session.Session{ID: 4, TicketKey: "OOT-3751", Title: "Fix tax calculation", StartedAt: instant("2026-10-02T09:14:00+07:00"), Status: session.Active}
	sid := int64(4)
	source := domain.Snapshot{Sessions: []session.Session{
		finished(1, "OOT-3751", "2026-10-02T08:00:00+07:00", "2026-10-02T08:48:00+07:00"),
		finished(2, "OOT-3747", "2026-10-02T06:00:00+07:00", "2026-10-02T07:15:00+07:00"),
		finished(3, "OOT-3668", "2026-10-02T07:15:00+07:00", "2026-10-02T08:00:00+07:00"), active,
	}, Activities: []domain.Activity{
		{ID: 1, TicketKey: "OOT-3751", SessionID: &sid, Type: "NOTE", Text: "investigate", At: now},
		{ID: 2, TicketKey: "OOT-9", Type: "GIT_COMMIT", Text: "fix", At: now},
		{ID: 3, Type: "GIT_COMMIT", At: now},
		{ID: 4, Type: "GIT_COMMIT", At: instant("2026-10-01T23:59:59+07:00")},
	}}
	view, err := domain.Build(source, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.TicketSummary{{Key: "OOT-3668", Seconds: 2700}, {Key: "OOT-3747", Seconds: 4500}, {Key: "OOT-3751", Title: "Fix tax calculation", Seconds: 9000}, {Key: "OOT-9", Title: "fix", Seconds: 0}}
	if !reflect.DeepEqual(view.Tickets, want) || view.TotalSeconds != 16200 || view.Active == nil || view.Active.ElapsedSeconds != 6120 {
		t.Fatalf("view: %+v", view)
	}
	if len(view.Events) != 10 || len(view.Unsessioned) != 1 || len(view.Unassigned) != 1 {
		t.Fatalf("event counts: %+v", view)
	}
	if view.Unassigned[0].Text != "(no commit message)" || view.Unassigned[0].Kind != "COMMIT" {
		t.Fatalf("unassigned: %+v", view.Unassigned)
	}
	view.Active.Session.Title = "changed"
	if source.Sessions[3].Title != active.Title || source.Sessions[3].EndedAt != nil {
		t.Fatal("source mutated")
	}
}

func TestCalendarAndDurationBoundaries(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		date  string
		hours int
	}{{"2026-03-08T12:00:00-04:00", 23}, {"2026-11-01T12:00:00-05:00", 25}} {
		view, err := domain.Build(domain.Snapshot{}, instant(tc.date), loc)
		if err != nil || view.End.Sub(view.Start) != time.Duration(tc.hours)*time.Hour {
			t.Fatalf("calendar: %+v %v", view, err)
		}
	}
	loc = time.FixedZone("Jakarta", 7*3600)
	now := instant("2026-10-02T01:00:00+07:00")
	start := instant("2026-10-01T23:00:00+07:00")
	view, err := domain.Build(domain.Snapshot{Sessions: []session.Session{{ID: 1, TicketKey: "OOT-1", StartedAt: start, Status: session.Active}}}, now, loc)
	if err != nil || view.TotalSeconds != 3600 || view.Active.ElapsedSeconds != 7200 || len(view.Events) != 0 {
		t.Fatalf("overnight: %+v %v", view, err)
	}
	source := domain.Snapshot{Sessions: []session.Session{
		finished(1, "OOT-1", "2026-10-01T23:30:00+07:00", "2026-10-02T00:30:00+07:00"),
		finished(2, "OOT-1", "2026-10-01T10:00:00+07:00", "2026-10-01T11:00:00+07:00"),
		finished(3, "OOT-2", "2026-10-02T02:00:00+07:00", "2026-10-02T03:00:00+07:00"),
		finished(4, "OOT-3", "2026-10-02T00:00:00+07:00", "2026-10-02T00:00:00+07:00"),
	}, Activities: []domain.Activity{
		{ID: 1, Type: "NOTE", At: instant("2026-10-02T00:00:00+07:00")},
		{ID: 2, Type: "NOTE", At: instant("2026-10-03T00:00:00+07:00")},
	}}
	view, err = domain.Build(source, now, loc)
	if err != nil || view.TotalSeconds != 1800 || len(view.Events) != 6 || len(view.Tickets) != 3 {
		t.Fatalf("boundaries: %+v %v", view, err)
	}
	// Completed future interval spanning now is clipped at now, not its persisted end.
	view, err = domain.Build(domain.Snapshot{Sessions: []session.Session{finished(1, "OOT-1", "2026-10-02T00:30:00+07:00", "2026-10-03T01:00:00+07:00")}}, now, loc)
	if err != nil || view.TotalSeconds != 1800 {
		t.Fatalf("future end: %+v %v", view, err)
	}
}

func TestTimelineOrderingPreservesDuplicateEvidence(t *testing.T) {
	at := instant("2026-10-02T09:00:00Z")
	source := domain.Snapshot{Sessions: []session.Session{finished(9, "OOT-1", "2026-10-02T09:00:00Z", "2026-10-02T09:00:00Z")}, Activities: []domain.Activity{
		{ID: 3, Type: "GIT_COMMIT", At: at, Text: "commit", Hash: "abc123456"},
		{ID: 2, Type: "NOTE", At: at, Text: "same"},
		{ID: 1, Type: "NOTE", At: at, Text: "same"},
		{ID: 4, Type: "NOTE", At: at.Add(time.Nanosecond), Text: "later"},
	}}
	view, err := domain.Build(source, at, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	var ids []int64
	for _, event := range view.Events {
		order = append(order, event.Kind)
		ids = append(ids, event.SourceID)
	}
	if !reflect.DeepEqual(order, []string{"START", "NOTE", "NOTE", "COMMIT", "STOP", "NOTE"}) || !reflect.DeepEqual(ids, []int64{9, 1, 2, 3, 9, 4}) {
		t.Fatalf("order: %v %v", order, ids)
	}
}

func TestDailyTotalPreservesSecondsBeforePresentation(t *testing.T) {
	now := instant("2026-10-02T12:00:00Z")
	view, err := domain.Build(domain.Snapshot{Sessions: []session.Session{
		finished(1, "OOT-1", "2026-10-02T09:00:00Z", "2026-10-02T09:00:40Z"),
		finished(2, "OOT-2", "2026-10-02T09:01:00Z", "2026-10-02T09:01:40Z"),
	}}, now, time.UTC)
	if err != nil || view.TotalSeconds != 80 || len(view.Tickets) != 2 || view.Tickets[0].Seconds != 40 || view.Tickets[1].Seconds != 40 {
		t.Fatalf("seconds lost: %+v %v", view, err)
	}
}

func TestRepositoryNamesFollowStoredSessionsAndActivities(t *testing.T) {
	now := instant("2026-10-02T12:00:00Z")
	repoA, repoB, oldRepo := "/work/api", "/work/frontend", "/work/yesterday"
	one := finished(1, "OOT-1", "2026-10-02T09:00:00Z", "2026-10-02T10:00:00Z")
	one.Repository = &repoA
	two := finished(2, "OOT-1", "2026-10-02T10:00:00Z", "2026-10-02T11:00:00Z")
	two.Repository = &repoB
	old := finished(3, "OOT-1", "2026-10-01T09:00:00Z", "2026-10-01T10:00:00Z")
	old.Repository = &oldRepo
	view, err := domain.Build(domain.Snapshot{Sessions: []session.Session{one, two, old}, Activities: []domain.Activity{
		{ID: 1, Type: "NOTE", TicketKey: "OOT-1", Repository: repoA, At: now},
		{ID: 2, Type: "GIT_COMMIT", TicketKey: "OOT-1", Repository: "/work/tools", At: now},
		{ID: 3, Type: "GIT_COMMIT", Repository: repoB, At: now},
	}}, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{repoA, repoB, "/work/tools"}
	if len(view.Tickets) != 1 || !reflect.DeepEqual(view.Tickets[0].Repositories, want) || view.TotalSeconds != 7200 {
		t.Fatalf("summaries=%+v total=%d", view.Tickets, view.TotalSeconds)
	}
	for _, event := range view.Events {
		if event.Repository == "" || event.Repository == oldRepo {
			t.Fatalf("incorrect event repository: %+v", event)
		}
	}
	if view.Unsessioned[1].Repository != "/work/tools" || view.Unassigned[0].Repository != repoB {
		t.Fatalf("commit repositories lost: %+v %+v", view.Unsessioned, view.Unassigned)
	}
}

func TestInvalidSnapshots(t *testing.T) {
	now := instant("2026-10-02T12:00:00Z")
	valid := session.Session{ID: 1, TicketKey: "OOT-1", Status: session.Active, StartedAt: now}
	for _, tc := range []struct {
		name       string
		sessions   []session.Session
		activities []domain.Activity
		location   *time.Location
	}{
		{"nil zone", nil, nil, nil},
		{"missing ticket", []session.Session{{Status: session.Active, StartedAt: now}}, nil, time.UTC},
		{"unknown status", []session.Session{{TicketKey: "OOT-1", Status: "invalid"}}, nil, time.UTC},
		{"completed without end", []session.Session{{TicketKey: "OOT-1", Status: session.Completed}}, nil, time.UTC},
		{"backward range", []session.Session{finished(1, "OOT-1", "2026-10-02T10:00:00Z", "2026-10-02T09:00:00Z")}, nil, time.UTC},
		{"future active", []session.Session{{TicketKey: "OOT-1", Status: session.Active, StartedAt: now.Add(time.Second)}}, nil, time.UTC},
		{"active with end", []session.Session{{TicketKey: "OOT-1", Status: session.Active, StartedAt: now, EndedAt: &now}}, nil, time.UTC},
		{"multiple active", []session.Session{valid, valid}, nil, time.UTC},
		{"invalid activity", nil, []domain.Activity{{Type: "OTHER", At: now}}, time.UTC},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := domain.Build(domain.Snapshot{Sessions: tc.sessions, Activities: tc.activities}, now, tc.location)
			if !errors.Is(err, domain.ErrInvalidData) {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestTicketTitlesUseFirstDailySessionOrCommit(t *testing.T) {
	now := instant("2026-10-02T12:00:00Z")
	first := finished(2, "OOT-1", "2026-10-01T23:30:00Z", "2026-10-02T00:30:00Z")
	first.Title = "Overnight work"
	later := finished(3, "OOT-1", "2026-10-02T09:00:00Z", "2026-10-02T10:00:00Z")
	later.Title = "Later session"
	old := finished(1, "OOT-1", "2026-10-01T09:00:00Z", "2026-10-01T10:00:00Z")
	old.Title = "Yesterday"
	zero := finished(4, "OOT-3", "2026-10-02T11:00:00Z", "2026-10-02T11:00:00Z")
	zero.Title = "meeting be"
	view, err := domain.Build(domain.Snapshot{Sessions: []session.Session{later, old, first, zero}, Activities: []domain.Activity{
		{ID: 8, TicketKey: "OOT-2", Type: "GIT_COMMIT", Text: "Later commit", At: now},
		{ID: 7, TicketKey: "OOT-2", Type: "GIT_COMMIT", Text: "Same timestamp, higher ID", At: now.Add(-time.Hour)},
		{ID: 6, TicketKey: "OOT-2", Type: "GIT_COMMIT", Text: " First commit\r\n\nCommit body", At: now.Add(-time.Hour)},
		{ID: 5, TicketKey: "OOT-2", Type: "GIT_COMMIT", Text: "Yesterday's commit", At: now.Add(-24 * time.Hour)},
		{ID: 4, TicketKey: "OOT-1", Type: "GIT_COMMIT", Text: "Commit on session ticket", At: now},
		{ID: 3, TicketKey: "OOT-4", Type: "NOTE", Text: "A note is not a session title", At: now},
	}}, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.TicketSummary{
		{Key: "OOT-1", Title: "Overnight work", Seconds: 5400},
		{Key: "OOT-2", Title: "First commit"},
		{Key: "OOT-3", Title: "meeting be"},
		{Key: "OOT-4"},
	}
	if !reflect.DeepEqual(view.Tickets, want) {
		t.Fatalf("titles: %+v want %+v", view.Tickets, want)
	}
}
