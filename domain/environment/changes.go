package environment

import "regexp"

var variableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidName(name string) bool { return variableName.MatchString(name) }

// Changes contains names only. Secret values never belong in this model.
type Changes struct {
	Status              string   `json:"status"`
	NewVariables        []string `json:"new_variables"`
	MissingFromTemplate []string `json:"missing_from_template"`
	CIOnlyVariables     []string `json:"ci_only_variables,omitempty"`
}

type References struct {
	Names      []string
	CIOnly     []string
	Incomplete bool
	Imports    []Import
	Resources  map[string][]string
}

type Import struct{ Resource, Prefix string }

type Range struct{ Repository, Start, End string }
