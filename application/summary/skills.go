package summary

import (
	"context"
	"fmt"
	"strings"
)

// Native instructions stay in the installed skill; Custom agents receive the
// complete instructions that wlog actually read, never just a directory name.
type TicketSkill struct {
	Name          string
	Path          string
	Loaded        bool
	Native        bool
	Invocation    string
	Instructions  string
	CheckedPaths  []string
	FailureReason string
}
type TicketSkillResolver interface {
	Resolve(context.Context, string, string, string, ProgressHandler) (TicketSkill, error)
}

const TicketGeneratorSkill = "ticket-generator"
const RootCauseSummarySkill = "root-cause-summary"

func (s TicketSkill) NameOrDefault() string {
	if s.Name == "" {
		return TicketGeneratorSkill
	}
	return s.Name
}

func ResolveTicketSkillName(value, fallback string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		name = fallback
	}
	if name != TicketGeneratorSkill && name != RootCauseSummarySkill {
		return "", fmt.Errorf("unsupported AI skill %q; supported skills: ticket-generator, root-cause-summary", name)
	}
	return name, nil
}
