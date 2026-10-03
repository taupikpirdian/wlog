package cli

import (
	environment "github.com/taupikpirdian/wlog/domain/environment"
	"strings"
)

func formatEnvironmentChanges(changes environment.Changes) string {
	var b strings.Builder
	b.WriteString("\n### Environment Changes\n\n")
	if changes.Status != "checked" {
		b.WriteString("Environment variable check could not be completed.\n")
	}
	if len(changes.NewVariables) > 0 {
		b.WriteString("New environment variables:\n")
		writeSummaryBullets(&b, changes.NewVariables)
	} else if changes.Status == "checked" {
		b.WriteString("No new environment variables detected.\n")
	}
	if len(changes.MissingFromTemplate) > 0 {
		b.WriteString("\nMissing from environment template:\n")
		writeSummaryBullets(&b, changes.MissingFromTemplate)
	}
	if len(changes.CIOnlyVariables) > 0 {
		b.WriteString("\nNew CI-only environment variables:\n")
		writeSummaryBullets(&b, changes.CIOnlyVariables)
	}
	return b.String()
}
