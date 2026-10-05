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
	fmt.Fprintf(&b, "Time:\n%s\n\nGenerated for Logs:\nDetail:\n", summaryDuration(result.Context.Summary.Day))
	writeSummaryBullets(&b, result.Response.Worklog.Details)
	b.WriteString("\nResult:\n")
	writeSummaryBullets(&b, result.Response.Worklog.Results)
	fmt.Fprintf(&b, "\nDev By:\n%s\n\n", email)
	b.WriteString("Generated for Details Ticket:\n\n")
	b.WriteString(result.Ticket.Content)
	b.WriteString(formatEnvironmentChanges(result.Context.Summary.EnvironmentChanges))
	return b.String()
}
