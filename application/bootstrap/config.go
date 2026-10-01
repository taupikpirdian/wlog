package bootstrap

// Config is the application configuration needed during startup.
type Config struct {
	DataDirectory string
	ConfigFile    string
	DatabasePath  string

	Ticket TicketConfig
	Git    GitConfig
	AI     AIConfig
}

type TicketConfig struct {
	Pattern string `mapstructure:"pattern"`
}

type GitConfig struct {
	CaptureChangedFiles bool `mapstructure:"capture_changed_files"`
	CaptureDiffStat     bool `mapstructure:"capture_diff_stat"`
	CaptureFullDiff     bool `mapstructure:"capture_full_diff"`
}

type AIConfig struct {
	Enabled     bool `mapstructure:"enabled"`
	IncludeDiff bool `mapstructure:"include_diff"`
}
