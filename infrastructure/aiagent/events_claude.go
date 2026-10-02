package aiagent

import (
	"encoding/json"
	"fmt"

	application "github.com/taupikpirdian/wlog/application/summary"
)

type claudeEventParser struct {
	sink      eventSink
	result    []byte
	resultErr error
}

func (p *claudeEventParser) Consume(line []byte) error {
	var event struct {
		Type       string          `json:"type"`
		Subtype    string          `json:"subtype"`
		IsError    bool            `json:"is_error"`
		Structured json.RawMessage `json:"structured_output"`
		Result     string          `json:"result"`
		Message    struct {
			Content []struct {
				Type  string         `json:"type"`
				Name  string         `json:"name"`
				Input map[string]any `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if !decodeEvent(line, &event) {
		return nil
	}
	switch event.Type {
	case "system":
		if event.Subtype == "init" {
			p.sink.emit(application.ProgressStatus, "Session initialized", "", "")
		}
	case "assistant":
		for _, block := range event.Message.Content {
			if block.Type == "tool_use" {
				p.sink.tool(block.Name, block.Input)
			}
		}
	case "result":
		if event.IsError {
			p.resultErr = fmt.Errorf("Claude generation failed: %s", event.Result)
			p.sink.emit(application.ProgressWarning, "Generation failed", "", "")
			return nil
		}
		if len(event.Structured) > 0 {
			p.result = append([]byte(nil), event.Structured...)
		} else {
			p.result = []byte(event.Result)
		}
		p.sink.emit(application.ProgressStatus, "Generation completed", "", "")
	}
	// Thinking blocks, text blocks, tool results, and stream deltas are excluded.
	return nil
}
func (p *claudeEventParser) Result() ([]byte, error) {
	if p.resultErr != nil {
		return nil, p.resultErr
	}
	if len(p.result) == 0 {
		return nil, fmt.Errorf("Claude produced no final structured result")
	}
	return p.result, nil
}
