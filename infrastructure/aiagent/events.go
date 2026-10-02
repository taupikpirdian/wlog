package aiagent

import (
	"encoding/json"
	"strings"

	application "github.com/taupikpirdian/wlog/application/summary"
)

type eventParser interface {
	Consume([]byte) error
	Result() ([]byte, error)
}
type eventSink struct {
	provider, repository string
	handler              application.ProgressHandler
}

func (s eventSink) emit(kind application.ProgressEventType, message, tool, path string) {
	if s.handler != nil {
		s.handler(application.ProgressEvent{Provider: s.provider, RepositoryPath: s.repository, Type: kind, Message: message, ToolName: tool, FilePath: path})
	}
}
func (s eventSink) tool(name string, input map[string]any) {
	if name == "StructuredOutput" {
		return
	}
	if strings.EqualFold(name, "skill") {
		value, _ := input["skill"].(string)
		if value == "" {
			value, _ = input["name"].(string)
		}
		if value != "" {
			s.emit(application.ProgressTool, "Loading skill "+value, name, "")
			return
		}
	}
	for _, key := range []string{"file_path", "path", "filePath"} {
		if path, ok := input[key].(string); ok && path != "" && strings.EqualFold(name, "read") {
			s.emit(application.ProgressFile, "", name, path)
			return
		}
	}
	message := "Invoking " + name
	if command, ok := input["command"].(string); ok {
		message = "Running " + command
	}
	s.emit(application.ProgressTool, message, name, "")
}

// Unknown or malformed progress events are ignored. Arbitrary raw event JSON
// can contain reasoning, results, tool output, or credentials and is not prose.
func decodeEvent(line []byte, value any) bool { return json.Unmarshal(line, value) == nil }
