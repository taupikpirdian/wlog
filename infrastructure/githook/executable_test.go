//go:build unix

package githook

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	domain "github.com/taupikpirdian/wlog/domain/hook"
)

func TestPinnedExecutableHandlesShellCharactersAndIgnoresPATH(t *testing.T) {
	location := hookLocation(t)
	bin := filepath.Join(canonicalTemp(t), "space's $dollar `backtick` $(command)")
	executable := filepath.Join(bin, "wl")
	spy := filepath.Join(canonicalTemp(t), "calls")
	writeFixture(t, executable, "#!/bin/sh\nprintf '%s|%s\\n' \"$PWD\" \"$1\" >> \"$WLOG_TEST_SPY\"\n", 0755)
	plan, err := NewStore().Apply(context.Background(), location, func(snapshot domain.Snapshot) (domain.Plan, error) {
		return domain.PlanInstallWithExecutable(snapshot, executable)
	})
	if err != nil || plan.Executable != executable {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	cmd := exec.Command(location.HookPath)
	cmd.Dir = location.Root
	cmd.Env = append(os.Environ(), "PATH=/nonexistent", "WLOG_TEST_SPY="+spy)
	if out, err := cmd.CombinedOutput(); err != nil || len(out) != 0 {
		t.Fatalf("out=%s err=%v", out, err)
	}
	assertFile(t, spy, location.Root+"|git\n", 0644)
}
