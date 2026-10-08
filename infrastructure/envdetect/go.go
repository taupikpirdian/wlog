package envdetect

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"

	domain "github.com/taupikpirdian/wlog/domain/environment"
)

var goDirect = patterns([]string{".go"}, `\bos\.(?:Getenv|LookupEnv)\s*\(\s*["']([A-Za-z_][A-Za-z0-9_]*)["']\s*[,)]`)

type GoExtractor struct{}

func (GoExtractor) Supports(path string) bool { return goDirect.Supports(path) }

// Resolve literal calls to local helpers whose parameter is passed to os.Getenv.
// Dynamic names and helpers without environment evidence remain unclassified.
func (GoExtractor) Extract(path, source string) (domain.References, error) {
	refs, _ := goDirect.Extract(path, source)
	file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
	if err != nil {
		return refs, nil // Direct patterns also support partial source snippets.
	}
	constants := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			return true
		}
		for _, spec := range decl.Specs {
			value := spec.(*ast.ValueSpec)
			for i, name := range value.Names {
				if i < len(value.Values) {
					if literal, ok := value.Values[i].(*ast.BasicLit); ok && literal.Kind == token.STRING {
						if text, err := strconv.Unquote(literal.Value); err == nil {
							constants[name.Name] = text
						}
					}
				}
			}
		}
		return false
	})
	type helper struct{ nameIndex, defaultIndex int }
	helpers := map[string]helper{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil {
			continue
		}
		params := map[string]int{}
		index := 0
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				params[name.Name] = index
				index++
			}
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "Getenv" && selector.Sel.Name != "LookupEnv") {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "os" {
				return true
			}
			arg, ok := call.Args[0].(*ast.Ident)
			if !ok {
				return true
			}
			position, ok := params[arg.Name]
			if !ok {
				return true
			}
			h := helper{position, -1}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				ret, ok := n.(*ast.ReturnStmt)
				if ok && len(ret.Results) == 1 {
					if id, ok := ret.Results[0].(*ast.Ident); ok {
						if p, ok := params[id.Name]; ok && p != position {
							h.defaultIndex = p
						}
					}
				}
				return true
			})
			helpers[fn.Name.Name] = h
			return true
		})
	}
	resolve := func(expr ast.Expr) (string, bool) {
		if literal, ok := expr.(*ast.BasicLit); ok && literal.Kind == token.STRING {
			s, err := strconv.Unquote(literal.Value)
			return s, err == nil
		}
		if id, ok := expr.(*ast.Ident); ok {
			s, ok := constants[id.Name]
			return s, ok
		}
		return "", false
	}
	set := map[string]bool{}
	for _, name := range refs.Names {
		set[name] = true
	}
	refs.Examples = map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fn, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		h, ok := helpers[fn.Name]
		if !ok || h.nameIndex >= len(call.Args) {
			return true
		}
		name, ok := resolve(call.Args[h.nameIndex])
		if !ok || !domain.ValidName(name) {
			return true
		}
		set[name] = true
		if h.defaultIndex >= 0 && h.defaultIndex < len(call.Args) {
			if value, ok := resolve(call.Args[h.defaultIndex]); ok {
				refs.Examples[name] = value
			}
		}
		return true
	})
	refs.Names = names(set)
	return refs, nil
}
