package summary

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Environment values are irrelevant to the names-only detector result. Keep
// source-code diffs intact, but omit config-file contents from the AI prompt.
// This does not alter the immutable Git input used by the detector.
func namesOnlyEnvironmentEvidence(repo *RepositoryAIContext) *RepositoryAIContext {
	if repo == nil {
		return nil
	}
	copy := *repo
	copy.Changes = append([]CodeChange(nil), repo.Changes...)
	for i := range copy.Changes {
		var out strings.Builder
		sensitive := false
		for _, line := range strings.Split(copy.Changes[i].Diff, "\n") {
			if strings.HasPrefix(line, "diff --git ") {
				fields := strings.Fields(line)
				path := ""
				if len(fields) >= 4 {
					path = strings.TrimPrefix(strings.Trim(fields[3], "\""), "b/")
				}
				sensitive = environmentConfigurationPath(path)
				out.WriteString(line + "\n")
				if sensitive {
					out.WriteString("Configuration values omitted; use authoritative environment_changes names.\n")
				}
			} else if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
				path := line[4:]
				if unquoted, err := strconv.Unquote(path); err == nil {
					path = unquoted
				}
				if environmentConfigurationPath(path) {
					sensitive = true
				}
				out.WriteString(line + "\n")
			} else if !sensitive {
				out.WriteString(line + "\n")
			}
		}
		copy.Changes[i].Diff = out.String()
	}
	return &copy
}

func environmentConfigurationPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := filepath.Ext(base)
	return base == ".env" || strings.HasPrefix(base, ".env.") || ext == ".env" || base == "dockerfile" || strings.HasPrefix(base, "dockerfile.") || strings.HasSuffix(base, ".dockerfile") || ext == ".yaml" || ext == ".yml"
}
