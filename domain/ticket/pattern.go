package ticket

import (
	"fmt"
	"regexp"
)

// KeyPattern validates complete ticket keys and extracts keys from free text.
type KeyPattern struct {
	pattern *regexp.Regexp
}

func NewKeyPattern(expression string) (KeyPattern, error) {
	if expression == "" {
		return KeyPattern{}, fmt.Errorf("%w: pattern is empty", ErrInvalidKey)
	}
	compiled, err := regexp.Compile(expression)
	if err != nil {
		return KeyPattern{}, fmt.Errorf("compile ticket key pattern: %w", err)
	}
	return KeyPattern{pattern: compiled}, nil
}

// Extract returns the first key-shaped substring in message order.
func (p KeyPattern) Extract(message string) (string, bool) {
	if p.pattern == nil {
		return "", false
	}
	key := p.pattern.FindString(message)
	return key, key != ""
}

// Validate requires the configured pattern to match the entire key.
func (p KeyPattern) Validate(key string) error {
	if p.pattern == nil || key == "" || p.pattern.FindString(key) != key {
		return fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	return nil
}
