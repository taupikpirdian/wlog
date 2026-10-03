package aiagent

import (
	"encoding/json"
	"path/filepath"

	application "github.com/taupikpirdian/wlog/application/summary"
)

func openCodePermissions(request application.AIRequest) string {
	bash := map[string]string{"*": "deny", "git diff *": "allow", "git show *": "allow", "git log *": "allow", "git cat-file *": "allow"}
	permissions := map[string]any{"*": "deny", "read": "allow", "glob": "allow", "grep": "allow", "bash": bash}
	if request.Skill.Loaded && request.Skill.Native {
		permissions["skill"] = map[string]string{"*": "deny", "ticket-generator": "allow"}
	}
	if request.Context.TicketOnly {
		for _, operation := range []string{"diff", "show", "log", "cat-file"} {
			bash["git -C * "+operation+" *"] = "allow"
		}
		external := map[string]string{"*": "deny"}
		allow := func(path string) {
			external[filepath.ToSlash(path)] = "allow"
			external[filepath.ToSlash(path)+"/**"] = "allow"
		}
		for _, repo := range request.Context.Repositories {
			allow(repo.Path)
		}
		if request.Skill.Loaded && request.Skill.Native {
			allow(filepath.Dir(request.Skill.Path))
		}
		permissions["external_directory"] = external
	}
	// JSON keys are ordered, so catch-all deny rules precede specific allowances.
	body, _ := json.Marshal(map[string]any{"agent": map[string]any{"plan": map[string]any{"permission": permissions}}})
	return string(body)
}
