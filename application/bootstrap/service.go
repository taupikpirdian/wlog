package bootstrap

import (
	"context"
	"fmt"
)

// ConfigLoader loads or creates the user's configuration.
type ConfigLoader interface {
	LoadOrCreate(context.Context) (Config, error)
}

// StorageInitializer creates the local database and applies pending migrations.
type StorageInitializer interface {
	Initialize(context.Context, string) error
}

// Result describes the local paths prepared during startup.
type Result struct {
	DataDirectory string
	ConfigFile    string
	DatabasePath  string
}

// Service coordinates first-run configuration and storage initialization.
type Service struct {
	config ConfigLoader
	store  StorageInitializer
}

func NewService(config ConfigLoader, store StorageInitializer) *Service {
	return &Service{config: config, store: store}
}

func (s *Service) Initialize(ctx context.Context) (Result, error) {
	cfg, err := s.config.LoadOrCreate(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("initialize configuration: %w", err)
	}

	if err := s.store.Initialize(ctx, cfg.DatabasePath); err != nil {
		return Result{}, fmt.Errorf("initialize database %q: %w", cfg.DatabasePath, err)
	}

	return Result{
		DataDirectory: cfg.DataDirectory,
		ConfigFile:    cfg.ConfigFile,
		DatabasePath:  cfg.DatabasePath,
	}, nil
}
