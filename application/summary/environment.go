package summary

import (
	"regexp"
	"sort"
	"strings"
)

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var identifierTokenPattern = regexp.MustCompile(`[A-Za-z0-9_]+`)

// The AI classifies environment/configuration changes. The application only
// accepts literal names supported by additions in captured selected-date diffs.
// This is a conservative evidence check, not proof of absence in the full parent
// tree; the prompt also asks the agent to inspect the recorded parent revision.
func environmentVariablesWithEvidence(names []string, repository RepositoryAIContext) []string {
	candidates := map[string]bool{}
	for _, name := range uniqueEnvironmentNames(names) {
		candidates[name] = true
	}
	if len(candidates) == 0 {
		return nil
	}
	supported := map[string]bool{}
	for _, change := range repository.Changes {
		if !change.SelectedDate {
			continue
		}
		added, existing := map[string]bool{}, map[string]bool{}
		// Scan each bounded diff once, independently of the number of AI names.
		for _, line := range strings.Split(change.Diff, "\n") {
			if len(line) == 0 || strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
				continue
			}
			if line[0] != '+' && line[0] != '-' && line[0] != ' ' {
				continue
			}
			for _, token := range identifierTokenPattern.FindAllString(line[1:], -1) {
				if !candidates[token] {
					continue
				}
				if line[0] == '+' {
					added[token] = true
				} else {
					// Changed values and additional uses of existing names are
					// not new environment variables.
					existing[token] = true
				}
			}
		}
		for name := range added {
			if !existing[name] {
				supported[name] = true
			}
		}
	}
	var verified []string
	for name := range supported {
		verified = append(verified, name)
	}
	sort.Strings(verified)
	return verified
}

func uniqueEnvironmentNames(names []string) []string {
	seen := map[string]bool{}
	for _, name := range names {
		if environmentNamePattern.MatchString(name) {
			seen[name] = true
		}
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
