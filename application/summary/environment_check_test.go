package summary_test

import (
	"context"
	application "github.com/taupikpirdian/wlog/application/summary"
	"github.com/taupikpirdian/wlog/domain/dashboard"
	environment "github.com/taupikpirdian/wlog/domain/environment"
	"github.com/taupikpirdian/wlog/domain/session"
	domain "github.com/taupikpirdian/wlog/domain/summary"
	"reflect"
	"testing"
	"time"
)

type recordingEnvironmentDetector struct{ ranges []environment.Range }

func (d *recordingEnvironmentDetector) Check(_ context.Context, ranges []environment.Range) environment.Changes {
	d.ranges = ranges
	return environment.Changes{Status: "checked", NewVariables: []string{}}
}

func TestEnvironmentCheckSelectedDateAndSameRecordedRanges(t *testing.T) {
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	previous := date.AddDate(0, 0, -1)
	earlierEnd := previous.Add(time.Hour)
	missing := "/missing"
	value := application.TicketAIContext{Summary: application.Result{Day: domain.Day{Date: date}}, AllTicketWorklogs: dashboard.Snapshot{
		Sessions:   []session.Session{{Repository: &missing, StartedAt: previous, EndedAt: &earlierEnd}},
		Activities: []dashboard.Activity{{Type: "GIT_COMMIT", Repository: "/recorded", Hash: "today", At: date.Add(time.Hour)}, {Type: "GIT_COMMIT", Repository: "/missing", Hash: "yesterday", At: previous}},
	}}
	d := &recordingEnvironmentDetector{}
	git := &gitFake{}
	got := application.NewEnvironmentCheck(git, d)(context.Background(), value)
	if got.Status != "checked" || !reflect.DeepEqual(git.calls, []string{"/recorded:today"}) || !reflect.DeepEqual(d.ranges, []environment.Range{{Repository: "/recorded", Start: "todayparent", End: "today"}}) {
		t.Fatalf("got=%+v calls=%v ranges=%v", got, git.calls, d.ranges)
	}
}

func TestEnvironmentUnavailableDependenciesDegrade(t *testing.T) {
	if got := application.NewEnvironmentCheck(nil, nil)(context.Background(), application.TicketAIContext{}); got.Status != "failed" {
		t.Fatalf("got=%+v", got)
	}
}

func TestEnvironmentCapturedCommitsIgnoreMissingSessionRepository(t *testing.T) {
	date := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	ended := date.Add(2 * time.Hour)
	uncaptured := "/uncaptured-session-repository"
	value := application.TicketAIContext{Summary: application.Result{Day: domain.Day{Date: date}}, AllTicketWorklogs: dashboard.Snapshot{
		Sessions:   []session.Session{{StartedAt: date, EndedAt: &ended}, {Repository: &uncaptured, StartedAt: date, EndedAt: &ended}},
		Activities: []dashboard.Activity{{Type: "GIT_COMMIT", Repository: "/recorded", Hash: "today", At: date.Add(time.Hour)}},
	}}
	d := &recordingEnvironmentDetector{}
	git := &gitFake{}
	got := application.NewEnvironmentCheck(git, d)(context.Background(), value)
	if got.Status != "checked" || !reflect.DeepEqual(d.ranges, []environment.Range{{Repository: "/recorded", Start: "todayparent", End: "today"}}) {
		t.Fatalf("got=%+v ranges=%v", got, d.ranges)
	}
}
