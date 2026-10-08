package cli

import (
	environment "github.com/taupikpirdian/wlog/domain/environment"
	"strconv"
	"strings"
)

func formatEnvironmentChanges(changes environment.Changes) string {
	var b strings.Builder
	b.WriteString("\nEnvironment Changes\n\n")
	if changes.Status != "checked" {
		b.WriteString("Environment variable check could not be completed.\n")
	}
	if len(changes.NewVariables) > 0 {
		b.WriteString("New environment variables:\n")
		writeSummaryBullets(&b, changes.NewVariables)
		writeEnvironmentExamples(&b, changes.NewVariables, changes.Examples)
	} else if changes.Status == "checked" && len(changes.RemovedVariables) == 0 && len(changes.ModifiedVariables) == 0 {
		b.WriteString("No new environment variables detected.\n")
	}
	if len(changes.ModifiedVariables) > 0 {
		b.WriteString("\nChanged environment example/default values:\n")
		writeSummaryBullets(&b, changes.ModifiedVariables)
		b.WriteString("\nBefore:\n")
		writeEnvironmentExamples(&b, changes.ModifiedVariables, changes.PreviousExamples)
		b.WriteString("\nAfter:\n")
		writeEnvironmentExamples(&b, changes.ModifiedVariables, changes.Examples)
	}
	if len(changes.RemovedVariables) > 0 {
		b.WriteString("\nRemoved environment variables:\n")
		writeSummaryBullets(&b, changes.RemovedVariables)
		writeEnvironmentExamples(&b, changes.RemovedVariables, changes.RemovedExamples)
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

func writeEnvironmentExamples(b *strings.Builder, names []string, examples map[string]string) {
	b.WriteString("\nExample values (template or code default; placeholder if unavailable):\n```dotenv\n")
	for _, name := range names {
		value, ok := examples[name]
		if !ok {
			value = "<example-value>"
		}
		upper := strings.ToUpper(name)
		if strings.Contains(upper, "SECRET") || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "PRIVATE_KEY") {
			value = "<redacted>"
		}
		b.WriteString(name + "=" + strconv.Quote(value) + "\n")
	}
	b.WriteString("```\n")
}
