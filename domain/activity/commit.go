package activity

import (
	"time"

	"github.com/taupikpirdian/wlog/domain/session"
)

const CommitType = "GIT_COMMIT"

type Commit struct {
	Repository string
	Hash       string
	Message    *string
	Branch     *string
	Files      []string
	Insertions *int64
	Deletions  *int64
	CreatedAt  time.Time
	Metadata   CommitMetadata
}

type CommitMetadata struct {
	Version      int               `json:"version"`
	Source       string            `json:"source"`
	CapturedAt   time.Time         `json:"captured_at"`
	TimeSource   string            `json:"time_source"`
	TicketSource string            `json:"ticket_source"`
	Status       map[string]string `json:"capture_status"`
	DiffBasis    string            `json:"diff_basis"`
	RenameMode   string            `json:"rename_mode"`
	StatScope    string            `json:"diff_stat_scope"`
	BinaryFiles  []string          `json:"binary_files"`
	Warnings     []string          `json:"warnings"`
}

type Attribution struct {
	TicketKey    string
	Source       string
	SessionID    *int64
	ActiveTicket string
	Warnings     []string
}

func ResolveCommit(messageKey, branchKey string, active *session.Session) Attribution {
	value := Attribution{Source: "UNASSIGNED"}
	switch {
	case messageKey != "":
		value.TicketKey, value.Source = messageKey, "MESSAGE"
	case active != nil:
		value.TicketKey, value.Source = active.TicketKey, "ACTIVE_SESSION"
	case branchKey != "":
		value.TicketKey, value.Source = branchKey, "BRANCH"
	}
	if active != nil {
		value.ActiveTicket = active.TicketKey
		if value.TicketKey == active.TicketKey {
			id := active.ID
			value.SessionID = &id
		} else {
			value.Warnings = append(value.Warnings, "ticket_mismatch")
		}
	}
	if value.TicketKey == "" {
		value.Warnings = append(value.Warnings, "unassigned")
	}
	return value
}

type CaptureResult struct {
	ID              int64
	AlreadyCaptured bool
	Attribution     Attribution
	Warnings        []string
}
