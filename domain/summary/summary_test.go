package summary

import (
	"reflect"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/domain/dashboard"
	"github.com/taupikpirdian/wlog/domain/session"
)

func TestWeekDates(t *testing.T) {
	location := time.FixedZone("WIB", 7*3600)
	for _, tc := range []struct{ now, monday string }{
		{"2026-10-03T12:00:00+07:00", "2026-09-28"},
		{"2026-10-04T12:00:00+07:00", "2026-09-28"},
		{"2026-10-04T18:00:00Z", "2026-10-05"},
		{"2026-01-01T12:00:00+07:00", "2025-12-29"},
	} {
		t.Run(tc.now, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339, tc.now)
			dates, err := WeekDates(now, location)
			if err != nil || len(dates) != 7 {
				t.Fatalf("dates=%v err=%v", dates, err)
			}
			if dates[0].Format("2006-01-02") != tc.monday || dates[0].Weekday() != time.Monday || dates[6].Weekday() != time.Sunday {
				t.Fatalf("dates=%v", dates)
			}
			for i, date := range dates {
				if !date.Equal(dates[0].AddDate(0, 0, i)) || date.Hour() != 0 || date.Location() != location {
					t.Fatalf("date=%v", date)
				}
			}
		})
	}
	if _, err := WeekDates(time.Now(), nil); err == nil {
		t.Fatal("missing timezone accepted")
	}
}

func completed(id int64, title string, start, end time.Time) session.Session {
	return session.Session{ID: id, TicketKey: "OOT-1", Title: title, StartedAt: start, EndedAt: &end, Status: session.Completed}
}

func TestBuildDayAggregationAndDetails(t *testing.T) {
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	now := date.Add(36 * time.Hour)
	source := dashboard.Snapshot{
		Sessions: []session.Session{
			completed(2, "Pekerjaan 2", date.Add(11*time.Hour), date.Add(12*time.Hour)),
			completed(1, " Pekerjaan   1 ", date.Add(9*time.Hour), date.Add(10*time.Hour)),
			completed(3, "Other date", date.Add(-48*time.Hour), date.Add(-47*time.Hour)),
		},
		Activities: []dashboard.Activity{
			{ID: 4, Type: "GIT_COMMIT", Text: "Pekerjaan 3\n\nPekerjaan 4", At: date.Add(12 * time.Hour)},
			{ID: 1, Type: "NOTE", Text: "Pekerjaan 1", At: date.Add(9*time.Hour + time.Minute)},
			{ID: 2, Type: "NOTE", Text: "Pekerjaan 2", At: date.Add(11*time.Hour + time.Minute)},
			{ID: 3, Type: "NOTE", Text: "Other date", At: date.Add(24 * time.Hour)},
		},
	}
	day, err := BuildDay(source, date, now, time.UTC)
	if err != nil || day.Seconds != 7200 || !day.HasWorklog {
		t.Fatalf("day=%+v err=%v", day, err)
	}
	if want := []string{"Pekerjaan 1", "Pekerjaan 2", "Pekerjaan 3", "Pekerjaan 4"}; !reflect.DeepEqual(day.Details, want) {
		t.Fatalf("details=%v", day.Details)
	}
	// Empty and future dates must still be selectable with zero duration.
	for _, date := range []time.Time{date.AddDate(0, 0, -1), date.AddDate(0, 0, 2)} {
		day, err := BuildDay(source, date, now, time.UTC)
		if err != nil || day.HasWorklog || day.Seconds != 0 || len(day.Details) != 0 {
			t.Fatalf("empty=%+v err=%v", day, err)
		}
	}
}

func TestBuildDayClipsMidnightAndActiveSessions(t *testing.T) {
	location := time.FixedZone("WIB", 7*3600)
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, location)
	now := date.Add(10 * time.Hour)
	source := dashboard.Snapshot{Sessions: []session.Session{
		completed(1, "Overnight", date.Add(-30*time.Minute), date.Add(30*time.Minute)),
		{ID: 2, Title: "Active", StartedAt: date.Add(9 * time.Hour), Status: session.Active},
	}}
	day, err := BuildDay(source, date, now, location)
	if err != nil || day.Seconds != 5400 {
		t.Fatalf("day=%+v err=%v", day, err)
	}
	previous, err := BuildDay(source, date.AddDate(0, 0, -1), now, location)
	if err != nil || previous.Seconds != 1800 || !reflect.DeepEqual(previous.Details, []string{"Overnight"}) {
		t.Fatalf("previous=%+v err=%v", previous, err)
	}
	// A long session must not spill its entire duration into an earlier day.
	source.Sessions = []session.Session{completed(1, "Long", date.Add(-time.Hour), date.Add(25*time.Hour))}
	day, err = BuildDay(source, date, date.Add(48*time.Hour), location)
	if err != nil || day.Seconds != 24*3600 {
		t.Fatalf("day=%+v err=%v", day, err)
	}
}

func TestBuildDayActivityWithoutSession(t *testing.T) {
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	source := dashboard.Snapshot{Activities: []dashboard.Activity{{Type: "GIT_COMMIT", Text: "Fix bug", At: date.Add(time.Hour)}}}
	day, err := BuildDay(source, date, date.Add(2*time.Hour), time.UTC)
	if err != nil || !day.HasWorklog || day.Seconds != 0 || !reflect.DeepEqual(day.Details, []string{"Fix bug"}) {
		t.Fatalf("day=%+v err=%v", day, err)
	}
}

func TestBuildDayInvalidData(t *testing.T) {
	date := time.Now()
	for _, s := range []session.Session{
		{Status: session.Completed},
		completed(1, "Bad range", date, date.Add(-time.Hour)),
		{Status: session.Active, StartedAt: date.Add(time.Hour)},
		{Status: "invalid"},
	} {
		if _, err := BuildDay(dashboard.Snapshot{Sessions: []session.Session{s}}, date, date, time.UTC); err == nil {
			t.Fatalf("accepted %+v", s)
		}
	}
	if _, err := BuildDay(dashboard.Snapshot{}, date, date, nil); err == nil {
		t.Fatal("missing timezone accepted")
	}
}
