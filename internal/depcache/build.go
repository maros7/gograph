package depcache

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// BuildDepGraph parses Go source files in a module cache directory and produces
// a lightweight DepGraph containing only symbol definitions (structs, interfaces,
// functions, methods) with their fields and doc comments. No call edges.
func BuildDepGraph(module, version, srcDir string) (*DepGraph, error) {
	dg := &DepGraph{
		Module:  module,
		Version: version,
	}

	goFiles, err := findGoFiles(srcDir)
	if err != nil {
		return nil, fmt.Errorf("walk dep source %s: %w", srcDir, err)
	}

	fset := token.NewFileSet()
	for _, path := range goFiles {
		if err := parseDepFile(fset, path, srcDir, dg); err != nil {
			continue // skip unparseable files
		}
	}

	return dg, nil
}

func findGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			switch base {
			case "testdata", "vendor", "internal":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func parseDepFile(fset *token.FileSet, path, srcDir string, dg *DepGraph) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return err
	}

	rel, _ := filepath.Rel(srcDir, path)
	pkgName := file.Name.Name

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					sym := graph.SymbolNode{
						Name:        s.Name.Name,
						PackageName: pkgName,
						File:        rel,
						Line:        fset.Position(s.Pos()).Line,
						EndLine:     fset.Position(s.End()).Line,
						Doc:         extractDoc(d.Doc, s.Doc),
					}
					switch t := s.Type.(type) {
					case *ast.StructType:
						sym.Kind = graph.KindStruct
						sym.StructFields = extractFields(t)
					case *ast.InterfaceType:
						sym.Kind = graph.KindInterface
						sym.InterfaceMethods = extractInterfaceMethods(t)
					default:
						continue // skip type aliases etc.
					}
					// Only export exported symbols
					if ast.IsExported(sym.Name) {
						dg.Symbols = append(dg.Symbols, sym)
					}
				}
			}
		case *ast.FuncDecl:
			if !ast.IsExported(d.Name.Name) {
				continue
			}
			sym := graph.SymbolNode{
				Name:        d.Name.Name,
				PackageName: pkgName,
				File:        rel,
				Line:        fset.Position(d.Pos()).Line,
				EndLine:     fset.Position(d.End()).Line,
				Doc:         extractDoc(d.Doc, nil),
				Signature:   funcSignature(d),
			}
			if d.Recv != nil && len(d.Recv.List) > 0 {
				sym.Kind = graph.KindMethod
				sym.Receiver = receiverType(d.Recv.List[0].Type)
			} else {
				sym.Kind = graph.KindFunction
			}
			dg.Symbols = append(dg.Symbols, sym)
		}
	}
	return nil
}

func extractDoc(groups ...*ast.CommentGroup) string {
	for _, g := range groups {
		if g != nil {
			return strings.TrimSpace(g.Text())
		}
	}
	return ""
}

func extractFields(st *ast.StructType) []graph.StructField {
	if st.Fields == nil {
		return nil
	}
	var fields []graph.StructField
	for _, f := range st.Fields.List {
		typStr := exprString(f.Type)
		tag := ""
		if f.Tag != nil {
			tag = f.Tag.Value
		}
		if len(f.Names) == 0 {
			// embedded field
			fields = append(fields, graph.StructField{
				Name: typStr,
				Type: typStr,
				Tag:  tag,
			})
		} else {
			for _, name := range f.Names {
				if ast.IsExported(name.Name) {
					fields = append(fields, graph.StructField{
						Name: name.Name,
						Type: typStr,
						Tag:  tag,
					})
				}
			}
		}
	}
	return fields
}

func extractInterfaceMethods(iface *ast.InterfaceType) map[string]string {
	if iface.Methods == nil {
		return nil
	}
	methods := make(map[string]string)
	for _, m := range iface.Methods.List {
		if len(m.Names) > 0 {
			methods[m.Names[0].Name] = exprString(m.Type)
		}
	}
	return methods
}

func funcSignature(d *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		b.WriteString("(")
		b.WriteString(receiverType(d.Recv.List[0].Type))
		b.WriteString(") ")
	}
	b.WriteString(d.Name.Name)
	b.WriteString("(")
	if d.Type.Params != nil {
		b.WriteString(fieldListString(d.Type.Params))
	}
	b.WriteString(")")
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		b.WriteString(" ")
		if len(d.Type.Results.List) == 1 && len(d.Type.Results.List[0].Names) == 0 {
			b.WriteString(exprString(d.Type.Results.List[0].Type))
		} else {
			b.WriteString("(")
			b.WriteString(fieldListString(d.Type.Results))
			b.WriteString(")")
		}
	}
	return b.String()
}

func fieldListString(fl *ast.FieldList) string {
	var parts []string
	for _, f := range fl.List {
		typStr := exprString(f.Type)
		if len(f.Names) == 0 {
			parts = append(parts, typStr)
		} else {
			for _, n := range f.Names {
				parts = append(parts, n.Name+" "+typStr)
			}
		}
	}
	return strings.Join(parts, ", ")
}

func receiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return exprString(expr)
	}
}

func exprString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + exprString(t.Elt)
		}
		return "[...]" + exprString(t.Elt)
	case *ast.MapType:
		return "map[" + exprString(t.Key) + "]" + exprString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.Ellipsis:
		return "..." + exprString(t.Elt)
	case *ast.ChanType:
		return "chan " + exprString(t.Value)
	default:
		return "?"
	}
}
