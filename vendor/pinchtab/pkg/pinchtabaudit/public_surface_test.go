package pinchtabaudit

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateSurface = flag.Bool("update", false, "rewrite the pinned public surface")

const surfacePath = "testdata/public_surface.txt"

// This package is the module's only public Go surface, so a change to an exported name,
// field type or JSON tag is source-breaking for an external importer. The module is pre-1.0
// and docs/audit.md says such changes may happen without notice — so the point of this pin is
// not to prevent them. It is to make one ANNOUNCE ITSELF in the diff: the retype that
// prompted it broke no in-repo build and was not obviously a public-surface change to anyone
// reviewing it.
//
// It follows the acknowledge-on-change shape of the audit schema-version pin rather than
// inventing a second convention: a committed expectation that fails the moment the surface
// moves, updated deliberately in the same commit. It differs in HOW the expectation is
// stored, and that is the reason to prefer a file: a schema version is one token that fits in
// a literal, while this surface is every exported declaration in the package, and a reviewer
// needs to read WHICH line moved. A hash would fail just as loudly and say nothing.
//
// Everything is derived from the package source, so a newly added exported type is covered
// without anyone extending a list.
func TestPublicSurfaceIsPinned(t *testing.T) {
	got := renderPublicSurface(t)

	if *updateSurface {
		if err := os.MkdirAll(filepath.Dir(surfacePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(surfacePath, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(surfacePath)
	if err != nil {
		t.Fatalf("read the pinned surface: %v\nRun `go test ./pkg/pinchtabaudit -update` and review the diff.", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("the exported surface of this package no longer matches %s.\n"+
			"This package is the module's only public Go surface, so this diff is source-breaking for an external importer.\n"+
			"If the change is intended: run `go test ./pkg/pinchtabaudit -update`, read the diff, and commit it alongside the change so the break is visible in review.\n"+
			"If a field mirrors an internal type, internal/audit/types_test.go pins that pairing separately and may need the same edit.\n\n%s",
			surfacePath, firstDifference(string(want), string(got)))
	}
}

// A rename that keeps the same shape is the change most likely to slip through review, so it
// is the control this pin is proved with: the guard must red on the NAME alone.
//
// It is also the only thing standing between a weakened renderer and a permanently green pin.
// Drop the tag from the render, run -update, and the pin agrees with itself forever while a
// tag change stops announcing anything — measured, not supposed. So the sample must cover one
// instance of EVERY branch the renderer emits, not merely a representative few: a branch with
// no sample here can be deleted from the render and baked into the pin by one -update.
func TestPublicSurfaceRenderIncludesNamesTypesAndTags(t *testing.T) {
	rendered := string(renderPublicSurface(t))

	for _, want := range []string{
		"type AuditReport struct",
		"SchemaVersion string `json:\"schemaVersion\"`",
		"Input AuditReportInput `json:\"input\"`",
		"func (*Client) EnrichPage",
		"const DefaultTimeout",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the rendered surface is missing %q, so a change to it could not red this pin:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "func newRequest") || strings.Contains(rendered, "unexported") {
		t.Error("the rendered surface carries unexported declarations, which are not part of the public contract")
	}
}

func renderPublicSurface(t *testing.T) []byte {
	t.Helper()

	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("enumerate the package: %v", err)
	}

	fset := token.NewFileSet()
	var lines []string
	files := 0
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if file.Name.Name != "pinchtabaudit" {
			t.Fatalf("%s declares package %s, so this pin is scanning the wrong source", path, file.Name.Name)
		}
		files++
		for _, decl := range file.Decls {
			lines = append(lines, exportedDeclLines(fset, decl)...)
		}
	}
	if files < 2 {
		t.Fatalf("parsed %d non-test files; the package has more, so this pin has stopped seeing most of its source", files)
	}
	if len(lines) == 0 {
		t.Fatal("no exported declarations were found; this pin would pass vacuously")
	}
	sort.Strings(lines)

	return []byte(strings.Join(lines, "\n") + "\n")
}

func exportedDeclLines(fset *token.FileSet, decl ast.Decl) []string {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if !d.Name.IsExported() {
			return nil
		}
		receiver := ""
		if d.Recv != nil {
			if len(d.Recv.List) == 0 || !exportedReceiver(d.Recv.List[0].Type) {
				return nil
			}
			receiver = "(" + typeString(fset, d.Recv.List[0].Type) + ") "
		}
		return []string{fmt.Sprintf("func %s%s%s", receiver, d.Name.Name, typeString(fset, d.Type)[len("func"):])}
	case *ast.GenDecl:
		var lines []string
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if !s.Name.IsExported() {
					continue
				}
				lines = append(lines, typeSpecLines(fset, s)...)
			case *ast.ValueSpec:
				for _, name := range s.Names {
					if name.IsExported() {
						lines = append(lines, fmt.Sprintf("%s %s", d.Tok, name.Name))
					}
				}
			}
		}
		return lines
	}
	return nil
}

func typeSpecLines(fset *token.FileSet, spec *ast.TypeSpec) []string {
	structType, ok := spec.Type.(*ast.StructType)
	if !ok {
		return []string{fmt.Sprintf("type %s %s", spec.Name.Name, typeString(fset, spec.Type))}
	}

	lines := []string{fmt.Sprintf("type %s struct", spec.Name.Name)}
	for _, field := range structType.Fields.List {
		tag := ""
		if field.Tag != nil {
			tag = " " + field.Tag.Value
		}
		if len(field.Names) == 0 {
			lines = append(lines, fmt.Sprintf("type %s struct: %s%s", spec.Name.Name, typeString(fset, field.Type), tag))
			continue
		}
		for _, name := range field.Names {
			if !name.IsExported() {
				continue
			}
			lines = append(lines, fmt.Sprintf("type %s struct: %s %s%s", spec.Name.Name, name.Name, typeString(fset, field.Type), tag))
		}
	}
	return lines
}

func exportedReceiver(expr ast.Expr) bool {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	ident, ok := expr.(*ast.Ident)
	return ok && ident.IsExported()
}

func typeString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		return "<unprintable>"
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

// A whole-surface diff buries the one line that moved, which is what a reviewer needs.
func firstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		w, g := "", ""
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			return fmt.Sprintf("first difference at line %d:\n  pinned  %q\n  current %q", i+1, w, g)
		}
	}
	return "the files differ only in trailing bytes"
}
