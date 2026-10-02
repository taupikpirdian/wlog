package aiagent

import (
	"encoding/json"
	"fmt"
	"strings"

	application "github.com/taupikpirdian/wlog/application/summary"
)

type openCodeEventParser struct {
	sink      eventSink
	text      strings.Builder
	resultErr error
}

func (p *openCodeEventParser) Consume(line []byte) error {
	var event struct {
		Type  string          `json:"type"`
		Error json.RawMessage `json:"error"`
		Part  struct {
			Text  string `json:"text"`
			Tool  string `json:"tool"`
			State struct {
				Status string         `json:"status"`
				Input  map[string]any `json:"input"`
			} `json:"state"`
		} `json:"part"`
	}
	if !decodeEvent(line, &event) {
		return nil
	}
	switch event.Type {
	case "step_start":
		p.sink.emit(application.ProgressStatus, "Step started", "", "")
	case "step_finish":
		p.sink.emit(application.ProgressStatus, "Step completed", "", "")
	case "tool_use":
		if event.Part.State.Status == "running" || event.Part.State.Status == "completed" {
			p.sink.tool(event.Part.Tool, event.Part.State.Input)
		}
	case "text":
		if p.text.Len()+len(event.Part.Text) > maxResponseBytes {
			return fmt.Errorf("OpenCode final result exceeds size limit")
		}
		p.text.WriteString(event.Part.Text)
	case "error":
		p.resultErr = fmt.Errorf("OpenCode generation failed: %s", event.Error)
		p.sink.emit(application.ProgressWarning, "Generation failed", "", "")
	}
	// Reasoning and unknown event types are deliberately ignored.
	return nil
}
func (p *openCodeEventParser) Result() ([]byte, error) {
	if p.resultErr != nil {
		return nil, p.resultErr
	}
	return []byte(p.text.String()), nil
}
