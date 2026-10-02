//go:build windows

package aiagent

import "os/exec"

func configureProcessCancellation(cmd *exec.Cmd) { /* CommandContext terminates the Windows provider process. */
}
