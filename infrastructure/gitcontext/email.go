package gitcontext

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type EmailReader struct{ directory string }

func NewEmailReader(directory string) *EmailReader { return &EmailReader{directory: directory} }

// Email uses Git's effective config in cwd, then explicitly tries global config.
// Missing Git or unset email yields an empty identity; cancellation is an error.
func (r *EmailReader) Email(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for _, args := range [][]string{{"config", "user.email"}, {"config", "--global", "user.email"}} {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = r.directory
		output, err := cmd.Output()
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			var exit *exec.ExitError
			if errors.Is(err, exec.ErrNotFound) {
				return "", nil
			}
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				continue
			}
			return "", fmt.Errorf("read Git developer email: %w", err)
		}
		if email := strings.TrimSpace(string(output)); email != "" {
			return email, nil
		}
	}
	return "", nil
}
