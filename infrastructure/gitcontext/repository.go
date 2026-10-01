package gitcontext

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// Current returns no repository when Git is unavailable or the cwd is outside Git.
func Current(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
