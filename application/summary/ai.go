package summary

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	"github.com/taupikpirdian/wlog/domain/dashboard"
)

type CommitRange struct{ Start, End string }
type CodeChange struct {
	CommitRange
	SelectedDate bool
	Diff         string
}
type RepositoryAIContext struct {
	Name, Path string
	Branches   []string
	Changes    []CodeChange
}
type TicketAIContext struct {
	TicketKey         string
	Summary           Result
	AllTicketWorklogs dashboard.Snapshot
	Repositories      []RepositoryAIContext
	Warnings          []string
	TicketOnly        bool
	OutputLanguage    OutputLanguage
}
type GitService interface {
	ValidateRepository(context.Context, string) (string, error)
	Change(context.Context, string, string) (CodeChange, error)
}
type AIRequest struct {
	Context          TicketAIContext
	Repository       *RepositoryAIContext
	WorkingDirectory string
	WorklogsOnly     bool
	Skill            TicketSkill
}
type WorklogText struct {
	Details              []string `json:"details"`
	Results              []string `json:"results"`
	EnvironmentVariables []string `json:"environment_variables,omitempty"`
}
type TicketDescription struct {
	Background         string   `json:"background"`
	ProblemRequirement string   `json:"problem_requirement"`
	Scope              []string `json:"scope"`
	ExpectedResult     string   `json:"expected_result"`
	TechnicalNotes     string   `json:"technical_notes"`
}
type AIResponse struct {
	Worklog           WorklogText       `json:"worklog"`
	TicketDescription TicketDescription `json:"ticket_description"`
}
type AIAgent interface {
	Generate(context.Context, AIRequest, ProgressHandler) (*AIResponse, error)
	Capabilities() AICapabilities
}
type AIAgentFactory interface {
	Create(bootstrap.AIConfig) (AIAgent, error)
}
type AIResult struct {
	Context  TicketAIContext
	Response AIResponse
}

// CollectCode uses only captured ticket commit identities, not the current HEAD.
// Each parent..commit range is atomic, so overlapping sessions cannot add the
// same change twice or introduce uncaptured intervening commits.
func CollectCode(ctx context.Context, value TicketAIContext, git GitService, progress ...ProgressHandler) (TicketAIContext, error) {
	var handler ProgressHandler
	if len(progress) > 0 {
		handler = progress[0]
	}
	paths := map[string][]dashboard.Activity{}
	for _, s := range value.AllTicketWorklogs.Sessions {
		if s.Repository == nil || *s.Repository == "" {
			value.Warnings = append(value.Warnings, fmt.Sprintf("Session %d has no recorded repository path; source-code context unavailable.", s.ID))
		} else if _, ok := paths[*s.Repository]; !ok {
			paths[*s.Repository] = nil
		}
	}
	for _, a := range value.AllTicketWorklogs.Activities {
		if a.Type != "GIT_COMMIT" {
			continue
		}
		if a.Repository == "" {
			value.Warnings = append(value.Warnings, fmt.Sprintf("Commit %s has no recorded repository path.", a.Hash))
			continue
		}
		paths[a.Repository] = append(paths[a.Repository], a)
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	// Canonical repository path plus resolved end hash deduplicates path aliases.
	seen := map[string]bool{}
	for _, path := range ordered {
		if err := ctx.Err(); err != nil {
			return TicketAIContext{}, err
		}
		emitProgress(handler, ProgressEvent{Type: ProgressStatus, RepositoryPath: path, Message: "Validating repository"})
		canonical, err := git.ValidateRepository(ctx, path)
		if err != nil {
			value.Warnings = append(value.Warnings, sourceWarning(path, err.Error()))
			continue
		}
		repo := RepositoryAIContext{Name: repositoryBase(canonical), Path: canonical}
		branches := map[string]bool{}
		for _, a := range paths[path] {
			emitProgress(handler, ProgressEvent{Type: ProgressStatus, RepositoryPath: canonical, Message: "Loading captured commit " + a.Hash})
			change, err := git.Change(ctx, canonical, a.Hash)
			if err != nil {
				value.Warnings = append(value.Warnings, sourceWarning(path, fmt.Sprintf("Captured commit %s: %s", a.Hash, err)))
				continue
			}
			identity := canonical + ":" + change.End
			selected := !a.At.Before(value.Summary.Day.Date) && a.At.Before(value.Summary.Day.Date.AddDate(0, 0, 1))
			if seen[identity] {
				for i := range value.Repositories {
					for j := range value.Repositories[i].Changes {
						if value.Repositories[i].Path == canonical && value.Repositories[i].Changes[j].End == change.End {
							value.Repositories[i].Changes[j].SelectedDate = value.Repositories[i].Changes[j].SelectedDate || selected
						}
					}
				}
				for i := range repo.Changes {
					if repo.Changes[i].End == change.End {
						repo.Changes[i].SelectedDate = repo.Changes[i].SelectedDate || selected
					}
				}
				continue
			}
			seen[identity] = true
			change.SelectedDate = selected
			repo.Changes = append(repo.Changes, change)
			if a.Branch != "" {
				branches[a.Branch] = true
			}
		}
		for branch := range branches {
			repo.Branches = append(repo.Branches, branch)
		}
		sort.Strings(repo.Branches)
		sort.Slice(repo.Changes, func(i, j int) bool { return repo.Changes[i].End < repo.Changes[j].End })
		if len(paths[path]) == 0 {
			value.Warnings = append(value.Warnings, sourceWarning(path, "No captured commit hashes are recorded for this ticket's sessions."))
		}
		if len(repo.Changes) > 0 {
			value.Repositories = append(value.Repositories, repo)
		}
	}
	if err := ctx.Err(); err != nil {
		return TicketAIContext{}, err
	}
	return value, nil
}

func sourceWarning(path, reason string) string {
	return fmt.Sprintf("Repository:\n  %s\n\n⚠ Source-code context unavailable\n\nReason:\n  %s", path, reason)
}

func repositoryBase(path string) string {
	parts := strings.Split(strings.TrimRight(strings.ReplaceAll(path, "\\", "/"), "/"), "/")
	return parts[len(parts)-1]
}

func GenerateAI(ctx context.Context, value TicketAIContext, config bootstrap.AIConfig, factory AIAgentFactory, worklogsOnly bool, fallbackDirectory string, progress ...ProgressHandler) (AIResult, error) {
	var handler ProgressHandler
	if len(progress) > 0 {
		handler = progress[0]
	}
	return generateAI(ctx, value, config, factory, worklogsOnly, fallbackDirectory, nil, handler)
}

func GenerateTicketAI(ctx context.Context, value TicketAIContext, config bootstrap.AIConfig, factory AIAgentFactory, worklogsOnly bool, fallbackDirectory string, skills TicketSkillResolver, progress ProgressHandler) (AIResult, error) {
	value.TicketOnly = true
	return generateAI(ctx, value, config, factory, worklogsOnly, fallbackDirectory, skills, progress)
}

func generateAI(ctx context.Context, value TicketAIContext, config bootstrap.AIConfig, factory AIAgentFactory, worklogsOnly bool, fallbackDirectory string, skills TicketSkillResolver, handler ProgressHandler) (AIResult, error) {
	language, err := ResolveOutputLanguage(value.OutputLanguage)
	if err != nil {
		return AIResult{}, err
	}
	value.OutputLanguage = language
	if len(value.Repositories) == 0 && !worklogsOnly {
		return AIResult{}, errors.New("source-code context is unavailable; explicit worklogs-only consent is required")
	}
	agent, err := factory.Create(config)
	if err != nil {
		return AIResult{}, err
	}
	var responses []AIResponse
	resolveSkill := func(directory string) (TicketSkill, error) {
		if skills == nil {
			return TicketSkill{}, nil
		}
		return skills.Resolve(ctx, config.Provider, directory, handler)
	}
	if len(value.Repositories) == 0 {
		skill, err := resolveSkill(fallbackDirectory)
		if err != nil {
			return AIResult{}, err
		}
		emitProgress(handler, ProgressEvent{Type: ProgressStatus, Message: "Starting " + config.Provider})
		response, err := agent.Generate(ctx, AIRequest{Context: value, WorkingDirectory: fallbackDirectory, WorklogsOnly: true, Skill: skill}, handler)
		if err != nil {
			return AIResult{}, err
		}
		if err := ValidateAIResponse(response); err != nil {
			return AIResult{}, err
		}
		// Worklogs alone cannot establish that an environment variable was added.
		response.Worklog.EnvironmentVariables = nil
		responses = append(responses, *response)
	} else {
		for i := range value.Repositories {
			repo := value.Repositories[i]
			skill, err := resolveSkill(repo.Path)
			if err != nil {
				return AIResult{}, err
			}
			emitProgress(handler, ProgressEvent{Type: ProgressStatus, RepositoryPath: repo.Path, Message: fmt.Sprintf("Repository %d/%d: starting %s", i+1, len(value.Repositories), config.Provider)})
			for _, change := range repo.Changes {
				rangeText := change.Start + ".." + change.End
				if change.Start == "" {
					rangeText = "root.." + change.End
				}
				emitProgress(handler, ProgressEvent{Type: ProgressInfo, RepositoryPath: repo.Path, Message: "Commit range: " + rangeText})
			}
			response, err := agent.Generate(ctx, AIRequest{Context: value, Repository: &repo, WorkingDirectory: repo.Path, Skill: skill}, handler)
			if err != nil {
				return AIResult{}, fmt.Errorf("AI generation for %s: %w", repo.Path, err)
			}
			if err := ValidateAIResponse(response); err != nil {
				return AIResult{}, err
			}
			if value.TicketOnly {
				response.Worklog.EnvironmentVariables = nil
			} else {
				response.Worklog.EnvironmentVariables = environmentVariablesWithEvidence(response.Worklog.EnvironmentVariables, repo)
			}
			responses = append(responses, *response)
			emitProgress(handler, ProgressEvent{Type: ProgressStatus, RepositoryPath: repo.Path, Message: "Repository analysis completed"})
		}
	}
	var combined AIResponse
	emitProgress(handler, ProgressEvent{Type: ProgressStatus, Message: "Combining repository results"})
	for _, response := range responses {
		combined.Worklog.Details = append(combined.Worklog.Details, response.Worklog.Details...)
		combined.Worklog.Results = append(combined.Worklog.Results, response.Worklog.Results...)
		combined.Worklog.EnvironmentVariables = append(combined.Worklog.EnvironmentVariables, response.Worklog.EnvironmentVariables...)
		combined.TicketDescription.Background = joinText(combined.TicketDescription.Background, response.TicketDescription.Background)
		combined.TicketDescription.ProblemRequirement = joinText(combined.TicketDescription.ProblemRequirement, response.TicketDescription.ProblemRequirement)
		combined.TicketDescription.Scope = append(combined.TicketDescription.Scope, response.TicketDescription.Scope...)
		combined.TicketDescription.ExpectedResult = joinText(combined.TicketDescription.ExpectedResult, response.TicketDescription.ExpectedResult)
		combined.TicketDescription.TechnicalNotes = joinText(combined.TicketDescription.TechnicalNotes, response.TicketDescription.TechnicalNotes)
	}
	combined.Worklog.EnvironmentVariables = uniqueEnvironmentNames(combined.Worklog.EnvironmentVariables)
	return AIResult{Context: value, Response: combined}, nil
}
func joinText(a, b string) string {
	if b == "" || a == b {
		return a
	}
	if a == "" {
		return b
	}
	return a + "\n\n" + b
}
