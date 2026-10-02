package hook

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestInstallPlansAndPreservation(t *testing.T) {
	newPlan, err := PlanInstall(Snapshot{})
	if err != nil || newPlan.Status != Installed || newPlan.Mode != 0755 || newPlan.PreserveOriginal || newPlan.HasOriginal || !bytes.Contains(newPlan.Content, []byte("wl git >/dev/null 2>&1 || true")) {
		t.Fatalf("new=%+v error=%v", newPlan, err)
	}
	for _, executable := range []bool{true, false} {
		original := File{Exists: true, Regular: true, Content: []byte("#!/bin/sh\necho original\nexit 7\n"), Mode: 0640, Executable: executable}
		if executable {
			original.Mode = 0750
		}
		plan, err := PlanInstall(Snapshot{Target: original})
		if err != nil || !plan.PreserveOriginal || !plan.HasOriginal || plan.Mode != original.Mode|0100 || plan.Status != Installed {
			t.Fatalf("preserve=%+v error=%v", plan, err)
		}
		if !strings.Contains(string(plan.Content), "executable="+map[bool]string{true: "true", false: "false"}[executable]) {
			t.Fatalf("wrapper=%s", plan.Content)
		}
		managed := File{Exists: true, Regular: true, Content: plan.Content, Mode: plan.Mode, Executable: true}
		rerun, err := PlanInstall(Snapshot{Target: managed, Original: original})
		if err != nil || rerun.Status != AlreadyInstalled || rerun.PreserveOriginal || !rerun.HasOriginal {
			t.Fatalf("rerun=%+v error=%v", rerun, err)
		}
		managed.Executable = false
		managed.Mode = 0640
		repair, err := PlanInstall(Snapshot{Target: managed, Original: original})
		if err != nil || repair.Status != Repaired || repair.Mode != 0740 || !bytes.Equal(repair.Content, managed.Content) {
			t.Fatalf("repair=%+v error=%v", repair, err)
		}
	}
	managed := File{Exists: true, Regular: true, Content: newPlan.Content, Mode: 0755, Executable: true}
	if plan, err := PlanInstall(Snapshot{Target: managed}); err != nil || plan.Status != AlreadyInstalled || plan.HasOriginal {
		t.Fatalf("new rerun=%+v error=%v", plan, err)
	}
}

func TestInstallRejectsConflictsAndUnsafeFiles(t *testing.T) {
	original := File{Exists: true, Regular: true, Content: []byte("original"), Mode: 0755, Executable: true}
	plan, _ := PlanInstall(Snapshot{Target: original})
	for _, tc := range []struct {
		name     string
		snapshot Snapshot
		want     error
	}{
		{"target nonregular", Snapshot{Target: File{Exists: true}}, ErrUnsafe},
		{"backup nonregular", Snapshot{Original: File{Exists: true}}, ErrUnsafe},
		{"backup collision absent target", Snapshot{Original: original}, ErrConflict},
		{"backup collision unmanaged target", Snapshot{Target: original, Original: original}, ErrConflict},
		{"marker only", Snapshot{Target: File{Exists: true, Regular: true, Content: []byte("# wlog-managed-post-commit v2")}}, ErrConflict},
		{"missing backup", Snapshot{Target: File{Exists: true, Regular: true, Content: plan.Content}}, ErrConflict},
		{"changed backup", Snapshot{Target: File{Exists: true, Regular: true, Content: plan.Content}, Original: File{Exists: true, Regular: true, Content: []byte("changed"), Mode: 0755, Executable: true}}, ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := PlanInstall(tc.snapshot); !errors.Is(err, tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestLocationValidation(t *testing.T) {
	for _, tc := range []struct {
		location Location
		want     error
	}{
		{Location{}, ErrInvalidLocation},
		{Location{Root: "/repo", HookPath: "/repo/.git/hooks/post-commit", Custom: true}, ErrUnsupported},
		{Location{Root: "/repo", HookPath: "/repo/.git/hooks/post-commit"}, nil},
	} {
		if err := ValidateLocation(tc.location); !errors.Is(err, tc.want) {
			t.Fatalf("location=%+v error=%v", tc.location, err)
		}
	}
}
