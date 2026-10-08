package summary

import (
	"context"
	"github.com/taupikpirdian/wlog/domain/dashboard"
	environment "github.com/taupikpirdian/wlog/domain/environment"
)

type EnvironmentDetector interface {
	Check(context.Context, []environment.Range) environment.Changes
}

// Uses the same immutable, captured commit ranges as summary generation.
// Environment checks do not require an AI provider or user AI consent.
func NewEnvironmentCheck(git GitService, detector EnvironmentDetector) EnvironmentCheck {
	return func(ctx context.Context, value TicketAIContext) environment.Changes {
		if git == nil || detector == nil {
			return environment.Changes{Status: "failed", NewVariables: []string{}, MissingFromTemplate: []string{}}
		}
		// Other dates belong to the ticket description, not this check. Missing
		// repositories in unrelated history must not mark today's check incomplete.
		start, end := value.Summary.Day.Date, value.Summary.Day.Date.AddDate(0, 0, 1)
		selected := dashboard.Snapshot{}
		for _, activity := range value.AllTicketWorklogs.Activities {
			if !activity.At.Before(start) && activity.At.Before(end) {
				selected.Activities = append(selected.Activities, activity)
			}
		}
		// Only captured commits define environment evidence. Sessions may have
		// no repository (manual work) or no captured changes; neither invalidates
		// the immutable ranges recorded on commit activities.
		value.AllTicketWorklogs = selected
		collected, err := CollectCode(ctx, value, git)
		if err != nil {
			return environment.Changes{Status: "failed", NewVariables: []string{}, MissingFromTemplate: []string{}}
		}
		var ranges []environment.Range
		for _, repo := range collected.Repositories {
			for _, change := range repo.Changes {
				if change.SelectedDate {
					ranges = append(ranges, environment.Range{Repository: repo.Path, Start: change.Start, End: change.End})
				}
			}
		}
		result := detector.Check(ctx, ranges)
		if len(collected.Warnings) > 0 && result.Status == "checked" {
			result.Status = "incomplete"
		}
		return result
	}
}
