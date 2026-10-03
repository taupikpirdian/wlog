package summary

import (
	"context"
	"errors"
	"strings"

	"github.com/taupikpirdian/wlog/application/bootstrap"
)

// GeneratedTicket is the final Markdown artifact. Its title, sections and tables
// belong to the loaded skill, and are never mapped into the summary schema.
type GeneratedTicket struct {
	Content string
}

type TicketAIAgent interface {
	GenerateTicket(context.Context, AIRequest, ProgressHandler) (*GeneratedTicket, error)
}

func ValidateGeneratedTicket(ticket *GeneratedTicket) error {
	if ticket == nil || strings.TrimSpace(ticket.Content) == "" {
		return errors.New("AI produced an empty Jira ticket; retry generation or check the configured agent")
	}
	return nil
}

func GenerateTicketAI(ctx context.Context, value TicketAIContext, config bootstrap.AIConfig, factory AIAgentFactory, worklogsOnly bool, fallbackDirectory string, skills TicketSkillResolver, progress ProgressHandler) (GeneratedTicket, error) {
	if err := ctx.Err(); err != nil {
		return GeneratedTicket{}, err
	}
	language, err := ResolveOutputLanguage(value.OutputLanguage)
	if err != nil {
		return GeneratedTicket{}, err
	}
	value.OutputLanguage = language
	value.TicketOnly = true
	if len(value.Repositories) == 0 && !worklogsOnly {
		return GeneratedTicket{}, errors.New("source-code context is unavailable; explicit worklogs-only consent is required")
	}
	directory := fallbackDirectory
	if len(value.Repositories) > 0 {
		directory = value.Repositories[0].Path
	}
	var skill TicketSkill
	if skills != nil {
		paths := []string{directory}
		if len(value.Repositories) > 0 {
			paths = nil
			for _, repo := range value.Repositories {
				paths = append(paths, repo.Path)
			}
		}
		for _, path := range paths {
			skill, err = skills.Resolve(ctx, config.Provider, path, progress)
			if err != nil {
				if ctx.Err() != nil {
					return GeneratedTicket{}, ctx.Err()
				}
				emitProgress(progress, ProgressEvent{Type: ProgressWarning, Message: "Failed to load ticket-generator skill"})
				emitProgress(progress, ProgressEvent{Type: ProgressInfo, Message: "Using built-in wlog ticket generator"})
				skill = TicketSkill{}
				continue
			}
			if skill.Loaded {
				// A repository-local native skill is discoverable from this root.
				directory = path
				break
			}
		}
	} else {
		emitProgress(progress, ProgressEvent{Type: ProgressWarning, Message: "ticket-generator skill resolver unavailable"})
		emitProgress(progress, ProgressEvent{Type: ProgressInfo, Message: "Using built-in wlog ticket generator"})
	}
	agent, err := factory.Create(config)
	if err != nil {
		return GeneratedTicket{}, err
	}
	ticketAgent, ok := agent.(TicketAIAgent)
	if !ok {
		return GeneratedTicket{}, errors.New("configured AI adapter does not support Markdown ticket generation")
	}
	// One invocation inspects all recorded repositories and produces one ticket.
	// Joining per-repository tickets would duplicate titles and corrupt skill format.
	emitProgress(progress, ProgressEvent{Type: ProgressStatus, RepositoryPath: directory, Message: "Starting " + config.Provider})
	ticket, err := ticketAgent.GenerateTicket(ctx, AIRequest{Context: value, WorkingDirectory: directory, WorklogsOnly: len(value.Repositories) == 0, Skill: skill}, progress)
	if err != nil {
		return GeneratedTicket{}, err
	}
	if err := ValidateGeneratedTicket(ticket); err != nil {
		return GeneratedTicket{}, err
	}
	return *ticket, nil
}
