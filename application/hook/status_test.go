package hook

import (
	"context"
	"errors"
	"testing"

	domain "github.com/taupikpirdian/wlog/domain/hook"
)

type snapshotReaderFunc func(context.Context, domain.Location) (domain.Snapshot, error)

func (f snapshotReaderFunc) Read(ctx context.Context, location domain.Location) (domain.Snapshot, error) {
	return f(ctx, location)
}

func TestStatusDoesNotReadCustomHookLocations(t *testing.T) {
	service := NewStatusService(inspectorFunc(func(context.Context) (domain.Location, error) {
		return domain.Location{Root: "/repo", Custom: true}, nil
	}), snapshotReaderFunc(func(context.Context, domain.Location) (domain.Snapshot, error) {
		t.Fatal("read custom hooks")
		return domain.Snapshot{}, nil
	}))
	result, err := service.Status(context.Background())
	if err != nil || result.Status != CustomHooks || result.Location.Root != "/repo" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestStatusDoesNotClaimMissingHookOnReadFailure(t *testing.T) {
	failure := errors.New("permission denied")
	location := domain.Location{Root: "/repo", HookPath: "/repo/.git/hooks/post-commit"}
	service := NewStatusService(inspectorFunc(func(context.Context) (domain.Location, error) {
		return location, nil
	}), snapshotReaderFunc(func(context.Context, domain.Location) (domain.Snapshot, error) {
		return domain.Snapshot{}, failure
	}))
	result, err := service.Status(context.Background())
	if !errors.Is(err, failure) || result.Status != "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestStatusStopsWhenInspectionCancelsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service := NewStatusService(inspectorFunc(func(context.Context) (domain.Location, error) {
		cancel()
		return domain.Location{Root: "/repo", Custom: true}, nil
	}), snapshotReaderFunc(func(context.Context, domain.Location) (domain.Snapshot, error) {
		t.Fatal("read after cancellation")
		return domain.Snapshot{}, nil
	}))
	if _, err := service.Status(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
