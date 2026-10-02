package summary_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
)

func responseWithEnvironment(names []string) string {
	encoded, _ := json.Marshal(names)
	return strings.Replace(validResponse, `"results":[]`, `"results":[],"environment_variables":`+string(encoded), 1)
}

func TestNewEnvironmentVariablesRequireSelectedDateCommitEvidence(t *testing.T) {
	factory := &fakeFactory{agent: &fakeAgent{body: responseWithEnvironment([]string{
		"NEW_ENV", "NEW_ENV", "SECOND_ENV", "EXISTING_ENV", "CONTEXT_ENV", "REMOVED_ENV", "ANOTHER_DATE", "GHOST_ENV", "NAME", "INVENTED_ENV",
	})}}
	value := application.TicketAIContext{Repositories: []application.RepositoryAIContext{
		{Path: "/repoA", Changes: []application.CodeChange{
			{SelectedDate: true, Diff: "diff --git a/.env.example b/.env.example\n--- a/.env.example\n+++ b/GHOST_ENV\n@@ -1,2 +1,5 @@\n+NEW_ENV=secret-fixture-value\n-EXISTING_ENV=old\n+EXISTING_ENV=new\n CONTEXT_ENV=existing\n+use(CONTEXT_ENV)\n-REMOVED_ENV=old\n+NAME_LONG=placeholder\n"},
			{SelectedDate: false, Diff: "+ANOTHER_DATE=placeholder\n"},
		}},
		{Path: "/repoB", Changes: []application.CodeChange{{SelectedDate: true, Diff: "+config := os.Getenv(\"NEW_ENV\")\n+- name: SECOND_ENV\n"}}},
	}}
	result, err := application.GenerateAI(context.Background(), value, bootstrap.AIConfig{}, factory, false, "/unused")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"NEW_ENV", "SECOND_ENV"}; !reflect.DeepEqual(result.Response.Worklog.EnvironmentVariables, want) {
		t.Fatalf("names=%v want=%v", result.Response.Worklog.EnvironmentVariables, want)
	}
	if factory.agent.requests[0].WorkingDirectory != "/repoA" || factory.agent.requests[1].WorkingDirectory != "/repoB" {
		t.Fatal("environment analysis lost recorded repository scope")
	}
}

func TestWorklogsOnlyCannotReportNewEnvironmentVariables(t *testing.T) {
	factory := &fakeFactory{agent: &fakeAgent{body: responseWithEnvironment([]string{"ENV_FROM_NOTES"})}}
	result, err := application.GenerateAI(context.Background(), application.TicketAIContext{}, bootstrap.AIConfig{}, factory, true, "/fallback")
	if err != nil || len(result.Response.Worklog.EnvironmentVariables) != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestEnvironmentNamesResponseValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		names []string
		valid bool
	}{
		{"names only", []string{"API_TOKEN", "NEW_ENV", "_lowercase"}, true},
		{"empty list", []string{}, true},
		{"assignment", []string{"API_TOKEN=private-fixture-value"}, false},
		{"empty name", []string{""}, false},
		{"description", []string{"NEW_ENV - used for authentication"}, false},
		{"terminal controls", []string{"NEW_ENV\x1b[0m"}, false},
		{"invalid identifier", []string{"1INVALID"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response, err := application.ParseAIResponse([]byte(responseWithEnvironment(tc.names)))
			if (err == nil) != tc.valid {
				t.Fatalf("response=%+v error=%v", response, err)
			}
			if err != nil && strings.Contains(err.Error(), "private-fixture-value") {
				t.Fatal("invalid environment value leaked into validation error")
			}
		})
	}
	if _, err := application.ParseAIResponse([]byte(validResponse)); err != nil {
		t.Fatalf("legacy custom response rejected: %v", err)
	}
}

func TestEnvironmentInstructionsAndStructuredSchema(t *testing.T) {
	prompt, err := application.BuildAIPrompt(application.AIRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"NEW ENVIRONMENT VARIABLES", "worklog.environment_variables", "recorded start/parent revision", "Names only", "changed values/defaults", "Return [] when no new variables are supported"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("missing instruction %q", required)
		}
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(application.ResponseSchema), &schema); err != nil {
		t.Fatal(err)
	}
	properties := schema["properties"].(map[string]any)["worklog"].(map[string]any)["properties"].(map[string]any)
	if properties["environment_variables"] == nil {
		t.Fatal("native structured schema lacks environment variables")
	}
}
