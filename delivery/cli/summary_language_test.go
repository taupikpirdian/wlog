package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	application "github.com/taupikpirdian/wlog/application/summary"
)

func TestLanguageSelection(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		want        application.OutputLanguage
		wantError   bool
	}{
		{"default", "\n", application.LanguageIndonesian, false},
		{"Indonesian", "1\n", application.LanguageIndonesian, false},
		{"English", "2\n", application.LanguageEnglish, false},
		{"retry invalid input", "3\nEN\n", application.LanguageEnglish, false},
		{"cancel", "q\n", "", true},
		{"closed input", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prompts bytes.Buffer
			got, err := selectOutputLanguage(bufio.NewReader(strings.NewReader(tc.input)), &prompts)
			if got != tc.want || (err != nil) != tc.wantError {
				t.Fatalf("language=%q error=%v", got, err)
			}
			if !strings.Contains(prompts.String(), "Bahasa Indonesia") || !strings.Contains(prompts.String(), "English") {
				t.Fatalf("prompts=%q", prompts.String())
			}
		})
	}
}
