package cli

import (
	application "github.com/taupikpirdian/wlog/application/summary"
	environment "github.com/taupikpirdian/wlog/domain/environment"
	"strings"
	"testing"
)

func TestEnvironmentSectionAlwaysUsesApplicationResult(t *testing.T) {
	for _, tc := range []struct {
		changes environment.Changes
		want    string
	}{
		{environment.Changes{Status: "checked"}, "No new environment variables detected."},
		{environment.Changes{Status: "checked", NewVariables: []string{"API_URL", "CLIENT_ID"}}, "New environment variables:\n- API_URL\n- CLIENT_ID"},
		{environment.Changes{Status: "failed"}, "Environment variable check could not be completed."},
		{environment.Changes{Status: "incomplete", NewVariables: []string{"API_URL"}}, "Environment variable check could not be completed."},
	} {
		r := application.Result{EnvironmentChanges: tc.changes}
		ai := application.AIResult{Context: application.TicketAIContext{Summary: r}, Response: application.AIResponse{Worklog: application.WorklogText{EnvironmentVariables: []string{"INVENTED_BY_AI"}}}}
		for _, text := range []string{formatSummary(r), formatAISummary(ai)} {
			if strings.Count(text, "Environment Changes") != 1 || !strings.Contains(text, tc.want) || strings.Contains(text, "INVENTED_BY_AI") {
				t.Fatalf("text=%q", text)
			}
			if tc.changes.Status != "checked" && strings.Contains(text, "No new environment") {
				t.Fatal("incomplete check reported success")
			}
		}
	}
}

func TestEnvironmentExamplesAndRemovedVariables(t *testing.T) {
	changes := environment.Changes{Status: "checked", NewVariables: []string{"API_URL", "API_TOKEN"}, RemovedVariables: []string{"OLD_URL"}, Examples: map[string]string{"API_URL": "https://example.com", "API_TOKEN": "private-value"}, RemovedExamples: map[string]string{"OLD_URL": "https://old.example.com"}}
	text := formatEnvironmentChanges(changes)
	for _, expected := range []string{"New environment variables:\n- API_URL\n- API_TOKEN", `API_URL="https://example.com"`, `API_TOKEN="<redacted>"`, "Removed environment variables:\n- OLD_URL", `OLD_URL="https://old.example.com"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %s", expected, text)
		}
	}
	if strings.Contains(text, "private-value") || strings.Contains(text, "could not be completed") {
		t.Fatalf("unexpected output: %s", text)
	}
}
