package activity

import (
	"testing"

	"github.com/taupikpirdian/wlog/domain/session"
)

func TestCommitAttributionPriority(t *testing.T) {
	active := &session.Session{ID: 12, TicketKey: "OOT-1", Status: session.Active}
	for _, tc := range []struct {
		name, message, branch, key, source string
		active                             *session.Session
		attached, mismatch                 bool
	}{
		{"matching", "OOT-1", "OOT-2", "OOT-1", "MESSAGE", active, true, false},
		{"mismatch", "OOT-2", "OOT-1", "OOT-2", "MESSAGE", active, false, true},
		{"without session", "OOT-2", "OOT-3", "OOT-2", "MESSAGE", nil, false, false},
		{"session fallback", "", "OOT-2", "OOT-1", "ACTIVE_SESSION", active, true, false},
		{"branch fallback", "", "OOT-2", "OOT-2", "BRANCH", nil, false, false},
		{"unassigned", "", "", "", "UNASSIGNED", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveCommit(tc.message, tc.branch, tc.active)
			if got.TicketKey != tc.key || got.Source != tc.source || (got.SessionID != nil) != tc.attached {
				t.Fatalf("attribution=%+v", got)
			}
			mismatch := false
			for _, warning := range got.Warnings {
				if warning == "ticket_mismatch" {
					mismatch = true
				}
			}
			if mismatch != tc.mismatch {
				t.Fatalf("warnings=%v", got.Warnings)
			}
		})
	}
}
