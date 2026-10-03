package environment_test

import (
	"context"
	"encoding/json"
	"errors"
	application "github.com/taupikpirdian/wlog/application/environment"
	domain "github.com/taupikpirdian/wlog/domain/environment"
	"github.com/taupikpirdian/wlog/infrastructure/envdetect"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type revisions struct {
	trees  map[string]map[string]string
	broken string
}

type failingExtractor struct{}

func (failingExtractor) Supports(path string) bool { return path == "a.ts" }
func (failingExtractor) Extract(string, string) (domain.References, error) {
	return domain.References{}, errors.New("extractor failed")
}

func TestRegisteredExtractorFailureRetainsOtherEvidence(t *testing.T) {
	d := application.Detector{Git: revisions{trees: map[string]map[string]string{"end": {"a.ts": "process.env.API_URL"}}}, Extractors: append(envdetect.DefaultExtractors(), failingExtractor{})}
	got := d.Check(context.Background(), []domain.Range{{End: "end"}})
	if got.Status != "incomplete" || !reflect.DeepEqual(got.NewVariables, []string{"API_URL"}) {
		t.Fatalf("got=%+v", got)
	}
}

func (r revisions) Files(_ context.Context, _, rev string) ([]string, error) {
	if r.broken == rev {
		return nil, errors.New("unavailable")
	}
	var files []string
	for path := range r.trees[rev] {
		files = append(files, path)
	}
	sort.Strings(files)
	return files, nil
}
func (r revisions) ChangedFiles(_ context.Context, _, base, end string) ([]string, error) {
	var files []string
	for path, text := range r.trees[end] {
		if before, ok := r.trees[base][path]; !ok || before != text {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	return files, nil
}
func (r revisions) ReadFile(_ context.Context, _, rev, path string) (string, error) {
	return r.trees[rev][path], nil
}

func TestRevisionComparisonAndTemplates(t *testing.T) {
	for _, tc := range []struct {
		name          string
		base, end     map[string]string
		want, missing []string
	}{
		{"new duplicate", map[string]string{}, map[string]string{"a.ts": `process.env.API_URL; process.env.API_URL`}, []string{"API_URL"}, []string{}},
		{"refactor", map[string]string{"a.go": `url := os.Getenv("API_URL")`}, map[string]string{"a.go": `apiURL := os.Getenv("API_URL")`}, []string{}, []string{}},
		{"existing in unchanged file", map[string]string{"a.py": `os.getenv("API_URL")`}, map[string]string{"a.py": `os.getenv("API_URL")`, "b.ts": `process.env.API_URL`}, []string{}, []string{}},
		{"empty unsupported", map[string]string{}, map[string]string{"a.txt": "not env"}, []string{}, []string{}},
		{"dynamic", map[string]string{}, map[string]string{"a.ts": "process.env[key]"}, []string{}, []string{}},
		{"template present", map[string]string{}, map[string]string{"a.ts": "process.env.API_URL", ".env.example": "API_URL=secret-value"}, []string{"API_URL"}, []string{}},
		{"template missing", map[string]string{}, map[string]string{"a.ts": "process.env.API_URL", ".env.example": "# template"}, []string{"API_URL"}, []string{"API_URL"}},
		{"rename", map[string]string{"old.ts": "process.env.API_URL"}, map[string]string{"new.ts": "process.env.API_URL"}, []string{}, []string{}},
		{"envFrom local manifest", map[string]string{}, map[string]string{"deployment.yaml": "spec:\n  envFrom:\n    - prefix: APP_\n      configMapRef:\n        name: settings\n", "configmap.yaml": "kind: ConfigMap\nmetadata:\n  name: settings\ndata:\n  API_URL: secret-value\n"}, []string{"API_URL", "APP_API_URL"}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := application.Detector{Git: revisions{trees: map[string]map[string]string{"base": tc.base, "end": tc.end}}, Extractors: envdetect.DefaultExtractors()}
			got := d.Check(context.Background(), []domain.Range{{Repository: "/recorded", Start: "base", End: "end"}, {Repository: "/recorded", Start: "base", End: "end"}})
			if got.Status != "checked" || !reflect.DeepEqual(got.NewVariables, tc.want) || !reflect.DeepEqual(got.MissingFromTemplate, tc.missing) {
				t.Fatalf("got=%+v", got)
			}
			encoded, _ := json.Marshal(got)
			if strings.Contains(string(encoded), "secret-value") {
				t.Fatal("secret value leaked")
			}
		})
	}
}

func TestFailedAndPartialChecks(t *testing.T) {
	d := application.Detector{Git: revisions{broken: "broken", trees: map[string]map[string]string{"end": {"a.ts": "process.env.API_URL"}, "partial": {"a.ts": "process.env.API_URL", "compose.yml": "environment: [broken"}}}, Extractors: envdetect.DefaultExtractors()}
	if got := d.Check(context.Background(), nil); got.Status != "failed" {
		t.Fatalf("got=%+v", got)
	}
	if got := d.Check(context.Background(), []domain.Range{{End: "broken"}}); got.Status != "failed" {
		t.Fatalf("got=%+v", got)
	}
	got := d.Check(context.Background(), []domain.Range{{End: "partial"}})
	if got.Status != "incomplete" || !reflect.DeepEqual(got.NewVariables, []string{"API_URL"}) {
		t.Fatalf("got=%+v", got)
	}
	got = d.Check(context.Background(), []domain.Range{{End: "broken"}, {End: "end"}})
	if got.Status != "incomplete" || !reflect.DeepEqual(got.NewVariables, []string{"API_URL"}) {
		t.Fatalf("got=%+v", got)
	}
	d.Git = revisions{trees: map[string]map[string]string{"end": {"deployment.yaml": "spec:\n  envFrom:\n    - secretRef:\n        name: external-secret\n"}}}
	got = d.Check(context.Background(), []domain.Range{{End: "end"}})
	if got.Status != "incomplete" || len(got.NewVariables) != 0 {
		t.Fatalf("unresolved envFrom guessed a variable: %+v", got)
	}
}
