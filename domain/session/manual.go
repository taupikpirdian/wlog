package session

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrManualTime   = errors.New("invalid manual time: use unique HH:mm on today's local date, not in the future")
	ErrManualData   = errors.New("invalid existing session range")
	ErrOverlap      = errors.New("manual session overlaps an existing session")
	ErrActiveManual = errors.New("active session exists; review it with wl before adding a backdated start")
)

type ManualRequest struct {
	TicketKey, Title, From, To string
	Backdated                  bool
}
type ManualConflict struct {
	Existing Session
	Active   bool
}

func (e *ManualConflict) Error() string { return e.Unwrap().Error() }
func (e *ManualConflict) Unwrap() error {
	if e.Active {
		return ErrActiveManual
	}
	return ErrOverlap
}

func ValidateWallTime(value string) error {
	if len(value) != 5 || value[2] != ':' || value[0] < '0' || value[0] > '2' || value[1] < '0' || value[1] > '9' || value[3] < '0' || value[3] > '5' || value[4] < '0' || value[4] > '9' || value[:2] > "23" {
		return ErrManualTime
	}
	return nil
}

// ResolveWallTime considers every zone interval near the requested local date.
// A gap has no matching instant, while a fold has more than one.
func ResolveWallTime(value string, observedAt time.Time, location *time.Location) (time.Time, error) {
	if err := ValidateWallTime(value); err != nil {
		return time.Time{}, err
	}
	if location == nil {
		return time.Time{}, ErrManualTime
	}
	local := observedAt.In(location)
	hour := int(value[0]-'0')*10 + int(value[1]-'0')
	minute := int(value[3]-'0')*10 + int(value[4]-'0')
	wall := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, time.UTC)
	offsets := map[int]bool{}
	limit := wall.Add(26 * time.Hour)
	for probe := wall.Add(-26 * time.Hour); !probe.After(limit); {
		zoned := probe.In(location)
		_, offset := zoned.Zone()
		offsets[offset] = true
		_, end := zoned.ZoneBounds()
		if end.IsZero() || end.After(limit) {
			break
		}
		probe = end
	}
	var matches []time.Time
	for offset := range offsets {
		at := wall.Add(-time.Duration(offset) * time.Second)
		zoned := at.In(location)
		if zoned.Year() == local.Year() && zoned.Month() == local.Month() && zoned.Day() == local.Day() && zoned.Hour() == hour && zoned.Minute() == minute && zoned.Second() == 0 {
			matches = append(matches, at)
		}
	}
	if len(matches) != 1 {
		return time.Time{}, fmt.Errorf("%w: nonexistent or ambiguous local time %s", ErrManualTime, value)
	}
	if matches[0].After(observedAt) {
		return time.Time{}, ErrManualTime
	}
	return matches[0].UTC(), nil
}

func NewManualSession(request ManualRequest, now time.Time, location *time.Location) (Session, error) {
	title, err := ValidateTitle(request.Title)
	if err != nil {
		return Session{}, err
	}
	start, err := ResolveWallTime(request.From, now, location)
	if err != nil {
		return Session{}, err
	}
	value := Session{TicketKey: request.TicketKey, Title: title, StartedAt: start, Status: Active}
	if request.Backdated {
		return value, nil
	}
	end, err := ResolveWallTime(request.To, now, location)
	if err != nil {
		return Session{}, err
	}
	if !end.After(start) {
		return Session{}, fmt.Errorf("%w: from must precede to; cross-day ranges are unsupported", ErrManualTime)
	}
	return value.Complete(end)
}

// CheckManualAvailability compares parsed instants; active ranges are open-ended.
// Call it under the persistence writer lock before inserting the candidate.
func CheckManualAvailability(candidate Session, existing []Session) error {
	for _, s := range existing {
		if s.Status == Active {
			if s.EndedAt != nil {
				return ErrManualData
			}
		} else if s.Status != Completed || s.EndedAt == nil || s.EndedAt.Before(s.StartedAt) {
			return ErrManualData
		}
	}
	for _, s := range existing {
		if candidate.Status == Active && s.Status == Active {
			return &ManualConflict{Existing: s, Active: true}
		}
		if s.EndedAt != nil && s.EndedAt.Equal(s.StartedAt) {
			continue
		}
		if (s.EndedAt == nil || candidate.StartedAt.Before(*s.EndedAt)) && (candidate.EndedAt == nil || s.StartedAt.Before(*candidate.EndedAt)) {
			return &ManualConflict{Existing: s}
		}
	}
	return nil
}
