package summary_test

import (
	application "github.com/taupikpirdian/wlog/application/summary"
	environment "github.com/taupikpirdian/wlog/domain/environment"
	"strings"
	"testing"
)

func TestAIEnvironmentEvidenceNamesOnlyAndInputUnchanged(t *testing.T) {
	for _, path := range []string{".env.example", "secret.yaml", "folder with spaces/.env", `"b/folder\t/.env"`} {
		diff := "diff --git a/file b/file\n--- a/file\n+++ b/" + path + "\n@@ -0,0 +1 @@\n+API_TOKEN=private-test-value\ndiff --git a/app.ts b/app.ts\n--- a/app.ts\n+++ b/app.ts\n@@ -0,0 +1 @@\n+process.env.API_TOKEN\n"
		if strings.HasPrefix(path, "\"") {
			diff = strings.Replace(diff, "+++ b/"+path, "+++ "+path, 1)
		}
		repo := &application.RepositoryAIContext{Path: "/recorded", Changes: []application.CodeChange{{Diff: diff}}}
		request := application.AIRequest{Repository: repo, Context: application.TicketAIContext{Summary: application.Result{EnvironmentChanges: environment.Changes{Status: "checked", NewVariables: []string{"API_TOKEN"}}}}}
		prompt, err := application.BuildAIPrompt(request)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(prompt, "private-test-value") || !strings.Contains(prompt, "environment_changes") || !strings.Contains(prompt, "process.env.API_TOKEN") || repo.Changes[0].Diff != diff {
			t.Fatalf("values leaked or actual source/input altered: %s", prompt)
		}
	}
}
