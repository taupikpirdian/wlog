package summary

import (
	"regexp"
	"sort"
)

var environmentNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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
