package cli

import (
	"fmt"
	"strings"

	application "github.com/taupikpirdian/wlog/application/summary"
)

func formatSummary(result application.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Time:\n%s\n\nDetail:\n", dailyDuration(result.Day.Seconds))
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
	fmt.Fprintf(&b, "\nHasil:\n-\n\nDev By:\n%s\n", email)
	return b.String()
}
