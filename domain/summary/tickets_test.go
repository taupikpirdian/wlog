package summary

import (
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/domain/dashboard"
	"github.com/taupikpirdian/wlog/domain/session"
)

func TestTicketsForSelectedDate(t *testing.T) {
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	source := dashboard.Snapshot{Sessions: []session.Session{
		completed(1, "A", date.Add(9*time.Hour), date.Add(10*time.Hour)),
		completed(2, "B", date.Add(11*time.Hour), date.Add(12*time.Hour)),
		completed(3, "Yesterday", date.Add(-15*time.Hour), date.Add(-14*time.Hour)),
	}}
	source.Sessions[2].TicketKey = "OOT-3"
	for _, multiple := range []bool{false, true} {
		if multiple {
			source.Sessions[1].TicketKey = "OOT-2"
		}
		tickets, err := TicketsForDay(source, date, date.Add(36*time.Hour), time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		if !multiple && (len(tickets) != 1 || tickets[0].Key != "OOT-1" || tickets[0].Day.Seconds != 7200) {
			t.Fatalf("single=%+v", tickets)
		}
		if multiple && (len(tickets) != 2 || tickets[0].Day.Seconds != 3600 || tickets[1].Day.Seconds != 3600 || tickets[1].Key != "OOT-2") {
			t.Fatalf("multiple=%+v", tickets)
		}
	}
	filtered := FilterTicket(source, "OOT-1")
	day, err := BuildDay(filtered, date, date.Add(36*time.Hour), time.UTC)
	if err != nil || len(day.Details) != 1 || day.Details[0] != "A" {
		t.Fatalf("filtered=%+v %v", day, err)
	}
}

func TestTicketsActivityOnlyAndNoWorklog(t *testing.T) {
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	source := dashboard.Snapshot{Activities: []dashboard.Activity{
		{TicketKey: "OOT-1", Type: "GIT_COMMIT", Text: "Change", At: date.Add(time.Hour)},
		{TicketKey: "", Type: "GIT_COMMIT", Text: "Unassigned", At: date.Add(time.Hour)},
	}}
	tickets, err := TicketsForDay(source, date, date.Add(48*time.Hour), time.UTC)
	if err != nil || len(tickets) != 1 || tickets[0].Day.Seconds != 0 || tickets[0].Day.CommitCount != 1 {
		t.Fatalf("tickets=%+v err=%v", tickets, err)
	}
	tickets, err = TicketsForDay(source, date.AddDate(0, 0, 1), date.Add(48*time.Hour), time.UTC)
	if err != nil || len(tickets) != 0 {
		t.Fatalf("empty=%+v err=%v", tickets, err)
	}
}

func TestTicketsForWeekDeduplicatesAndClipsLocalWeek(t *testing.T) {
	location := time.FixedZone("Asia/Jakarta", 7*3600)
	monday := time.Date(2026, 9, 28, 0, 0, 0, 0, location)
	now := monday.AddDate(0, 0, 5).Add(12 * time.Hour)
	source := dashboard.Snapshot{Sessions: []session.Session{
		completed(1, "Monday", monday.Add(time.Hour), monday.Add(2*time.Hour)),
		completed(2, "Friday", monday.AddDate(0, 0, 4).Add(time.Hour), monday.AddDate(0, 0, 4).Add(3*time.Hour)),
		completed(3, "Cross-week", monday.Add(-30*time.Minute), monday.Add(30*time.Minute)),
		completed(4, "Previous week", monday.Add(-3*time.Hour), monday.Add(-2*time.Hour)),
		completed(5, "Next week", monday.AddDate(0, 0, 7).Add(time.Hour), monday.AddDate(0, 0, 7).Add(2*time.Hour)),
	}, Activities: []dashboard.Activity{
		{TicketKey: "OOT-2", Type: "NOTE", Text: "Analysis", At: monday.AddDate(0, 0, 2)},
		{TicketKey: "OLD", Type: "NOTE", Text: "Old", At: monday.Add(-time.Second)},
		{TicketKey: "FUTURE", Type: "NOTE", Text: "Future", At: now.Add(time.Hour)},
		{Type: "NOTE", Text: "Unassigned", At: monday.Add(time.Hour)},
	}}
	source.Sessions[3].TicketKey = "PREVIOUS"
	source.Sessions[4].TicketKey = "NEXT"
	tickets, err := TicketsForWeek(source, now, location)
	if err != nil || len(tickets) != 2 || tickets[0].Key != "OOT-1" || tickets[0].Day.Seconds != 12600 || tickets[1].Key != "OOT-2" || tickets[1].Day.Seconds != 0 {
		t.Fatalf("weekly tickets=%+v error=%v", tickets, err)
	}
	if !tickets[0].Day.HasWorklog || !tickets[0].Day.Date.Equal(monday) {
		t.Fatalf("incorrect local week: %+v", tickets[0])
	}
}

func TestTicketsForWeekEmptyAndInvalidTimezone(t *testing.T) {
	if tickets, err := TicketsForWeek(dashboard.Snapshot{}, time.Now(), time.UTC); err != nil || len(tickets) != 0 {
		t.Fatalf("tickets=%+v error=%v", tickets, err)
	}
	if _, err := TicketsForWeek(dashboard.Snapshot{}, time.Now(), nil); err == nil {
		t.Fatal("missing timezone accepted")
	}
}
