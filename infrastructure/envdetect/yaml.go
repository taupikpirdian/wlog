package envdetect

import (
	"io"
	"path/filepath"
	"strings"

	application "github.com/taupikpirdian/wlog/application/environment"
	domain "github.com/taupikpirdian/wlog/domain/environment"
	"gopkg.in/yaml.v3"
)

type YAMLExtractor struct{}

func (YAMLExtractor) Supports(path string) bool {
	e := filepath.Ext(path)
	return application.SupportedPath(path) && (e == ".yaml" || e == ".yml")
}

func (YAMLExtractor) Extract(path, text string) (domain.References, error) {
	set := map[string]bool{}
	b := strings.ToLower(filepath.Base(path))
	configuration := strings.Contains(b, "compose") || strings.Contains(b, "deployment") || strings.HasPrefix(b, "values.")
	refs := domain.References{Resources: map[string][]string{}}
	decoder := yaml.NewDecoder(strings.NewReader(text))
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return domain.References{}, err
		}
		// Work with nodes, never unmarshalling secret values into an output model.
		namespace := "default"
		if len(doc.Content) > 0 {
			if metadata := field(doc.Content[0], "metadata"); metadata != nil {
				if ns := field(metadata, "namespace"); ns != nil {
					namespace = ns.Value
				}
			}
		}
		var walk func(*yaml.Node)
		walk = func(n *yaml.Node) {
			if n.Kind == yaml.ScalarNode && configuration {
				for _, match := range interpolation.FindAllStringSubmatch(n.Value, -1) {
					set[match[1]] = true
				}
			}
			if n.Kind == yaml.MappingNode {
				for i := 0; i+1 < len(n.Content); i += 2 {
					key, value := n.Content[i].Value, n.Content[i+1]
					if key == "envFrom" && value.Kind == yaml.SequenceNode {
						for _, item := range value.Content {
							prefix := ""
							if node := field(item, "prefix"); node != nil {
								prefix = node.Value
							}
							for _, source := range []struct{ key, kind string }{{"configMapRef", "ConfigMap"}, {"secretRef", "Secret"}} {
								if reference := field(item, source.key); reference != nil {
									if name := field(reference, "name"); name != nil {
										refs.Imports = append(refs.Imports, domain.Import{Resource: source.kind + "/" + namespace + "/" + name.Value, Prefix: prefix})
									}
								}
							}
						}
					}
					if key == "env" || key == "environment" {
						extractEnvNode(value, set)
					}
					walk(value)
				}
			} else {
				for _, child := range n.Content {
					walk(child)
				}
			}
		}
		walk(&doc)
		if len(doc.Content) > 0 {
			root := doc.Content[0]
			kind := field(root, "kind")
			if kind != nil && (kind.Value == "ConfigMap" || kind.Value == "Secret") {
				resource := ""
				if metadata := field(root, "metadata"); metadata != nil {
					if name := field(metadata, "name"); name != nil {
						resource = kind.Value + "/" + namespace + "/" + name.Value
						refs.Resources[resource] = []string{}
					}
				}
				for _, key := range []string{"data", "stringData"} {
					if data := field(root, key); data != nil && data.Kind == yaml.MappingNode {
						for i := 0; i+1 < len(data.Content); i += 2 {
							addName(set, data.Content[i].Value)
							if resource != "" && namePattern.MatchString(data.Content[i].Value) {
								refs.Resources[resource] = append(refs.Resources[resource], data.Content[i].Value)
							}
						}
					}
				}
			}
		}
	}
	refs.Names = names(set)
	if strings.Contains(filepath.ToSlash(path), ".github/workflows/") || filepath.Base(path) == ".gitlab-ci.yml" {
		refs.CIOnly = refs.Names
		refs.Names = nil
	}
	return refs, nil
}

func addName(set map[string]bool, name string) {
	if namePattern.MatchString(name) {
		set[name] = true
	}
}

func field(n *yaml.Node, key string) *yaml.Node {
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

func extractEnvNode(n *yaml.Node, set map[string]bool) {
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			addName(set, n.Content[i].Value)
		}
		return
	}
	if n.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range n.Content {
		if item.Kind == yaml.ScalarNode {
			addName(set, strings.SplitN(item.Value, "=", 2)[0])
		}
		if name := field(item, "name"); name != nil {
			addName(set, name.Value)
		}
	}
	// envFrom resource names are not environment variable names. Actual keys
	// are extracted from available ConfigMap/Secret manifests; unresolved external
	// resources cannot be guessed statically.
}
