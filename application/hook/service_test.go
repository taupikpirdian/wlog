package hook

import (
	"context"
	"errors"
	"testing"

	domain "github.com/taupikpirdian/wlog/domain/hook"
)

type inspectorFunc func(context.Context) (domain.Location, error)

func (f inspectorFunc) Inspect(ctx context.Context) (domain.Location, error) { return f(ctx) }

type storeFunc func(context.Context, domain.Location, Planner) (domain.Plan, error)

func (f storeFunc) Apply(ctx context.Context, location domain.Location, plan Planner) (domain.Plan, error) {
	return f(ctx, location, plan)
}

func TestInstallServiceOutcomes(t *testing.T) {
	failure := errors.New("dependency failed")
	for _, tc := range []struct {
		name                          string
		before, after                 bool
		inspectErr, storeErr, wantErr error
		custom                        bool
		status                        string
		reads, writes                 int
	}{
		{"cancel before inspection", true, false, nil, nil, context.Canceled, false, "", 0, 0},
		{"inspect fails", false, false, failure, nil, failure, false, "", 1, 0},
		{"custom path", false, false, nil, nil, domain.ErrUnsupported, true, "", 1, 0},
		{"cancel after inspection", false, true, nil, nil, context.Canceled, false, "", 1, 0},
		{"store fails", false, false, nil, failure, failure, false, "", 1, 1},
		{"installed", false, false, nil, nil, nil, false, domain.Installed, 1, 1},
		{"already installed", false, false, nil, nil, nil, false, domain.AlreadyInstalled, 1, 1},
		{"repaired", false, false, nil, nil, nil, false, domain.Repaired, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.before {
				cancel()
			}
			reads, writes := 0, 0
			location := domain.Location{Root: "/repo", HookPath: "/repo/.git/hooks/post-commit", Custom: tc.custom}
			inspect := inspectorFunc(func(got context.Context) (domain.Location, error) {
				reads++
				if got != ctx {
					t.Fatal("context changed")
				}
				if tc.after {
					cancel()
				}
				return location, tc.inspectErr
			})
			store := storeFunc(func(got context.Context, loc domain.Location, plan Planner) (domain.Plan, error) {
				writes++
				if got != ctx || loc != location {
					t.Fatalf("location=%+v", loc)
				}
				if _, err := plan(domain.Snapshot{}); err != nil {
					t.Fatal(err)
				}
				return domain.Plan{Status: tc.status, HasOriginal: true}, tc.storeErr
			})
			result, err := NewService(inspect, store).Install(ctx)
			if !errors.Is(err, tc.wantErr) || reads != tc.reads || writes != tc.writes {
				t.Fatalf("error=%v reads=%d writes=%d", err, reads, writes)
			}
			if err == nil && (result.Status != tc.status || result.Location != location || !result.HasOriginal) {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}
