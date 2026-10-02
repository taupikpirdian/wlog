package configfile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/taupikpirdian/wlog/application/bootstrap"
)

const (
	dataDirectoryName = ".worklog"
	configFileName    = "config.yaml"
	defaultTicketRule = `[A-Z][A-Z0-9]+-[0-9]+`
)

type Loader struct{ homeDirectory func() (string, error) }

func NewLoader() *Loader { return &Loader{} }

func (l *Loader) LoadOrCreate(ctx context.Context) (bootstrap.Config, error) {
	if err := ctx.Err(); err != nil {
		return bootstrap.Config{}, err
	}

	resolveHome := l.homeDirectory
	if resolveHome == nil {
		resolveHome = os.UserHomeDir
	}
	home, err := resolveHome()
	if err != nil {
		return bootstrap.Config{}, fmt.Errorf("resolve home directory: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return bootstrap.Config{}, errors.New("resolve home directory: home directory is empty")
	}

	dataDir := filepath.Join(home, dataDirectoryName)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return bootstrap.Config{}, fmt.Errorf("create data directory %q: %w", dataDir, err)
	}
	if err := os.Chmod(dataDir, 0o700); err != nil {
		return bootstrap.Config{}, fmt.Errorf("secure data directory %q: %w", dataDir, err)
	}

	configPath := filepath.Join(dataDir, configFileName)
	settings, err := readOrCreateConfig(configPath)
	if err != nil {
		return bootstrap.Config{}, err
	}

	var values struct {
		Database struct {
			Path string `mapstructure:"path"`
		} `mapstructure:"database"`
		Ticket bootstrap.TicketConfig `mapstructure:"ticket"`
		Git    bootstrap.GitConfig    `mapstructure:"git"`
		AI     bootstrap.AIConfig     `mapstructure:"ai"`
	}
	if err := settings.Unmarshal(&values); err != nil {
		return bootstrap.Config{}, fmt.Errorf("decode config %q: %w", configPath, err)
	}

	if _, err := regexp.Compile(values.Ticket.Pattern); err != nil {
		return bootstrap.Config{}, fmt.Errorf("validate ticket.pattern in %q: %w", configPath, err)
	}

	databasePath := expandHome(values.Database.Path, home)
	if strings.TrimSpace(databasePath) == "" {
		return bootstrap.Config{}, fmt.Errorf("validate database.path in %q: path is empty", configPath)
	}
	if !filepath.IsAbs(databasePath) {
		databasePath, err = filepath.Abs(databasePath)
		if err != nil {
			return bootstrap.Config{}, fmt.Errorf("resolve database.path in %q: %w", configPath, err)
		}
	}

	return bootstrap.Config{
		DataDirectory: dataDir,
		ConfigFile:    configPath,
		DatabasePath:  filepath.Clean(databasePath),
		Ticket:        values.Ticket,
		Git:           values.Git,
		AI:            values.AI,
	}, nil
}

func readOrCreateConfig(path string) (*viper.Viper, error) {
	settings := viper.New()
	settings.SetConfigFile(path)
	settings.SetConfigType("yaml")
	setDefaults(settings)

	if _, err := os.Stat(path); err == nil {
		if err := settings.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config %q: %w", path, err)
		}
		return settings, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect config %q: %w", path, err)
	}

	defaults := map[string]any{
		"database": map[string]any{"path": "~/.worklog/worklog.db"},
		"ticket":   map[string]any{"pattern": defaultTicketRule},
		"git": map[string]any{
			"capture_changed_files": true,
			"capture_diff_stat":     true,
			"capture_full_diff":     false,
		},
		"ai": map[string]any{"enabled": true, "include_diff": false},
	}
	body, err := yaml.Marshal(defaults)
	if err != nil {
		return nil, fmt.Errorf("encode default config: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			if err := settings.ReadInConfig(); err != nil {
				return nil, fmt.Errorf("read config %q: %w", path, err)
			}
			return settings, nil
		}
		return nil, fmt.Errorf("create config %q: %w", path, err)
	}
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write config %q: %w", path, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("sync config %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close config %q: %w", path, err)
	}
	if err := settings.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read created config %q: %w", path, err)
	}
	return settings, nil
}

func setDefaults(settings *viper.Viper) {
	settings.SetDefault("database.path", "~/.worklog/worklog.db")
	settings.SetDefault("ticket.pattern", defaultTicketRule)
	settings.SetDefault("git.capture_changed_files", true)
	settings.SetDefault("git.capture_diff_stat", true)
	settings.SetDefault("git.capture_full_diff", false)
	settings.SetDefault("ai.enabled", true)
	settings.SetDefault("ai.include_diff", false)
}

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		return filepath.Join(home, strings.TrimPrefix(path, "~"+string(filepath.Separator)))
	}
	return path
}
