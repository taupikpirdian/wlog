package summary

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// This schema deliberately excludes all factual metadata controlled by wlog.
const ResponseSchema = `{"type":"object","additionalProperties":false,"required":["worklog","ticket_description"],"properties":{"worklog":{"type":"object","additionalProperties":false,"required":["details","results"],"properties":{"details":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"results":{"type":"array","items":{"type":"string","minLength":1}}}},"ticket_description":{"type":"object","additionalProperties":false,"required":["background","problem_requirement","scope","expected_result","technical_notes"],"properties":{"background":{"type":"string","minLength":1},"problem_requirement":{"type":"string","minLength":1},"scope":{"type":"array","minItems":1,"items":{"type":"string","minLength":1}},"expected_result":{"type":"string","minLength":1},"technical_notes":{"type":"string"}}}}}`

func ParseAIResponse(body []byte) (*AIResponse, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var response AIResponse
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("AI response invalid: expected the summary JSON contract: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("AI response invalid: extra text after JSON")
	}
	if err := ValidateAIResponse(&response); err != nil {
		return nil, err
	}
	return &response, nil
}
func ValidateAIResponse(r *AIResponse) error {
	if r == nil {
		return errors.New("AI response invalid: no response")
	}
	for name, text := range map[string]string{"ticket_description.background": r.TicketDescription.Background, "ticket_description.problem_requirement": r.TicketDescription.ProblemRequirement, "ticket_description.expected_result": r.TicketDescription.ExpectedResult} {
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("AI response invalid: %s is required", name)
		}
	}
	for name, items := range map[string][]string{"worklog.details": r.Worklog.Details, "ticket_description.scope": r.TicketDescription.Scope} {
		if len(items) == 0 {
			return fmt.Errorf("AI response invalid: %s must not be empty", name)
		}
	}
	if r.Worklog.Results == nil {
		return errors.New("AI response invalid: worklog.results is required (use [] when no result is supported)")
	}
	// Omission remains compatible with existing custom agents. Names, when
	// present, must be identifiers rather than assignments or secret values.
	for _, name := range r.Worklog.EnvironmentVariables {
		if !environmentNamePattern.MatchString(name) {
			return errors.New("AI response invalid: worklog.environment_variables must contain environment variable names only, without values")
		}
	}
	for name, items := range map[string][]string{"worklog.details": r.Worklog.Details, "worklog.results": r.Worklog.Results, "ticket_description.scope": r.TicketDescription.Scope} {
		for _, text := range items {
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("AI response invalid: %s contains an empty entry", name)
			}
		}
	}
	return nil
}
