package activity

import (
	"context"
	"errors"
	"time"

	"github.com/taupikpirdian/wlog/domain/activity"
	"github.com/taupikpirdian/wlog/domain/session"
	"github.com/taupikpirdian/wlog/domain/ticket"
)

type CaptureOptions struct{ ChangedFiles, DiffStat, FullDiff bool }
type GitReader interface {
	Read(context.Context, CaptureOptions) (activity.Commit, error)
}
type AttributionResolver func(*session.Session) activity.Attribution
type CaptureStore interface {
	Capture(context.Context, activity.Commit, AttributionResolver) (activity.CaptureResult, error)
}

type CaptureService struct {
	reader  GitReader
	store   CaptureStore
	pattern ticket.KeyPattern
	options CaptureOptions
	now     func() time.Time
}

func NewCaptureService(reader GitReader, store CaptureStore, pattern ticket.KeyPattern, options CaptureOptions, now func() time.Time) *CaptureService {
	return &CaptureService{reader: reader, store: store, pattern: pattern, options: options, now: now}
}

func (s *CaptureService) Capture(ctx context.Context) (activity.CaptureResult, error) {
	value, err := s.reader.Read(ctx, s.options)
	if err != nil {
		return activity.CaptureResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return activity.CaptureResult{}, err
	}
	if value.Repository == "" || value.Hash == "" {
		return activity.CaptureResult{}, errors.New("Git capture requires repository and commit identity")
	}
	value.Metadata.Version, value.Metadata.Source = 1, "git-capture"
	value.Metadata.CapturedAt = s.now().UTC()
	value.Metadata.TimeSource = "commit"
	if value.CreatedAt.IsZero() {
		value.CreatedAt = value.Metadata.CapturedAt
		value.Metadata.TimeSource = "capture_fallback"
	}
	value.CreatedAt = value.CreatedAt.UTC()
	if s.options.FullDiff {
		value.Metadata.Warnings = append(value.Metadata.Warnings, "full_diff_unsupported")
	}
	messageKey, branchKey := "", ""
	if value.Message != nil {
		messageKey, _ = s.pattern.Extract(*value.Message)
	}
	if value.Branch != nil {
		branchKey, _ = s.pattern.Extract(*value.Branch)
	}
	return s.store.Capture(ctx, value, func(active *session.Session) activity.Attribution {
		return activity.ResolveCommit(messageKey, branchKey, active)
	})
}
