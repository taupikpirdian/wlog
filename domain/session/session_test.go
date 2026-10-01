package session

import (
	"errors"
	"testing"
	"time"
)

func TestCompleteUsesActualSeconds(t *testing.T) {
	start := time.Date(2026, 10, 1, 23, 59, 59, 900000000, time.UTC)
	for _, tc := range []struct {
		name    string
		elapsed time.Duration
		seconds int64
		err     error
	}{
		{"subsecond", 50 * time.Millisecond, 0, nil},
		{"cross midnight", 1500 * time.Millisecond, 1, nil},
		{"long session", 25*time.Hour + 37*time.Second, 90037, nil},
		{"clock backwards", -time.Nanosecond, 0, ErrClockBackwards},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := Session{Status: Active, StartedAt: start}
			got, err := original.Complete(start.Add(tc.elapsed))
			if !errors.Is(err, tc.err) {
				t.Fatalf("error: %v", err)
			}
			if err == nil && (*got.DurationSeconds != tc.seconds || got.Status != Completed) {
				t.Fatalf("completed: %+v", got)
			}
			if original.Status != Active || original.EndedAt != nil {
				t.Fatal("mutated original snapshot")
			}
		})
	}
}
