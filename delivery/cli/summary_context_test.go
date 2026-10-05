package cli

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdditionalContextInput(t *testing.T) {
	for _, tc := range []struct {
		name, input, want, next string
		wantError               bool
	}{
		{name: "skip", input: "\ny\n", next: "y"},
		{name: "empty terminator", input: ".\ny\n", next: "y"},
		{name: "multiline with blank paragraphs", input: "Investigation\n\nHTTP 400 from callback\n.\ny\n", want: "Investigation\n\nHTTP 400 from callback", next: "y"},
		{name: "Windows newlines", input: "Finding\r\nMore context\r\n.\r\ny\r\n", want: "Finding\nMore context", next: "y"},
		{name: "EOF after pasted text", input: "Investigation", want: "Investigation"},
		{name: "EOF skips optional input"},
		{name: "cancel", input: "q\n", wantError: true},
		{name: "oversized", input: strings.Repeat("x", maxAdditionalContextBytes+1) + "\n.\n", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := bufio.NewReader(strings.NewReader(tc.input))
			var prompts bytes.Buffer
			got, err := readAdditionalContext(input, &prompts)
			if got != tc.want || (err != nil) != tc.wantError {
				t.Fatalf("context=%q error=%v", got, err)
			}
			if !strings.Contains(prompts.String(), "not saved to the database") {
				t.Fatalf("missing transient-context prompt: %q", prompts.String())
			}
			if tc.next != "" {
				next, err := readInputLine(input)
				if err != nil || next != tc.next {
					t.Fatalf("consumed next prompt input: value=%q error=%v", next, err)
				}
			}
		})
	}
}

func TestAdditionalContextFileInputPreservesLongLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "investigation report.md")
	content := "CRM returned 0011.\n\n" + strings.Repeat("captured-log-data", 1000) + "\n.\nCallback returned CB422.\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	for _, reference := range []string{"@" + path, "@\"" + path + "\""} {
		input := bufio.NewReader(strings.NewReader(reference + "\ny\n"))
		var prompts bytes.Buffer
		got, err := readAdditionalContext(input, &prompts)
		if err != nil || got != strings.TrimSpace(content) {
			t.Fatalf("file content lost: length=%d error=%v", len(got), err)
		}
		if next, err := readInputLine(input); err != nil || next != "y" {
			t.Fatalf("file input consumed next answer: %q error=%v", next, err)
		}
		stored, err := os.ReadFile(path)
		if err != nil || string(stored) != content {
			t.Fatal("context file was modified")
		}
	}
}

func TestAdditionalContextFileRejectsInvalidInputs(t *testing.T) {
	directory := t.TempDir()
	for _, tc := range []struct{ name, content, wantError string }{
		{"oversized", strings.Repeat("x", maxAdditionalContextBytes+1), "exceeds 64 KiB"},
		{"binary", "\x00\xFF", "UTF-8 text"},
	} {
		path := filepath.Join(directory, tc.name)
		if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readAdditionalContextFile(path); err == nil || !strings.Contains(err.Error(), tc.wantError) {
			t.Fatalf("%s error=%v", tc.name, err)
		}
	}
	for _, path := range []string{"", filepath.Join(directory, "missing"), directory} {
		if _, err := readAdditionalContextFile(path); err == nil {
			t.Fatalf("invalid file accepted: %q", path)
		}
	}
}
