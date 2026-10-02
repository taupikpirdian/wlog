package summary

import "context"

// Native instructions stay in the installed skill; Custom agents receive the
// complete instructions that wlog actually read, never just a directory name.
type TicketSkill struct {
	Name         string
	Path         string
	Loaded       bool
	Native       bool
	Invocation   string
	Instructions string
}
type TicketSkillResolver interface {
	Resolve(context.Context, string, string, ProgressHandler) (TicketSkill, error)
}
