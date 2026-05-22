// Package scanner walks the target repository and identifies Go files to parse.
package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ignoredDirs are directory names that are skipped during the walk.
var ignoredDirs = map[string]bool{
	".git":         true,
	".gograph":     true,
	"vendor":       true,
	"node_modules": true,
	"dist":         true,
	"build":        true,
	".terraform":   true,
}

// Options controls the scanner behavior.
type Options struct {
	// IncludeGenerated includes generated Go files (*.pb.go, *_generated.go,
	// and files with "Code generated" comments) that are normally skipped.
	IncludeGenerated bool
}

// ShouldIgnoreDir reports whether the directory with the given base name should
// be skipped entirely.
func ShouldIgnoreDir(base string) bool {
	return ignoredDirs[base]
}

// ShouldIgnoreFile reports whether a file should be skipped before parsing.
// It checks name suffixes and, for .go files, inspects the first few lines for
// a generated-file marker.
func ShouldIgnoreFile(path string) (bool, error) {
	return shouldIgnoreFile(path, false)
}

// shouldIgnoreFile is the internal implementation that respects includeGenerated.
func shouldIgnoreFile(path string, includeGenerated bool) (bool, error) {
	if includeGenerated {
		return false, nil
	}

	base := filepath.Base(path)

	// Suffix checks (cheap, no I/O).
	if strings.HasSuffix(base, ".pb.go") {
		return true, nil
	}
	if strings.HasSuffix(base, "_generated.go") {
		return true, nil
	}

	// Content check: look for "Code generated" in the first 10 lines.
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() && lineNum < 10 {
		if strings.Contains(scanner.Text(), "Code generated") {
			return true, nil
		}
		lineNum++
	}
	return false, scanner.Err()
}

// Walk traverses root and returns paths of .go files that should be parsed.
// Generated files are excluded. An error slice is also returned for files that
// could not be inspected (non-fatal).
func Walk(root string) (paths []string, errs []error) {
	return WalkWithOptions(root, Options{})
}

// WalkWithOptions traverses root and returns paths of .go files that should be
// parsed. The Options struct controls whether generated files are included.
func WalkWithOptions(root string, opts Options) (paths []string, errs []error) {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errs = append(errs, err)
			return nil // keep walking
		}
		if info.IsDir() {
			if ShouldIgnoreDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		skip, serr := shouldIgnoreFile(path, opts.IncludeGenerated)
		if serr != nil {
			errs = append(errs, serr)
			return nil
		}
		if !skip {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}
	return paths, errs
}
