package cli

import (
	"fmt"
	"strings"

	application "github.com/taupikpirdian/wlog/application/summary"
)

func writeSummaryBullets(b *strings.Builder, items []string) {
	seen := map[string]bool{}
	for _, item := range items {
		for _, line := range strings.Split(item, "\n") {
			line = strings.TrimSpace(safeText(line))
			line = strings.TrimPrefix(line, "- ")
			if line == "" || seen[line] {
				continue
			}
			seen[line] = true
			fmt.Fprintf(b, "- %s\n", line)
		}
	}
	if len(seen) == 0 {
		b.WriteString("-\n")
	}
}
func formatAISummary(result application.AIResult) string {
	var b strings.Builder
	email := strings.TrimSpace(safeText(result.Context.Summary.Email))
	if email == "" {
		email = "-"
	}
	fmt.Fprintf(&b, "Time:\n%s\n\nDetail:\n", dailyDuration(result.Context.Summary.Day.Seconds))
	writeSummaryBullets(&b, result.Response.Worklog.Details)
	b.WriteString("\nHasil:\n")
	writeSummaryBullets(&b, result.Response.Worklog.Results)
	if len(result.Response.Worklog.EnvironmentVariables) > 0 {
		label := "Env Baru"
		if result.Context.OutputLanguage == application.LanguageEnglish {
			label = "New Environment Variables"
		}
		fmt.Fprintf(&b, "\n%s:\n", label)
		writeSummaryBullets(&b, result.Response.Worklog.EnvironmentVariables)
	}
	fmt.Fprintf(&b, "\nDev By:\n%s\n\n", email)
	b.WriteString(formatTicketDescription(result.Response.TicketDescription))
	return b.String()
}

func formatTicketDescription(description application.TicketDescription) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Background\n%s\n\n### Problem / Requirement\n%s\n\n### Scope\n", safeText(description.Background), safeText(description.ProblemRequirement))
	writeSummaryBullets(&b, description.Scope)
	fmt.Fprintf(&b, "\n### Expected Result\n%s\n", safeText(description.ExpectedResult))
	if strings.TrimSpace(description.TechnicalNotes) != "" {
		fmt.Fprintf(&b, "\n### Technical Notes\n%s\n", safeText(description.TechnicalNotes))
	}
	return b.String()
}
