package session_test

import (
	"errors"
	"testing"
	"time"

	domain "github.com/taupikpirdian/wlog/domain/session"
)

func TestManualTimeAndCandidate(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 15, 0, time.FixedZone("Jakarta", 7*3600))
	req := domain.ManualRequest{TicketKey: "OOT-1", Title: "  Fix tax  ", From: "09:00", To: "11:00"}
	value, err := domain.NewManualSession(req, now, now.Location())
	if err != nil || value.Title != "Fix tax" || value.Status != domain.Completed || *value.DurationSeconds != 7200 || value.StartedAt.Location() != time.UTC || value.StartedAt.Hour() != 2 {
		t.Fatalf("candidate: %+v %v", value, err)
	}
	req.Backdated = true
	req.From = "12:00"
	value, err = domain.NewManualSession(req, now, now.Location())
	if err != nil || value.Status != domain.Active || value.EndedAt != nil || value.DurationSeconds != nil {
		t.Fatalf("active: %+v %v", value, err)
	}
	for _, input := range []string{"", "9:00", "24:00", "09:60", "09:00:30", " 09:00", "09:00 ", "ab:cd", "2026-10-02T09:00:00Z"} {
		if err := domain.ValidateWallTime(input); !errors.Is(err, domain.ErrManualTime) {
			t.Fatalf("accepted %q: %v", input, err)
		}
	}
	for _, tc := range []struct {
		name     string
		request  domain.ManualRequest
		location *time.Location
	}{
		{"empty title", domain.ManualRequest{Title: "  ", From: "09:00", To: "10:00"}, now.Location()},
		{"bad from", domain.ManualRequest{Title: "x", From: "bad", To: "10:00"}, now.Location()},
		{"bad to", domain.ManualRequest{Title: "x", From: "09:00", To: "bad"}, now.Location()},
		{"future end", domain.ManualRequest{Title: "x", From: "09:00", To: "13:00"}, now.Location()},
		{"future since", domain.ManualRequest{Title: "x", From: "13:00", Backdated: true}, now.Location()},
		{"same", domain.ManualRequest{Title: "x", From: "09:00", To: "09:00"}, now.Location()},
		{"rollover", domain.ManualRequest{Title: "x", From: "23:00", To: "01:00"}, now.Location()},
		{"nil zone", domain.ManualRequest{Title: "x", From: "09:00", To: "11:00"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := domain.NewManualSession(tc.request, now, tc.location); err == nil {
				t.Fatal("accepted invalid candidate")
			}
		})
	}
}

func TestManualTimeOffsetTransitions(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		now      time.Time
		from, to string
		seconds  int64
	}{
		{time.Date(2026, 3, 8, 12, 0, 0, 0, loc), "02:30", "04:00", 0},
		{time.Date(2026, 11, 1, 12, 0, 0, 0, loc), "01:30", "03:00", 0},
		{time.Date(2026, 3, 8, 12, 0, 0, 0, loc), "01:00", "04:00", 7200},
		{time.Date(2026, 11, 1, 12, 0, 0, 0, loc), "00:00", "03:00", 14400},
	} {
		s, err := domain.NewManualSession(domain.ManualRequest{Title: "x", From: tc.from, To: tc.to}, tc.now, loc)
		if tc.seconds == 0 {
			if !errors.Is(err, domain.ErrManualTime) {
				t.Fatalf("gap/fold: %+v %v", s, err)
			}
		} else if err != nil || *s.DurationSeconds != tc.seconds {
			t.Fatalf("elapsed: %+v %v", s, err)
		}
	}
}

func TestManualRangeValidation(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	candidate, _ := domain.NewManualSession(domain.ManualRequest{TicketKey: "OOT-1", Title: "x", From: "09:00", To: "11:00"}, now, time.UTC)
	completed := func(from, to string) domain.Session {
		s, err := domain.NewManualSession(domain.ManualRequest{TicketKey: "OOT-2", Title: "old", From: from, To: to}, now, time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		s.ID = 12
		return s
	}
	for _, tc := range []struct {
		name     string
		existing domain.Session
		conflict bool
	}{
		{"overlap", completed("09:30", "10:30"), true}, {"cover", completed("08:00", "12:00"), true},
		{"before adjacent", completed("08:00", "09:00"), false}, {"after adjacent", completed("11:00", "12:00"), false},
		{"active overlap", domain.Session{ID: 1, TicketKey: "OOT-2", StartedAt: now.Add(-2 * time.Hour), Status: domain.Active}, true},
		{"active adjacent", domain.Session{ID: 1, TicketKey: "OOT-2", StartedAt: now.Add(-time.Hour), Status: domain.Active}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.CheckManualAvailability(candidate, []domain.Session{tc.existing})
			if tc.conflict {
				var conflict *domain.ManualConflict
				if !errors.As(err, &conflict) || !errors.Is(err, domain.ErrOverlap) || conflict.Existing.ID != tc.existing.ID || conflict.Error() == "" {
					t.Fatalf("conflict: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
	zero := completed("08:00", "09:00")
	zero.StartedAt = *zero.EndedAt
	if err := domain.CheckManualAvailability(candidate, []domain.Session{zero}); err != nil {
		t.Fatal(err)
	}
	active, _ := domain.NewManualSession(domain.ManualRequest{TicketKey: "OOT-1", Title: "x", From: "11:00", Backdated: true}, now, time.UTC)
	otherActive := domain.Session{ID: 3, Status: domain.Active, StartedAt: now}
	err := domain.CheckManualAvailability(active, []domain.Session{otherActive})
	if !errors.Is(err, domain.ErrActiveManual) {
		t.Fatal(err)
	}
	if err := domain.CheckManualAvailability(active, []domain.Session{completed("09:00", "11:00")}); err != nil {
		t.Fatal(err)
	}
	if err := domain.CheckManualAvailability(active, []domain.Session{completed("11:00", "12:00")}); !errors.Is(err, domain.ErrOverlap) {
		t.Fatal(err)
	}
	for _, old := range []domain.Session{
		{Status: "bad"}, {Status: domain.Completed}, {Status: domain.Active, EndedAt: &now},
		{Status: domain.Completed, StartedAt: now.Add(time.Hour), EndedAt: &now},
	} {
		if err := domain.CheckManualAvailability(candidate, []domain.Session{old}); !errors.Is(err, domain.ErrManualData) {
			t.Fatalf("bad data accepted: %v", err)
		}
	}
}
