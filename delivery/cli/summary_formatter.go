package cli

import (
	"fmt"
	"strings"

	application "github.com/taupikpirdian/wlog/application/summary"
	domain "github.com/taupikpirdian/wlog/domain/summary"
)

func summaryDuration(day domain.Day) string {
	duration := dailyDuration(day.Seconds)
	if day.CommitCount > 0 {
		label := "commits"
		if day.CommitCount == 1 {
			label = "commit"
		}
		duration += fmt.Sprintf(" (%d %s)", day.CommitCount, label)
	}
	return duration
}

func formatSummary(result application.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Time:\n%s\n\nGenerated for Logs:\nDetail:\n", summaryDuration(result.Day))
	seen := map[string]bool{}
	for _, detail := range result.Day.Details {
		detail = strings.TrimSpace(safeText(detail))
		if detail == "" || seen[detail] {
			continue
		}
		seen[detail] = true
		fmt.Fprintf(&b, "- %s\n", detail)
	}
	if len(seen) == 0 {
		b.WriteString("-\n")
	}
	email := strings.TrimSpace(safeText(result.Email))
	if email == "" {
		email = "-"
	}
	fmt.Fprintf(&b, "\nResult:\n-\n\nDev By:\n%s\n", email)
	b.WriteString(formatEnvironmentChanges(result.EnvironmentChanges))
	return b.String()
}
