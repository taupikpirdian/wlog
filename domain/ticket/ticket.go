package ticket

import (
	"strings"
	"time"
)

// Ticket is the local identity used to group worklog records.
type Ticket struct {
	ID        int64
	Key       string
	Title     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New creates a validated ticket value. Blank titles are treated as absent.
func (p KeyPattern) New(key, title string) (Ticket, error) {
	if err := p.Validate(key); err != nil {
		return Ticket{}, err
	}
	return Ticket{Key: key, Title: NormalizeTitle(title)}, nil
}

// NormalizeTitle trims user-provided title whitespace and treats blank text as absent.
func NormalizeTitle(title string) *string {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	return &title
}
