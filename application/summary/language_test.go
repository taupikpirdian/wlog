package summary_test

import (
	"context"
	"strings"
	"testing"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	application "github.com/taupikpirdian/wlog/application/summary"
)

func TestAIPromptUsesSelectedLanguage(t *testing.T) {
	for _, ticketOnly := range []bool{false, true} {
		for _, tc := range []struct {
			language   application.OutputLanguage
			code, name string
		}{
			{"", "id", "Bahasa Indonesia (Indonesian)"},
			{application.LanguageIndonesian, "id", "Bahasa Indonesia (Indonesian)"},
			{application.LanguageEnglish, "en", "English"},
		} {
			request := application.AIRequest{
				Context: application.TicketAIContext{TicketOnly: ticketOnly, OutputLanguage: tc.language},
				Skill:   application.TicketSkill{Loaded: true, Instructions: "Write prose in the skill's default language."},
			}
			prompt, err := application.BuildAIPrompt(request)
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{"output_language: " + tc.code, "prose in " + tc.name, "overrides any skill language default"} {
				if !strings.Contains(prompt, required) {
					t.Fatalf("ticketOnly=%v language=%q: missing %q", ticketOnly, tc.language, required)
				}
			}
			if ticketOnly && (!strings.Contains(prompt, "Follow the skill's language rules for headings") || strings.Contains(prompt, "Preserve JSON field names, Jira headings")) {
				t.Fatal("ticket language must follow skill formatting rules")
			}
			if tc.language == application.LanguageEnglish && (strings.Contains(prompt, "output_language: id") || strings.Contains(prompt, "Use Indonesian for the generated prose")) {
				t.Fatal("English prompt contains conflicting Indonesian instructions")
			}
		}
	}
}

func TestInvalidLanguageRejectedBeforeAgentExecution(t *testing.T) {
	value := application.TicketAIContext{OutputLanguage: "fr"}
	if _, err := application.BuildAIPrompt(application.AIRequest{Context: value}); err == nil {
		t.Fatal("unsupported prompt language accepted")
	}
	factory := &fakeFactory{agent: &fakeAgent{}}
	if _, err := application.GenerateAI(context.Background(), value, bootstrap.AIConfig{}, factory, true, "/config"); err == nil || !strings.Contains(err.Error(), "choose id or en") {
		t.Fatalf("error=%v", err)
	}
	if factory.calls != 0 {
		t.Fatal("invalid language started an AI process")
	}
}
