package aiagent

import (
	"encoding/json"
	"fmt"

	application "github.com/taupikpirdian/wlog/application/summary"
)

// Custom JSONL: info/status/warning/tool/file observable events and a final
// {"type":"result","data":{...summary contract...}} on stdout.
type customEventParser struct {
	sink   eventSink
	result []byte
}

func (p *customEventParser) Consume(line []byte) error {
	var event struct {
		Type, Message, Name, Command, Path string
		Data                               json.RawMessage
	}
	if !decodeEvent(line, &event) {
		return nil
	}
	switch event.Type {
	case "info":
		p.sink.emit(application.ProgressInfo, event.Message, "", "")
	case "status":
		p.sink.emit(application.ProgressStatus, event.Message, "", "")
	case "warning":
		p.sink.emit(application.ProgressWarning, event.Message, "", "")
	case "tool":
		p.sink.tool(event.Name, map[string]any{"command": event.Command})
	case "file":
		p.sink.emit(application.ProgressFile, "", "read", event.Path)
	case "result":
		p.result = append([]byte(nil), event.Data...)
	}
	return nil
}
func (p *customEventParser) Result() ([]byte, error) {
	if len(p.result) == 0 {
		return nil, fmt.Errorf("Custom JSONL agent produced no result event")
	}
	return p.result, nil
}
