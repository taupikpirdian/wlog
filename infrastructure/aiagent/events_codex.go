package aiagent

import application "github.com/taupikpirdian/wlog/application/summary"

type codexEventParser struct{ sink eventSink }

func (p *codexEventParser) Consume(line []byte) error {
	var event struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
		Item struct {
			Type, Command, Tool, Server string
			Status                      string `json:"status"`
		} `json:"item"`
	}
	if !decodeEvent(line, &event) {
		return nil
	}
	switch event.Type {
	case "thread.started":
		p.sink.emit(application.ProgressStatus, "Session started", "", "")
	case "turn.started":
		p.sink.emit(application.ProgressStatus, "Turn started", "", "")
	case "turn.completed":
		p.sink.emit(application.ProgressStatus, "Turn completed", "", "")
	case "error":
		p.sink.emit(application.ProgressWarning, event.Message, "", "")
	case "turn.failed":
		p.sink.emit(application.ProgressWarning, event.Error.Message, "", "")
	case "item.started":
		switch event.Item.Type {
		case "command_execution":
			p.sink.emit(application.ProgressTool, "Running "+event.Item.Command, "command_execution", "")
		case "mcp_tool_call":
			p.sink.emit(application.ProgressTool, "Invoking "+event.Item.Server+"/"+event.Item.Tool, event.Item.Tool, "")
		}
	}
	// In particular: never render reasoning or agent_message items.
	return nil
}
func (*codexEventParser) Result() ([]byte, error) { return nil, nil } // Separate output-last-message file.
