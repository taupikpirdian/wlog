package environment

import "regexp"

var variableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidName(name string) bool { return variableName.MatchString(name) }

// Example values stay local to CLI output and are excluded from AI evidence.
type Changes struct {
	Status              string            `json:"status"`
	NewVariables        []string          `json:"new_variables"`
	MissingFromTemplate []string          `json:"missing_from_template"`
	CIOnlyVariables     []string          `json:"ci_only_variables,omitempty"`
	RemovedVariables    []string          `json:"removed_variables,omitempty"`
	ModifiedVariables   []string          `json:"modified_variables,omitempty"`
	Examples            map[string]string `json:"-"`
	RemovedExamples     map[string]string `json:"-"`
	PreviousExamples    map[string]string `json:"-"`
}

type References struct {
	Names      []string
	CIOnly     []string
	Incomplete bool
	Imports    []Import
	Resources  map[string][]string
	Examples   map[string]string
}

type Import struct{ Resource, Prefix string }

type Range struct{ Repository, Start, End string }
