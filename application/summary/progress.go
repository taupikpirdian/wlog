package summary

type ProgressEventType string

const (
	ProgressInfo    ProgressEventType = "info"
	ProgressTool    ProgressEventType = "tool"
	ProgressFile    ProgressEventType = "file"
	ProgressWarning ProgressEventType = "warning"
	ProgressStatus  ProgressEventType = "status"
	ProgressSuccess ProgressEventType = "success"
)

// An empty Provider identifies application actions; a provider name identifies
// observable events reported by that provider, never model reasoning.
type ProgressEvent struct {
	Provider       string
	RepositoryPath string
	Type           ProgressEventType
	Message        string
	FilePath       string
	ToolName       string
}
type ProgressHandler func(ProgressEvent)
type AICapabilities struct{ SupportsEventStream, SupportsStderrProgress bool }

func emitProgress(handler ProgressHandler, event ProgressEvent) {
	if handler != nil {
		handler(event)
	}
}
