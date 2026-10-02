package summary

import (
	"regexp"
	"strings"
	"unicode"
)

var credentialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization\s*[:=]\s*(?:bearer\s+|basic\s+)?)[^\s,;"']+`),
	regexp.MustCompile(`(?i)((?:api[-_]?key|access[-_]?token|token|password|secret|credential)\s*[:= ]\s*["']?)[^\s"'&,;]+`),
	regexp.MustCompile(`\b(?:sk|ghp|github_pat)-?[A-Za-z0-9_]{16,}\b`),
}

func SecretValues(environment []string) []string {
	var values []string
	for _, item := range environment {
		key, value, ok := strings.Cut(item, "=")
		if !ok || len(value) < 4 {
			continue
		}
		key = strings.ToUpper(key)
		for _, marker := range []string{"API_KEY", "TOKEN", "PASSWORD", "SECRET", "CREDENTIAL", "AUTHORIZATION"} {
			if strings.Contains(key, marker) {
				values = append(values, value)
				break
			}
		}
	}
	return values
}
func RedactOutput(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	for i, pattern := range credentialPatterns {
		if i < 2 {
			value = pattern.ReplaceAllString(value, "${1}[REDACTED]")
		} else {
			value = pattern.ReplaceAllString(value, "[REDACTED]")
		}
	}
	return value
}

// Diagnostic output is also observable-only: exclude reasoning and arbitrary
// provider transcripts, and retain a few error/warning lines rather than dumps.
func SafeDiagnostics(value string, secrets []string) string {
	var lines []string
	for _, line := range strings.Split(value, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "reasoning") || strings.Contains(lower, "thinking") || strings.Contains(lower, "chain_of_thought") || strings.HasPrefix(strings.TrimSpace(line), "{") {
			continue
		}
		if !strings.Contains(lower, "error") && !strings.Contains(lower, "failed") && !strings.Contains(lower, "warning") && !strings.Contains(lower, "denied") {
			continue
		}
		line = RedactOutput(line, secrets)
		line = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, line)
		if len(line) > 500 {
			line = line[:500] + "…"
		}
		lines = append(lines, line)
	}
	if len(lines) > 5 {
		lines = lines[len(lines)-5:]
	}
	if len(lines) == 0 {
		return "no safe diagnostic message; check provider authentication and CLI configuration"
	}
	return strings.Join(lines, "; ")
}
