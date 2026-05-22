// Package depcache manages a global cache of lightweight dependency graphs
// stored at ~/.gograph/deps/. Each dependency is keyed by module@version and
// contains only symbol definitions (structs, interfaces, functions) with their
// fields and documentation — no call edges or other heavyweight data.
package depcache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// DepGraph is a lightweight graph containing only symbol information from a
// dependency module. It omits call edges, routes, SQL, concurrency, etc.
type DepGraph struct {
	Module  string             `json:"module"`
	Version string             `json:"version"`
	Symbols []graph.SymbolNode `json:"symbols"`
}

// CacheDir returns the global dependency cache directory (~/.gograph/deps/).
func CacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".gograph", "deps"), nil
}

// depPath returns the cache file path for a given module@version.
// e.g. ~/.gograph/deps/google.golang.org/protobuf@v1.36.11/graph.json
func depPath(cacheDir, module, version string) string {
	return filepath.Join(cacheDir, module+"@"+version, "graph.json")
}

// Has reports whether a dependency graph is cached for module@version.
func Has(module, version string) bool {
	cacheDir, err := CacheDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(depPath(cacheDir, module, version))
	return err == nil
}

// Read loads a cached dependency graph for module@version.
// Returns nil, nil if not cached (cache miss).
func Read(module, version string) (*DepGraph, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return nil, err
	}
	path := depPath(cacheDir, module, version)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil // cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("read dep cache %s@%s: %w", module, version, err)
	}
	var dg DepGraph
	if err := json.Unmarshal(data, &dg); err != nil {
		return nil, fmt.Errorf("parse dep cache %s@%s: %w", module, version, err)
	}
	return &dg, nil
}

// Write stores a dependency graph in the global cache.
func Write(dg *DepGraph) error {
	cacheDir, err := CacheDir()
	if err != nil {
		return err
	}
	path := depPath(cacheDir, dg.Module, dg.Version)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create dep cache dir: %w", err)
	}
	data, err := json.Marshal(dg)
	if err != nil {
		return fmt.Errorf("marshal dep graph: %w", err)
	}
	return os.WriteFile(path, data, 0o640)
}

// List returns all cached module@version entries.
func List() ([]string, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return nil, err
	}
	var entries []string
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() == "graph.json" && !info.IsDir() {
			rel, _ := filepath.Rel(cacheDir, filepath.Dir(path))
			entries = append(entries, rel)
		}
		return nil
	})
	return entries, err
}

// Clean removes cache entries not present in the given module@version set.
// The keep map keys should be "module@version" strings.
func Clean(keep map[string]bool) (removed int, err error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return 0, err
	}
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return 0, nil
	}

	// Walk all graph.json files and remove those not in keep set
	var toRemove []string
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || info.Name() != "graph.json" {
			return nil
		}
		// The parent directory name is module@version
		dir := filepath.Dir(path)
		rel, _ := filepath.Rel(cacheDir, dir)
		if !keep[rel] {
			toRemove = append(toRemove, dir)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, dir := range toRemove {
		if err := os.RemoveAll(dir); err == nil {
			removed++
		}
	}
	return removed, nil
}

// LookupSymbol searches all cached dep graphs for a symbol matching name.
// Returns the best match — prefers structs/interfaces, then picks the one with
// the most fields/documentation (avoids returning a bare embed from an unrelated pkg).
// Optional pkgHint filters by package name if non-empty.
func LookupSymbol(name string, pkgHint ...string) (*graph.SymbolNode, *DepGraph, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return nil, nil, err
	}
	nl := strings.ToLower(name)
	var pkg string
	if len(pkgHint) > 0 {
		pkg = strings.ToLower(pkgHint[0])
	}

	type candidate struct {
		sym *graph.SymbolNode
		dg  *DepGraph
	}
	var candidates []candidate

	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || info.Name() != "graph.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var dg DepGraph
		if err := json.Unmarshal(data, &dg); err != nil {
			return nil
		}
		for i := range dg.Symbols {
			s := &dg.Symbols[i]
			if strings.ToLower(s.Name) == nl {
				// If package hint provided, skip non-matching packages
				if pkg != "" && !strings.EqualFold(s.PackageName, pkg) {
					continue
				}
				dgCopy := dg
				candidates = append(candidates, candidate{sym: s, dg: &dgCopy})
			}
		}
		return nil
	})
	if err != nil || len(candidates) == 0 {
		return nil, nil, err
	}

	// Score candidates: prefer struct/interface > func/method, more fields > fewer, has doc > no doc
	best := candidates[0]
	bestScore := scoreCandidate(best.sym)
	for _, c := range candidates[1:] {
		s := scoreCandidate(c.sym)
		if s > bestScore {
			best = c
			bestScore = s
		}
	}
	return best.sym, best.dg, nil
}

func scoreCandidate(s *graph.SymbolNode) int {
	score := 0
	if s.Kind == graph.KindStruct || s.Kind == graph.KindInterface {
		score += 100
	}
	score += len(s.StructFields) * 10
	if s.Doc != "" {
		score += 50 + len(s.Doc)/10 // longer docs = more authoritative
	}
	return score
}
