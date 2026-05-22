package depcache

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ModuleDep represents a dependency from go.mod.
type ModuleDep struct {
	Module  string
	Version string
}

// ParseGoMod reads a go.mod file and returns all require dependencies.
func ParseGoMod(goModPath string) ([]ModuleDep, error) {
	f, err := os.Open(goModPath)
	if err != nil {
		return nil, fmt.Errorf("open go.mod: %w", err)
	}
	defer f.Close()

	var deps []ModuleDep
	inRequire := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == ")" {
			inRequire = false
			continue
		}
		if strings.HasPrefix(line, "require (") || strings.HasPrefix(line, "require(") {
			inRequire = true
			continue
		}
		if strings.HasPrefix(line, "require ") && !strings.Contains(line, "(") {
			// Single-line require
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				deps = append(deps, ModuleDep{Module: parts[1], Version: parts[2]})
			}
			continue
		}
		if inRequire {
			// Skip comments and indirect markers
			line = strings.Split(line, "//")[0]
			line = strings.TrimSpace(line)
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				deps = append(deps, ModuleDep{Module: parts[0], Version: parts[1]})
			}
		}
	}
	return deps, scanner.Err()
}

// ModuleCachePath returns the filesystem path in the Go module cache for a
// given module@version. Uses `go env GOMODCACHE` to find the cache root.
func ModuleCachePath(module, version string) (string, error) {
	modCache, err := goModCache()
	if err != nil {
		return "", err
	}
	// Go module cache uses module path with capitalized letters escaped.
	// e.g. github.com/Azure → github.com/!azure
	escaped := escapeModulePath(module)
	path := filepath.Join(modCache, escaped+"@"+version)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("module not in cache: %s@%s (expected at %s)", module, version, path)
	}
	return path, nil
}

// goModCache returns the GOMODCACHE path.
func goModCache() (string, error) {
	// Try GOMODCACHE env first
	if v := os.Getenv("GOMODCACHE"); v != "" {
		return v, nil
	}
	// Fall back to `go env GOMODCACHE`
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		return "", fmt.Errorf("cannot determine GOMODCACHE: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// escapeModulePath escapes uppercase letters in a module path for the Go module
// cache filesystem layout. Each uppercase letter X becomes !x.
func escapeModulePath(path string) string {
	var b strings.Builder
	for _, r := range path {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FindGoModFiles finds all go.mod files under root (respecting ignored dirs).
func FindGoModFiles(root string) ([]string, error) {
	var mods []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			switch base {
			case "vendor", "node_modules", ".git", ".gograph", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == "go.mod" {
			mods = append(mods, path)
		}
		return nil
	})
	return mods, err
}
