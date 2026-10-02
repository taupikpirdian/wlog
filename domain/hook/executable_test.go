package hook

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecutableUpgradePreservesOriginalAndIsIdempotent(t *testing.T) {
	original := File{Exists: true, Regular: true, Executable: true, Mode: 0755, Content: []byte("#!/bin/sh\nexit 7\n")}
	legacy, err := PlanInstall(Snapshot{Target: original})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := Snapshot{Target: File{Exists: true, Regular: true, Executable: true, Mode: 0755, Content: legacy.Content}, Original: original}
	executable := "/applications/work log's/$wl`binary`"
	upgrade, err := PlanInstallWithExecutable(snapshot, executable)
	if err != nil || upgrade.Status != Updated || upgrade.PreserveOriginal || !upgrade.HasOriginal || upgrade.Executable != executable {
		t.Fatalf("plan=%+v err=%v", upgrade, err)
	}
	if strings.Contains(string(upgrade.Content), "\nwl git ") || !strings.Contains(string(upgrade.Content), "'\"'\"'") {
		t.Fatalf("unquoted executable: %s", upgrade.Content)
	}
	snapshot.Target.Content = upgrade.Content
	for _, path := range []string{executable, ""} {
		plan, err := PlanInstallWithExecutable(snapshot, path)
		if err != nil || plan.Status != AlreadyInstalled || !bytes.Equal(plan.Content, upgrade.Content) || plan.Executable != executable {
			t.Fatalf("rerun=%+v err=%v", plan, err)
		}
	}
	move, err := PlanInstallWithExecutable(snapshot, "/new/location/wl")
	if err != nil || move.Status != Updated || move.PreserveOriginal {
		t.Fatalf("relocation=%+v err=%v", move, err)
	}
	snapshot.Target.Content = append(snapshot.Target.Content, []byte("echo modified\n")...)
	if _, err := PlanInstallWithExecutable(snapshot, executable); err != ErrConflict {
		t.Fatalf("upgraded modified hook: %v", err)
	}
}

func TestExecutableRejectsInvalidPaths(t *testing.T) {
	for _, path := range []string{"wl", "relative/wl", "/path\ncommand", "/path\rcommand", "/path\x00"} {
		if _, err := PlanInstallWithExecutable(Snapshot{}, path); err == nil {
			t.Fatalf("accepted %q", path)
		}
	}
}
