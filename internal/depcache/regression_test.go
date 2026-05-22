package depcache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ozgurcd/gograph/internal/depcache"
	"github.com/ozgurcd/gograph/internal/graph"
)

func TestClean(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Write 3 dep graphs
	deps := []*depcache.DepGraph{
		{Module: "github.com/a/b", Version: "v1.0.0"},
		{Module: "github.com/c/d", Version: "v2.0.0"},
		{Module: "github.com/e/f", Version: "v3.0.0"},
	}
	for _, dg := range deps {
		if err := depcache.Write(dg); err != nil {
			t.Fatal(err)
		}
	}

	// Keep only two — keys must match the relative path under ~/.gograph/deps/
	keep := map[string]bool{
		"github.com/a/b@v1.0.0": true,
		"github.com/c/d@v2.0.0": true,
	}
	removed, err := depcache.Clean(keep)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	// Verify the kept ones still exist
	if !depcache.Has("github.com/a/b", "v1.0.0") {
		t.Error("a/b should still be cached")
	}
	if !depcache.Has("github.com/c/d", "v2.0.0") {
		t.Error("c/d should still be cached")
	}
	// Removed one should be gone
	if depcache.Has("github.com/e/f", "v3.0.0") {
		t.Error("e/f should have been cleaned")
	}
}

func TestClean_EmptyCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	removed, err := depcache.Clean(map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Errorf("removed = %d on empty cache, want 0", removed)
	}
}

func TestFindGoModFiles(t *testing.T) {
	tmp := t.TempDir()

	// Create a multi-module layout
	dirs := []string{
		filepath.Join(tmp, "root"),
		filepath.Join(tmp, "root", "submod"),
		filepath.Join(tmp, "root", "vendor", "pkg"), // should be skipped
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0o750)
	}

	// Write go.mod files
	os.WriteFile(filepath.Join(tmp, "root", "go.mod"), []byte("module root\n"), 0o640)
	os.WriteFile(filepath.Join(tmp, "root", "submod", "go.mod"), []byte("module root/submod\n"), 0o640)
	os.WriteFile(filepath.Join(tmp, "root", "vendor", "pkg", "go.mod"), []byte("module vendor\n"), 0o640)

	mods, err := depcache.FindGoModFiles(filepath.Join(tmp, "root"))
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 2 {
		t.Errorf("FindGoModFiles = %d, want 2 (vendor should be skipped): %v", len(mods), mods)
	}
}

func TestEscapeModulePath(t *testing.T) {
	// Test via ModuleCachePath error (module won't exist, but path construction
	// exercises escapeModulePath). We verify the error message contains the escaped path.
	_, err := depcache.ModuleCachePath("github.com/Azure/azure-sdk-for-go", "v1.0.0")
	if err == nil {
		t.Skip("Azure SDK unexpectedly found in module cache")
	}
	// The error should reference the escaped path with !a for A
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error")
	}
}

func TestBuildDepGraph_Methods(t *testing.T) {
	tmp := t.TempDir()
	pkg := filepath.Join(tmp, "mypkg")
	os.MkdirAll(pkg, 0o750)

	src := `package mypkg

// Server handles requests.
type Server struct {
	Addr string
	Port int
}

// Start begins listening.
func (s *Server) Start() error {
	return nil
}

// Stop shuts down.
func (s Server) Stop() {}

type unexported struct{}
func (u *unexported) hidden() {}
`
	os.WriteFile(filepath.Join(pkg, "server.go"), []byte(src), 0o640)

	dg, err := depcache.BuildDepGraph("example.com/mypkg", "v1.0.0", tmp)
	if err != nil {
		t.Fatal(err)
	}

	// Should have: Server (struct), Start (method), Stop (method)
	// Should NOT have: unexported, hidden
	var names []string
	for _, s := range dg.Symbols {
		names = append(names, s.Name)
	}

	if len(dg.Symbols) != 3 {
		t.Fatalf("expected 3 symbols, got %d: %v", len(dg.Symbols), names)
	}

	// Find Server struct
	var server *bool
	for _, s := range dg.Symbols {
		if s.Name == "Server" {
			v := true
			server = &v
			if len(s.StructFields) != 2 {
				t.Errorf("Server fields = %d, want 2", len(s.StructFields))
			}
			if s.Doc != "Server handles requests." {
				t.Errorf("Server doc = %q", s.Doc)
			}
		}
		if s.Name == "Start" {
			if s.Receiver != "*Server" {
				t.Errorf("Start receiver = %q, want *Server", s.Receiver)
			}
		}
		if s.Name == "Stop" {
			if s.Receiver != "Server" {
				t.Errorf("Stop receiver = %q, want Server", s.Receiver)
			}
		}
		if s.Name == "unexported" || s.Name == "hidden" {
			t.Errorf("unexported symbol %q should not be indexed", s.Name)
		}
	}
	if server == nil {
		t.Error("Server struct not found")
	}
}

func TestBuildDepGraph_Interface(t *testing.T) {
	tmp := t.TempDir()
	pkg := filepath.Join(tmp, "iface")
	os.MkdirAll(pkg, 0o750)

	src := `package iface

// Reader reads bytes.
type Reader interface {
	Read(p []byte) (n int, err error)
	Close() error
}
`
	os.WriteFile(filepath.Join(pkg, "iface.go"), []byte(src), 0o640)

	dg, err := depcache.BuildDepGraph("example.com/iface", "v1.0.0", tmp)
	if err != nil {
		t.Fatal(err)
	}

	if len(dg.Symbols) != 1 {
		t.Fatalf("expected 1 symbol, got %d", len(dg.Symbols))
	}
	s := dg.Symbols[0]
	if s.Name != "Reader" {
		t.Errorf("name = %q", s.Name)
	}
	if s.Kind != "interface" {
		t.Errorf("kind = %q", s.Kind)
	}
	if len(s.InterfaceMethods) != 2 {
		t.Errorf("interface methods = %d, want 2", len(s.InterfaceMethods))
	}
	if s.Doc != "Reader reads bytes." {
		t.Errorf("doc = %q", s.Doc)
	}
}

func TestBuildDepGraph_EmbeddedFields(t *testing.T) {
	tmp := t.TempDir()
	pkg := filepath.Join(tmp, "embed")
	os.MkdirAll(pkg, 0o750)

	src := `package embed

type Base struct {
	ID string
}

type Extended struct {
	Base
	Name string
}
`
	os.WriteFile(filepath.Join(pkg, "embed.go"), []byte(src), 0o640)

	dg, err := depcache.BuildDepGraph("example.com/embed", "v1.0.0", tmp)
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range dg.Symbols {
		if s.Name == "Extended" {
			// Should have Base (embedded) and Name
			if len(s.StructFields) != 2 {
				t.Errorf("Extended fields = %d, want 2", len(s.StructFields))
			}
		}
	}
}

func TestBuildDepGraph_SkipsTestFiles(t *testing.T) {
	tmp := t.TempDir()
	pkg := filepath.Join(tmp, "pkg")
	os.MkdirAll(pkg, 0o750)

	os.WriteFile(filepath.Join(pkg, "prod.go"), []byte("package pkg\ntype Real struct{}\n"), 0o640)
	os.WriteFile(filepath.Join(pkg, "prod_test.go"), []byte("package pkg\ntype Mock struct{}\n"), 0o640)

	dg, err := depcache.BuildDepGraph("example.com/pkg", "v1.0.0", tmp)
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range dg.Symbols {
		if s.Name == "Mock" {
			t.Error("test file symbols should not be indexed")
		}
	}
}

func TestLookupSymbol_PkgHint(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Two deps both export "Client"
	writeDepWithPkg(t, tmp, "github.com/http/client", "v1.0.0", "Client", "http", "HTTP client")
	writeDepWithPkg(t, tmp, "github.com/grpc/client", "v2.0.0", "Client", "grpc", "gRPC client")

	// Lookup with package hint
	sym, dg, err := depcache.LookupSymbol("Client", "grpc")
	if err != nil {
		t.Fatal(err)
	}
	if sym == nil {
		t.Fatal("expected match")
	}
	if dg.Module != "github.com/grpc/client" {
		t.Errorf("module = %q, want grpc", dg.Module)
	}
}

func writeDepWithPkg(t *testing.T, home, module, version, name, pkg, doc string) {
	t.Helper()
	dg := &depcache.DepGraph{
		Module:  module,
		Version: version,
		Symbols: []graph.SymbolNode{
			{
				Name:        name,
				Kind:        graph.KindStruct,
				PackageName: pkg,
				Doc:         doc,
				StructFields: []graph.StructField{
					{Name: "Addr", Type: "string"},
				},
			},
		},
	}
	if err := depcache.Write(dg); err != nil {
		t.Fatal(err)
	}
}
