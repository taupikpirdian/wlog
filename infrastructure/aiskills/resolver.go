// Package aiskills resolves installed ticket-generation instructions before
// an AI invocation, without changing a provider's skill installation.
package aiskills

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	application "github.com/taupikpirdian/wlog/application/summary"
)

const maxSkillBytes = 256 * 1024

type Resolver struct {
	home     func() (string, error)
	readFile func(string) ([]byte, error)
}

func NewResolver() *Resolver { return &Resolver{home: os.UserHomeDir, readFile: readSkill} }

func readSkill(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("skill instructions must be a regular file")
	}
	body, err := io.ReadAll(io.LimitReader(file, maxSkillBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxSkillBytes {
		return nil, fmt.Errorf("skill instructions exceed %d bytes", maxSkillBytes)
	}
	if strings.TrimSpace(string(body)) == "" {
		return nil, fmt.Errorf("skill instructions are empty")
	}
	return body, nil
}

func (r *Resolver) Resolve(ctx context.Context, provider, directory string, progress application.ProgressHandler) (application.TicketSkill, error) {
	emit := func(kind application.ProgressEventType, message string) {
		if progress != nil {
			progress(application.ProgressEvent{Type: kind, Message: message})
		}
	}
	if err := ctx.Err(); err != nil {
		return application.TicketSkill{}, err
	}
	emit(application.ProgressStatus, "Checking installed AI skills")
	home, err := r.home()
	if err != nil {
		emit(application.ProgressWarning, "Failed to resolve installed AI skills")
		emit(application.ProgressInfo, "Using built-in wlog ticket-generation instructions")
		return application.TicketSkill{}, nil
	}
	paths, native := skillPaths(provider, directory, home)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return application.TicketSkill{}, err
		}
		if _, err := os.Lstat(path); err != nil {
			continue
		}
		emit(application.ProgressInfo, "Found skill: ticket-generator")
		emit(application.ProgressInfo, "Reading ticket-generator skill instructions")
		body, err := r.readFile(path)
		if err != nil {
			emit(application.ProgressWarning, "Failed to read ticket-generator skill instructions")
			emit(application.ProgressInfo, "Falling back to built-in wlog ticket-generation instructions")
			return application.TicketSkill{}, nil
		}
		if err := ctx.Err(); err != nil {
			return application.TicketSkill{}, err
		}
		value := application.TicketSkill{Name: "ticket-generator", Path: path, Loaded: true, Native: native}
		switch provider {
		case "codex":
			value.Invocation = "$ticket-generator"
		case "claude":
			value.Invocation = `Skill({skill: "ticket-generator"})`
		case "opencode":
			value.Invocation = `skill({name: "ticket-generator"})`
		}
		if native {
			// wlog verified the complete file, but does not claim the agent has
			// read it. The provider must load it using its native mechanism.
			emit(application.ProgressStatus, "ticket-generator instructions verified by wlog")
			emit(application.ProgressInfo, "ticket-generator will be requested during AI generation")
		} else {
			value.Instructions = string(body)
			emit(application.ProgressStatus, "ticket-generator skill loaded by wlog")
		}
		return value, nil
	}
	emit(application.ProgressWarning, "ticket-generator skill not found")
	emit(application.ProgressInfo, "Using built-in wlog ticket-generation instructions")
	return application.TicketSkill{}, nil
}

func skillPaths(provider, directory, home string) ([]string, bool) {
	var roots []string
	switch provider {
	case "codex":
		roots = []string{filepath.Join(directory, ".agents", "skills"), filepath.Join(home, ".agents", "skills"), filepath.Join(home, ".codex", "skills"), "/etc/codex/skills"}
	case "claude":
		roots = []string{filepath.Join(home, ".claude", "skills"), filepath.Join(directory, ".claude", "skills")}
	case "opencode":
		roots = []string{filepath.Join(directory, ".opencode", "skills"), filepath.Join(directory, ".claude", "skills"), filepath.Join(directory, ".agents", "skills"), filepath.Join(home, ".config", "opencode", "skills"), filepath.Join(home, ".claude", "skills"), filepath.Join(home, ".agents", "skills")}
	default:
		roots = []string{filepath.Join(directory, ".agents", "skills"), filepath.Join(home, ".agents", "skills"), filepath.Join(home, ".codex", "skills")}
	}
	paths := make([]string, 0, len(roots))
	for _, root := range roots {
		paths = append(paths, filepath.Join(root, "ticket-generator", "SKILL.md"))
	}
	return paths, provider != "custom"
}
