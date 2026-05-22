package depcache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozgurcd/gograph/internal/depcache"
	"github.com/ozgurcd/gograph/internal/graph"
)

func TestWriteAndRead(t *testing.T) {
	// Use temp dir as home to isolate cache
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dg := &depcache.DepGraph{
		Module:  "github.com/example/lib",
		Version: "v1.2.3",
		Symbols: []graph.SymbolNode{
			{
				Name: "Widget",
				Kind: graph.KindStruct,
				StructFields: []graph.StructField{
					{Name: "ID", Type: "string"},
					{Name: "Count", Type: "int"},
				},
			},
		},
	}

	if err := depcache.Write(dg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := depcache.Read("github.com/example/lib", "v1.2.3")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got == nil {
		t.Fatal("Read returned nil (cache miss)")
	}
	if got.Module != "github.com/example/lib" {
		t.Errorf("module = %q, want github.com/example/lib", got.Module)
	}
	if len(got.Symbols) != 1 {
		t.Fatalf("symbols = %d, want 1", len(got.Symbols))
	}
	if got.Symbols[0].Name != "Widget" {
		t.Errorf("symbol name = %q, want Widget", got.Symbols[0].Name)
	}
	if len(got.Symbols[0].StructFields) != 2 {
		t.Errorf("fields = %d, want 2", len(got.Symbols[0].StructFields))
	}
}

func TestReadCacheMiss(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := depcache.Read("github.com/nonexistent/lib", "v0.0.0")
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for cache miss")
	}
}

func TestHas(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	if depcache.Has("github.com/example/lib", "v1.0.0") {
		t.Error("Has should return false before Write")
	}

	dg := &depcache.DepGraph{Module: "github.com/example/lib", Version: "v1.0.0"}
	if err := depcache.Write(dg); err != nil {
		t.Fatal(err)
	}

	if !depcache.Has("github.com/example/lib", "v1.0.0") {
		t.Error("Has should return true after Write")
	}
}

func TestList(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	deps := []*depcache.DepGraph{
		{Module: "github.com/a/b", Version: "v1.0.0"},
		{Module: "github.com/c/d", Version: "v2.1.0"},
	}
	for _, dg := range deps {
		if err := depcache.Write(dg); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := depcache.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("List() = %d entries, want 2", len(entries))
	}
}

func TestLookupSymbol(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dg := &depcache.DepGraph{
		Module:  "google.golang.org/protobuf",
		Version: "v1.36.11",
		Symbols: []graph.SymbolNode{
			{
				Name: "Timestamp",
				Kind: graph.KindStruct,
				Doc:  "A Timestamp represents a point in time.",
				StructFields: []graph.StructField{
					{Name: "Seconds", Type: "int64"},
					{Name: "Nanos", Type: "int32"},
				},
			},
			{
				Name: "New",
				Kind: graph.KindFunction,
			},
		},
	}
	if err := depcache.Write(dg); err != nil {
		t.Fatal(err)
	}

	sym, fromDG, err := depcache.LookupSymbol("Timestamp")
	if err != nil {
		t.Fatal(err)
	}
	if sym == nil {
		t.Fatal("LookupSymbol returned nil")
	}
	if sym.Name != "Timestamp" {
		t.Errorf("name = %q, want Timestamp", sym.Name)
	}
	if sym.Kind != graph.KindStruct {
		t.Errorf("kind = %q, want struct", sym.Kind)
	}
	if len(sym.StructFields) != 2 {
		t.Errorf("fields = %d, want 2", len(sym.StructFields))
	}
	if fromDG.Module != "google.golang.org/protobuf" {
		t.Errorf("module = %q", fromDG.Module)
	}
}

func TestLookupSymbol_CaseInsensitive(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	dg := &depcache.DepGraph{
		Module:  "example.com/pkg",
		Version: "v1.0.0",
		Symbols: []graph.SymbolNode{
			{Name: "MyStruct", Kind: graph.KindStruct},
		},
	}
	if err := depcache.Write(dg); err != nil {
		t.Fatal(err)
	}

	sym, _, err := depcache.LookupSymbol("mystruct")
	if err != nil {
		t.Fatal(err)
	}
	if sym == nil {
		t.Fatal("case-insensitive lookup failed")
	}
}

func TestParseGoMod(t *testing.T) {
	tmp := t.TempDir()
	gomod := filepath.Join(tmp, "go.mod")
	content := `module github.com/example/myproject

go 1.21

require (
	github.com/stretchr/testify v1.8.4
	google.golang.org/protobuf v1.36.11
	cloud.google.com/go/bigquery v1.77.0 // indirect
)

require github.com/single/dep v0.1.0
`
	if err := os.WriteFile(gomod, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}

	deps, err := depcache.ParseGoMod(gomod)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 4 {
		t.Fatalf("ParseGoMod = %d deps, want 4: %v", len(deps), deps)
	}

	want := map[string]string{
		"github.com/stretchr/testify":  "v1.8.4",
		"google.golang.org/protobuf":   "v1.36.11",
		"cloud.google.com/go/bigquery": "v1.77.0",
		"github.com/single/dep":        "v0.1.0",
	}
	for _, d := range deps {
		if v, ok := want[d.Module]; ok {
			if d.Version != v {
				t.Errorf("%s: version = %q, want %q", d.Module, d.Version, v)
			}
		}
	}
}

func TestBuildDepGraph(t *testing.T) {
	tmp := t.TempDir()

	// Create a fake dependency source tree
	pkg := filepath.Join(tmp, "types")
	if err := os.MkdirAll(pkg, 0o750); err != nil {
		t.Fatal(err)
	}

	src := `package types

// Widget is a thing.
type Widget struct {
	ID    string
	Name  string
	count int // unexported, should be skipped
}

// Doer does things.
type Doer interface {
	Do(x int) error
}

// NewWidget creates a Widget.
func NewWidget(id string) *Widget {
	return &Widget{ID: id}
}

func privateHelper() {} // should be skipped
`
	if err := os.WriteFile(filepath.Join(pkg, "types.go"), []byte(src), 0o640); err != nil {
		t.Fatal(err)
	}

	dg, err := depcache.BuildDepGraph("example.com/types", "v1.0.0", tmp)
	if err != nil {
		t.Fatal(err)
	}
	if dg.Module != "example.com/types" {
		t.Errorf("module = %q", dg.Module)
	}

	// Should have: Widget (struct), Doer (interface), NewWidget (func)
	// Should NOT have: privateHelper, count field
	if len(dg.Symbols) != 3 {
		t.Fatalf("symbols = %d, want 3: %v", len(dg.Symbols), symbolNames(dg))
	}

	widgetFound := false
	for _, s := range dg.Symbols {
		if s.Name == "Widget" {
			widgetFound = true
			if s.Kind != "struct" {
				t.Errorf("Widget kind = %q", s.Kind)
			}
			if s.Doc != "Widget is a thing." {
				t.Errorf("Widget doc = %q", s.Doc)
			}
			// Should have ID and Name, but not count (unexported)
			if len(s.StructFields) != 2 {
				t.Errorf("Widget fields = %d, want 2", len(s.StructFields))
			}
		}
		if s.Name == "privateHelper" {
			t.Error("unexported function should not be indexed")
		}
	}
	if !widgetFound {
		t.Error("Widget struct not found in symbols")
	}
}

func symbolNames(dg *depcache.DepGraph) []string {
	var names []string
	for _, s := range dg.Symbols {
		names = append(names, s.Name)
	}
	return names
}
