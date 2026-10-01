package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type configLoaderFunc func(context.Context) (Config, error)

func (f configLoaderFunc) LoadOrCreate(ctx context.Context) (Config, error) { return f(ctx) }

type storageInitializerFunc func(context.Context, string) error

func (f storageInitializerFunc) Initialize(ctx context.Context, path string) error {
	return f(ctx, path)
}

func TestInitializeOrchestration(t *testing.T) {
	ctx := context.Background()
	failure := errors.New("dependency failed")
	for _, tc := range []struct {
		name                string
		configErr, storeErr error
		wantCalls           int
		errorStage          string
	}{
		{"success", nil, nil, 1, ""}, {"config fails", failure, nil, 0, "initialize configuration"}, {"storage fails", nil, failure, 1, "initialize database"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{DataDirectory: "/fixture", ConfigFile: "/fixture/config.yaml", DatabasePath: "/fixture/db.sqlite"}
			calls := 0
			loader := configLoaderFunc(func(got context.Context) (Config, error) {
				if got != ctx {
					t.Fatal("context changed")
				}
				return cfg, tc.configErr
			})
			store := storageInitializerFunc(func(got context.Context, path string) error {
				calls++
				if got != ctx || path != cfg.DatabasePath {
					t.Fatalf("storage context/path: %v %q", got, path)
				}
				return tc.storeErr
			})
			result, err := NewService(loader, store).Initialize(ctx)
			if calls != tc.wantCalls {
				t.Fatalf("storage calls=%d", calls)
			}
			if tc.errorStage != "" {
				if !errors.Is(err, failure) || !strings.Contains(err.Error(), tc.errorStage) || result != (Result{}) {
					t.Fatalf("result=%+v error=%v", result, err)
				}
				return
			}
			if err != nil || result != (Result{DataDirectory: cfg.DataDirectory, ConfigFile: cfg.ConfigFile, DatabasePath: cfg.DatabasePath}) {
				t.Fatalf("result=%+v error=%v", result, err)
			}
		})
	}
}
