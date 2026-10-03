// Package envdetect registers syntax-specific, names-only extractors behind a
// common interface. Adding a language requires no summary command changes.
package envdetect

import (
	application "github.com/taupikpirdian/wlog/application/environment"
	domain "github.com/taupikpirdian/wlog/domain/environment"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type PatternExtractor struct {
	Extensions []string
	Patterns   []*regexp.Regexp
}

func (e PatternExtractor) Supports(path string) bool {
	if !application.SupportedPath(path) {
		return false
	}
	for _, ext := range e.Extensions {
		if strings.EqualFold(filepath.Ext(path), ext) {
			return true
		}
	}
	return false
}

func (e PatternExtractor) Extract(path, content string) (domain.References, error) {
	// Comments are not implementation evidence. Keep quoted literals intact.
	content = stripComments(content, filepath.Ext(path))
	set := map[string]bool{}
	positions := codePositions(content)
	for _, pattern := range e.Patterns {
		for _, match := range pattern.FindAllStringSubmatchIndex(content, -1) {
			name := content[match[2]:match[3]]
			if positions[match[0]] && namePattern.MatchString(name) {
				set[name] = true
			}
		}
	}
	return domain.References{Names: names(set)}, nil
}

func codePositions(text string) []bool {
	positions := make([]bool, len(text))
	var quote byte
	escaped := false
	for i := range text {
		c := text[i]
		positions[i] = quote == 0
		if quote != 0 {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			quote = c
		}
	}
	return positions
}

func names(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func patterns(extensions []string, expressions ...string) PatternExtractor {
	e := PatternExtractor{Extensions: extensions}
	for _, p := range expressions {
		e.Patterns = append(e.Patterns, regexp.MustCompile(p))
	}
	return e
}

func DefaultExtractors() []application.Extractor {
	// Captures contain literal identifiers only; dynamic expressions do not match.
	const literal = `["']([A-Za-z_][A-Za-z0-9_]*)["']`
	return []application.Extractor{
		patterns([]string{".go"}, `\bos\.(?:Getenv|LookupEnv)\s*\(\s*`+literal+`\s*[,)]`),
		patterns([]string{".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"}, `\b(?:process|Bun)\.env\.([A-Za-z_][A-Za-z0-9_]*)\b`, `\b(?:process|Bun)\.env\s*\[\s*`+literal+`\s*\]`, `\bDeno\.env\.get\s*\(\s*`+literal+`\s*[,)]`),
		patterns([]string{".py"}, `\bos\.(?:getenv|environ\.get)\s*\(\s*`+literal+`\s*[,)]`, `\bos\.environ\s*\[\s*`+literal+`\s*\]`),
		patterns([]string{".php"}, `\b(?:getenv|env)\s*\(\s*`+literal+`\s*[,)]`, `\$_(?:ENV|SERVER)\s*\[\s*`+literal+`\s*\]`),
		patterns([]string{".java", ".kt", ".kts"}, `\bSystem\.getenv\s*\(\s*`+literal+`\s*[,)]`, `@Value\s*\(\s*"\$\{([A-Za-z_][A-Za-z0-9_]*)(?::[^}"]*)?\}"`, `\benvironment\.getProperty\s*\(\s*"([A-Z_][A-Z0-9_]*)"\s*[,)]`),
		DotNetExtractor{},
		patterns([]string{".rb"}, `\bENV\s*\[\s*`+literal+`\s*\]`, `\bENV\.fetch\s*\(\s*`+literal+`\s*[,)]`),
		patterns([]string{".rs"}, `\bstd::env::(?:var|var_os)\s*\(\s*`+literal+`\s*[,)]`),
		AssignmentExtractor{}, DockerExtractor{}, YAMLExtractor{},
	}
}

type DotNetExtractor struct{}

var dotNetDirect = patterns([]string{".cs"}, `\bEnvironment\.GetEnvironmentVariable\s*\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*[,)]`)
var environmentOnlyConfiguration = regexp.MustCompile(`\b(?:var|IConfiguration|IConfigurationRoot)\s+configuration\s*=\s*new\s+ConfigurationBuilder\s*\(\s*\)\s*\.\s*AddEnvironmentVariables\s*\(\s*\)\s*\.\s*Build\s*\(\s*\)`)
var dotNetConfiguration = patterns([]string{".cs"}, `\bconfiguration\s*\[\s*"([A-Za-z_][A-Za-z0-9_]*)"\s*\]`)

func (DotNetExtractor) Supports(path string) bool { return dotNetDirect.Supports(path) }
func (DotNetExtractor) Extract(path, text string) (domain.References, error) {
	refs, err := dotNetDirect.Extract(path, text)
	// Only an explicit, environment-only builder establishes provenance.
	// Mixed providers, aliases, prefixes and external setup stay unclassified.
	code := stripComments(text, ".cs")
	positions := codePositions(code)
	for _, match := range environmentOnlyConfiguration.FindAllStringIndex(code, -1) {
		if positions[match[0]] {
			extra, _ := dotNetConfiguration.Extract(path, text)
			refs.Names = append(refs.Names, extra.Names...)
			break
		}
	}
	set := map[string]bool{}
	for _, name := range refs.Names {
		set[name] = true
	}
	refs.Names = names(set)
	return refs, err
}

// Skip comments without stripping quoted strings containing #, // or /*.
func stripComments(text, ext string) string {
	var b strings.Builder
	var quote byte
	escaped, block := false, false
	hash := ext == ".py" || ext == ".rb"
	for i := 0; i < len(text); i++ {
		c := text[i]
		if block {
			if c == '*' && i+1 < len(text) && text[i+1] == '/' {
				block = false
				i++
			}
			if c == '\n' {
				b.WriteByte(c)
			}
			continue
		}
		if quote != 0 {
			b.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			quote = c
			b.WriteByte(c)
			continue
		}
		if !hash && c == '/' && i+1 < len(text) && text[i+1] == '*' {
			block = true
			b.WriteByte(' ')
			i++
			continue
		}
		if (hash && c == '#') || (!hash && c == '/' && i+1 < len(text) && text[i+1] == '/') {
			for i < len(text) && text[i] != '\n' {
				i++
			}
			if i < len(text) {
				b.WriteByte('\n')
			}
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

type AssignmentExtractor struct{}

func (AssignmentExtractor) Supports(path string) bool {
	if !application.SupportedPath(path) {
		return false
	}
	b := filepath.Base(path)
	ext := filepath.Ext(path)
	return b == ".env" || strings.HasPrefix(b, ".env.") || b == "example.env" || ext == ".env" || ext == ".sh" || ext == ".bash" || ext == ".zsh"
}

var assignment = regexp.MustCompile(`(?m)^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=`)
var exported = regexp.MustCompile(`(?m)^\s*export\s+([A-Za-z_][A-Za-z0-9_]*)(?:\s|$)`)

func (AssignmentExtractor) Extract(path string, text string) (domain.References, error) {
	set := map[string]bool{}
	ext := filepath.Ext(path)
	shell := ext == ".sh" || ext == ".bash" || ext == ".zsh"
	for _, p := range []*regexp.Regexp{assignment, exported} {
		for _, m := range p.FindAllStringSubmatch(text, -1) {
			if !shell || p == exported || strings.ToUpper(m[1]) == m[1] {
				set[m[1]] = true
			}
		}
	}
	return domain.References{Names: names(set)}, nil
}

type DockerExtractor struct{}

func (DockerExtractor) Supports(path string) bool {
	b := strings.ToLower(filepath.Base(path))
	return application.SupportedPath(path) && (b == "dockerfile" || strings.HasPrefix(b, "dockerfile.") || strings.HasSuffix(b, ".dockerfile"))
}

var interpolation = regexp.MustCompile(`\$\{?([A-Z_][A-Z0-9_]*)\b`)
var dockerAssignment = regexp.MustCompile(`(?:^|\s)([A-Za-z_][A-Za-z0-9_]*)=`)

func (DockerExtractor) Extract(_ string, text string) (domain.References, error) {
	set := map[string]bool{}
	text = strings.ReplaceAll(text, "\\\n", " ")
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "ENV", "ARG":
			if !strings.Contains(fields[1], "=") && namePattern.MatchString(fields[1]) {
				set[fields[1]] = true
			}
			body := strings.Join(fields[1:], " ")
			positions := codePositions(body)
			for _, m := range dockerAssignment.FindAllStringSubmatchIndex(body, -1) {
				if positions[m[2]] {
					set[body[m[2]:m[3]]] = true
				}
			}
		case "RUN":
			for _, m := range interpolation.FindAllStringSubmatch(line, -1) {
				if m[1] == "HOME" || m[1] == "PATH" || m[1] == "PWD" || m[1] == "USER" || m[1] == "SHELL" {
					continue
				}
				set[m[1]] = true
			}
		}
	}
	return domain.References{Names: names(set)}, nil
}
