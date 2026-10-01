package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/taupikpirdian/wlog/domain/activity"
	"github.com/taupikpirdian/wlog/domain/ticket"
)

type captureReaderFunc func(context.Context, CaptureOptions) (activity.Commit, error)

func (f captureReaderFunc) Read(ctx context.Context, options CaptureOptions) (activity.Commit, error) {
	return f(ctx, options)
}

type captureStoreFunc func(context.Context, activity.Commit, AttributionResolver) (activity.CaptureResult, error)

func (f captureStoreFunc) Capture(ctx context.Context, value activity.Commit, resolve AttributionResolver) (activity.CaptureResult, error) {
	return f(ctx, value, resolve)
}

func TestCaptureConfigurationFallbackAndCustomPattern(t *testing.T) {
	at := time.Date(2026, 10, 1, 9, 0, 0, 123, time.FixedZone("test", 7*3600))
	pattern, _ := ticket.NewKeyPattern(`TASK-[0-9]+`)
	options := CaptureOptions{ChangedFiles: true, FullDiff: true}
	reader := captureReaderFunc(func(_ context.Context, got CaptureOptions) (activity.Commit, error) {
		if got != options {
			t.Fatalf("options=%+v", got)
		}
		message := "subject\nTASK-10 first TASK-20 second"
		branch := "TASK-30"
		return activity.Commit{Repository: "/repo", Hash: "hash", Message: &message, Branch: &branch}, nil
	})
	store := captureStoreFunc(func(_ context.Context, value activity.Commit, resolve AttributionResolver) (activity.CaptureResult, error) {
		if !value.CreatedAt.Equal(at) || value.CreatedAt.Location() != time.UTC || value.Metadata.TimeSource != "capture_fallback" || len(value.Metadata.Warnings) != 1 || value.Metadata.Warnings[0] != "full_diff_unsupported" {
			t.Fatalf("commit=%+v", value)
		}
		attr := resolve(nil)
		if attr.TicketKey != "TASK-10" || attr.Source != "MESSAGE" {
			t.Fatalf("attribution=%+v", attr)
		}
		return activity.CaptureResult{}, nil
	})
	if _, err := NewCaptureService(reader, store, pattern, options, func() time.Time { return at }).Capture(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureIdentityFailureDoesNotWrite(t *testing.T) {
	pattern, _ := ticket.NewKeyPattern(`OOT-[0-9]+`)
	store := captureStoreFunc(func(context.Context, activity.Commit, AttributionResolver) (activity.CaptureResult, error) {
		t.Fatal("unexpected write")
		return activity.CaptureResult{}, nil
	})
	for _, cause := range []error{nil, errors.New("Git unavailable")} {
		reader := captureReaderFunc(func(context.Context, CaptureOptions) (activity.Commit, error) { return activity.Commit{}, cause })
		if _, err := NewCaptureService(reader, store, pattern, CaptureOptions{}, time.Now).Capture(context.Background()); err == nil {
			t.Fatal("accepted invalid identity")
		}
	}
}

func TestCaptureCancelledContextAndCommitTimestamp(t *testing.T) {
	pattern, _ := ticket.NewKeyPattern(`OOT-[0-9]+`)
	committed := time.Date(2026, 10, 1, 8, 0, 0, 0, time.FixedZone("test", 7*3600))
	reader := captureReaderFunc(func(context.Context, CaptureOptions) (activity.Commit, error) {
		return activity.Commit{Repository: "/repo", Hash: "hash", CreatedAt: committed}, nil
	})
	writes := 0
	store := captureStoreFunc(func(_ context.Context, value activity.Commit, resolve AttributionResolver) (activity.CaptureResult, error) {
		writes++
		if !value.CreatedAt.Equal(committed) || value.CreatedAt.Location() != time.UTC || value.Metadata.TimeSource != "commit" {
			t.Fatalf("timestamp=%+v", value)
		}
		if attr := resolve(nil); attr.Source != "UNASSIGNED" || attr.TicketKey != "" {
			t.Fatalf("nil sources=%+v", attr)
		}
		return activity.CaptureResult{}, nil
	})
	service := NewCaptureService(reader, store, pattern, CaptureOptions{}, time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Capture(ctx); !errors.Is(err, context.Canceled) || writes != 0 {
		t.Fatalf("cancelled=%v writes=%d", err, writes)
	}
	if _, err := service.Capture(context.Background()); err != nil || writes != 1 {
		t.Fatalf("capture=%v writes=%d", err, writes)
	}
}
