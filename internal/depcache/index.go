package depcache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ozgurcd/gograph/internal/graph"
)

// IndexEntry is a lightweight reference to a symbol in the dep cache.
// The full symbol data lives in the dep graph file; this is just for fast lookup.
type IndexEntry struct {
	Module      string         `json:"m"`
	Version     string         `json:"v"`
	Package     string         `json:"p"`
	Kind        graph.SymbolKind `json:"k"`
	HasDoc      bool           `json:"d,omitempty"`
	FieldCount  int            `json:"f,omitempty"`
}

// SymbolIndex is a name → entries mapping for fast symbol lookup.
// Stored at ~/.gograph/deps/index.json.
type SymbolIndex struct {
	Symbols map[string][]IndexEntry `json:"symbols"`
}

var (
	cachedIndex *SymbolIndex
	indexOnce   sync.Once
	indexErr    error
)

// indexPath returns the path to the global symbol index.
func indexPath() (string, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "index.json"), nil
}

// BuildIndex scans all cached dep graphs and builds a symbol name index.
// This is called after --index-deps completes.
func BuildIndex() error {
	cacheDir, err := CacheDir()
	if err != nil {
		return err
	}

	idx := &SymbolIndex{Symbols: make(map[string][]IndexEntry)}

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
		for _, s := range dg.Symbols {
			entry := IndexEntry{
				Module:     dg.Module,
				Version:    dg.Version,
				Package:    s.PackageName,
				Kind:       s.Kind,
				HasDoc:     s.Doc != "",
				FieldCount: len(s.StructFields),
			}
			key := strings.ToLower(s.Name)
			idx.Symbols[key] = append(idx.Symbols[key], entry)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Write index
	ipath, err := indexPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	// Reset cached index
	cachedIndex = nil
	indexOnce = sync.Once{}

	return os.WriteFile(ipath, data, 0o640)
}

// LoadIndex loads the symbol index from disk (cached after first load).
func LoadIndex() (*SymbolIndex, error) {
	indexOnce.Do(func() {
		ipath, err := indexPath()
		if err != nil {
			indexErr = err
			return
		}
		data, err := os.ReadFile(ipath)
		if err != nil {
			indexErr = err
			return
		}
		cachedIndex = &SymbolIndex{}
		indexErr = json.Unmarshal(data, cachedIndex)
	})
	return cachedIndex, indexErr
}

// ResetIndex resets the cached index (for testing).
func ResetIndex() {
	cachedIndex = nil
	indexOnce = sync.Once{}
	indexErr = nil
}

// LookupSymbolFast uses the symbol index for O(1) name lookup, then loads
// only the specific dep graph file needed. Much faster than scanning all files.
func LookupSymbolFast(name string, pkgHint ...string) (*graph.SymbolNode, *DepGraph, error) {
	idx, err := LoadIndex()
	if err != nil {
		// Fall back to slow path if index doesn't exist
		return LookupSymbol(name, pkgHint...)
	}

	nl := strings.ToLower(name)
	var pkg string
	if len(pkgHint) > 0 {
		pkg = strings.ToLower(pkgHint[0])
	}

	entries, ok := idx.Symbols[nl]
	if !ok || len(entries) == 0 {
		return nil, nil, nil
	}

	// Filter by package hint and score
	var best *IndexEntry
	bestScore := -1
	for i := range entries {
		e := &entries[i]
		if pkg != "" && !strings.EqualFold(e.Package, pkg) {
			continue
		}
		score := scoreEntry(e)
		if score > bestScore {
			best = e
			bestScore = score
		}
	}
	if best == nil {
		return nil, nil, nil
	}

	// Load the specific dep graph
	dg, err := Read(best.Module, best.Version)
	if err != nil || dg == nil {
		return nil, nil, err
	}

	// Find the exact symbol in the loaded graph
	for i := range dg.Symbols {
		s := &dg.Symbols[i]
		if strings.ToLower(s.Name) == nl {
			if pkg != "" && !strings.EqualFold(s.PackageName, pkg) {
				continue
			}
			return s, dg, nil
		}
	}
	return nil, nil, nil
}

func scoreEntry(e *IndexEntry) int {
	score := 0
	if e.Kind == graph.KindStruct || e.Kind == graph.KindInterface {
		score += 100
	}
	score += e.FieldCount * 10
	if e.HasDoc {
		score += 50
	}
	return score
}

// IndexSize returns the number of unique symbol names in the index.
func IndexSize() (int, error) {
	idx, err := LoadIndex()
	if err != nil {
		return 0, err
	}
	return len(idx.Symbols), nil
}

// LookupIndex checks if a symbol exists in the index without loading dep graphs.
func LookupIndex(name string) []IndexEntry {
	idx, err := LoadIndex()
	if err != nil {
		return nil
	}
	return idx.Symbols[strings.ToLower(name)]
}

// FormatIndexEntry returns a human-readable string for an index entry.
func (e IndexEntry) String() string {
	return fmt.Sprintf("%s@%s (%s.%s)", e.Module, e.Version, e.Package, e.Kind)
}
