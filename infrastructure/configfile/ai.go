package configfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/taupikpirdian/wlog/application/bootstrap"
	"github.com/taupikpirdian/wlog/application/summary"
	"gopkg.in/yaml.v3"
)

// SaveAI updates only ai in the existing config, preserving other YAML fields.
func (l *Loader) SaveAI(ctx context.Context, config bootstrap.AIConfig) error {
	if err := summary.ValidateAIConfig(config); err != nil {
		return err
	}
	current, err := l.LoadOrCreate(ctx)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(current.ConfigFile)
	if err != nil {
		return err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(body, &document); err != nil {
		return err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("config must be a YAML mapping: %s", current.ConfigFile)
	}
	root := document.Content[0]
	var ai *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "ai" {
			ai = root.Content[i+1]
		}
	}
	if ai == nil {
		ai = &yaml.Node{Kind: yaml.MappingNode}
		root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "ai"}, ai)
	}
	if ai.Kind != yaml.MappingNode {
		return fmt.Errorf("ai configuration must be a YAML mapping")
	}
	updates := map[string]any{"provider": config.Provider, "providers": config.Providers, "enabled": config.Enabled}
	for _, key := range []string{"provider", "providers", "enabled"} {
		var node yaml.Node
		if err := node.Encode(updates[key]); err != nil {
			return err
		}
		found := false
		for i := 0; i < len(ai.Content); i += 2 {
			if ai.Content[i].Value == key {
				ai.Content[i+1] = &node
				found = true
				break
			}
		}
		if !found {
			ai.Content = append(ai.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &node)
		}
	}
	body, err = yaml.Marshal(&document)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(current.ConfigFile), ".config-ai-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(body); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(file.Name(), current.ConfigFile)
}
