package environment

import (
	"context"
	domain "github.com/taupikpirdian/wlog/domain/environment"
	"path/filepath"
	"sort"
	"strings"
)

type RevisionReader interface {
	Files(context.Context, string, string) ([]string, error)
	ChangedFiles(context.Context, string, string, string) ([]string, error)
	ReadFile(context.Context, string, string, string) (string, error)
}

type Extractor interface {
	Supports(string) bool
	Extract(string, string) (domain.References, error)
}

type Detector struct {
	Git        RevisionReader
	Extractors []Extractor
}

func (d *Detector) supports(path string) bool {
	for _, e := range d.Extractors {
		if e.Supports(path) {
			return true
		}
	}
	return false
}

func (d *Detector) extract(path, text string) (domain.References, bool) {
	var refs domain.References
	ok := true
	for _, e := range d.Extractors {
		if !e.Supports(path) {
			continue
		}
		r, err := e.Extract(path, text)
		if err != nil {
			ok = false
			continue
		}
		for _, name := range r.Names {
			if domain.ValidName(name) {
				refs.Names = append(refs.Names, name)
			} else {
				ok = false
			}
		}
		for _, name := range r.CIOnly {
			if domain.ValidName(name) {
				refs.CIOnly = append(refs.CIOnly, name)
			} else {
				ok = false
			}
		}
		refs.Imports = append(refs.Imports, r.Imports...)
		if refs.Examples == nil {
			refs.Examples = map[string]string{}
		}
		for name, value := range r.Examples {
			if domain.ValidName(name) {
				refs.Examples[name] = value
			}
		}
		if refs.Resources == nil {
			refs.Resources = map[string][]string{}
		}
		for resource, names := range r.Resources {
			if _, exists := refs.Resources[resource]; !exists {
				refs.Resources[resource] = []string{}
			}
			for _, name := range names {
				if domain.ValidName(name) {
					refs.Resources[resource] = append(refs.Resources[resource], name)
				} else {
					ok = false
				}
			}
		}
		ok = ok && !r.Incomplete
	}
	return refs, ok
}

func IsTemplate(path string) bool {
	b := filepath.Base(path)
	return b == ".env.example" || b == ".env.sample" || b == ".env.template" || b == "example.env"
}

func sorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func exampleValues(index map[string]domain.References) map[string]string {
	paths := make([]string, 0, len(index))
	for path := range index {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	values := map[string]string{}
	// Templates take precedence over source defaults; path order breaks ties.
	for _, template := range []bool{false, true} {
		for _, path := range paths {
			if IsTemplate(path) != template {
				continue
			}
			for name, value := range index[path].Examples {
				values[name] = value
			}
		}
	}
	return values
}

// readIndex retains names and example defaults, never runtime environment values.
func (d *Detector) readIndex(ctx context.Context, repository, revision string) (map[string]domain.References, bool) {
	files, err := d.Git.Files(ctx, repository, revision)
	if err != nil {
		return nil, false
	}
	index := map[string]domain.References{}
	resources := map[string][]string{}
	complete, count, bytes := true, 0, 0
	for _, path := range files {
		if !d.supports(path) {
			continue
		}
		count++
		if count > 2048 || bytes > 8*1024*1024 || ctx.Err() != nil {
			complete = false
			break
		}
		text, err := d.Git.ReadFile(ctx, repository, revision, path)
		if err != nil {
			complete = false
			continue
		}
		bytes += len(text)
		refs, ok := d.extract(path, text)
		complete = complete && ok
		index[path] = refs
		for resource, names := range refs.Resources {
			resources[resource] = append(resources[resource], names...)
		}
	}
	for path, refs := range index {
		for _, imported := range refs.Imports {
			keys, available := resources[imported.Resource]
			if !available {
				complete = false
				continue
			}
			for _, key := range keys {
				name := imported.Prefix + key
				if domain.ValidName(name) {
					refs.Names = append(refs.Names, name)
				} else {
					complete = false
				}
			}
		}
		index[path] = refs
	}
	return index, complete
}

// Compare changed end-revision files against the entire supported base tree.
// Revision indexes are cached within a check for overlapping captured ranges.
func (d *Detector) Check(ctx context.Context, ranges []domain.Range) domain.Changes {
	result := domain.Changes{Status: "failed", NewVariables: []string{}, MissingFromTemplate: []string{}}
	if d.Git == nil || len(ranges) == 0 {
		return result
	}
	newNames, missing, ci, removed, modified := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	result.Examples, result.RemovedExamples = map[string]string{}, map[string]string{}
	result.PreviousExamples = map[string]string{}
	complete, successful := true, 0
	type revisionIndex struct {
		files    map[string]domain.References
		complete bool
	}
	cache := map[string]revisionIndex{}
	load := func(repository, revision string) revisionIndex {
		key := repository + "\x00" + revision
		if index, found := cache[key]; found {
			return index
		}
		files, ok := d.readIndex(ctx, repository, revision)
		index := revisionIndex{files, ok}
		cache[key] = index
		return index
	}
	seen := map[string]bool{}
	for i, r := range ranges {
		if i >= 128 || ctx.Err() != nil {
			complete = false
			break
		}
		key := r.Repository + "\x00" + r.Start + "\x00" + r.End
		if seen[key] {
			continue
		}
		seen[key] = true
		baseNames := map[string]bool{}
		baseExamples := map[string]string{}
		if r.Start != "" {
			base := load(r.Repository, r.Start)
			// Incomplete base evidence cannot establish that a name is new.
			if !base.complete {
				complete = false
				continue
			}
			for _, refs := range base.files {
				for _, name := range append(refs.Names, refs.CIOnly...) {
					baseNames[name] = true
				}
			}
			baseExamples = exampleValues(base.files)
		}
		files, err := d.Git.ChangedFiles(ctx, r.Repository, r.Start, r.End)
		if err != nil {
			complete = false
			continue
		}
		end := load(r.Repository, r.End)
		if end.files == nil {
			complete = false
			continue
		}
		complete = complete && end.complete
		endNames := map[string]bool{}
		endExamples := exampleValues(end.files)
		for name, value := range endExamples {
			result.Examples[name] = value
		}
		for _, refs := range end.files {
			for _, name := range append(refs.Names, refs.CIOnly...) {
				endNames[name] = true
			}
		}
		if end.complete {
			for name := range baseNames {
				if !endNames[name] {
					removed[name] = true
					if value, ok := baseExamples[name]; ok {
						result.RemovedExamples[name] = value
					}
				}
			}
		}
		for name, before := range baseExamples {
			if after, ok := endExamples[name]; ok && endNames[name] && before != after {
				modified[name] = true
				result.PreviousExamples[name] = before
			}
		}
		template := map[string]bool{}
		hasTemplate := false
		for path, refs := range end.files {
			if !IsTemplate(path) {
				continue
			}
			hasTemplate = true
			for _, name := range refs.Names {
				template[name] = true
			}
		}
		successful++
		for _, path := range files {
			refs, exists := end.files[path]
			if !exists {
				continue
			} // deleted or unsupported
			for _, name := range refs.Names {
				if !baseNames[name] {
					newNames[name] = true
					if hasTemplate && end.complete && !template[name] {
						missing[name] = true
					}
				}
			}
			for _, name := range refs.CIOnly {
				if !baseNames[name] {
					ci[name] = true
				}
			}
		}
	}
	if ctx.Err() != nil {
		complete = false
	}
	if successful > 0 {
		result.Status = "incomplete"
		if complete {
			result.Status = "checked"
		}
	}
	result.NewVariables, result.MissingFromTemplate, result.CIOnlyVariables = sorted(newNames), sorted(missing), sorted(ci)
	result.RemovedVariables = sorted(removed)
	result.ModifiedVariables = sorted(modified)
	return result
}

// Ignore generated/dependency trees; new extractors can be registered without
// changing the summary command or revision comparison algorithm.
func SupportedPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == "node_modules" || part == "vendor" || part == ".git" {
			return false
		}
	}
	return true
}
