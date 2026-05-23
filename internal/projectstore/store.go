// Package projectstore manages global per-project, per-branch graph storage.
// Graphs are stored at ~/.gograph/projects/<repo-id>/<branch>/graph.json
// so nothing lives inside the project repository itself.
package projectstore

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ProjectDir returns the global storage directory for a given project + branch.
// Structure: ~/.gograph/projects/<repo-id>/<branch>/
func ProjectDir(repoRoot string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home: %w", err)
	}
	repoID := resolveRepoID(repoRoot)
	return filepath.Join(home, ".gograph", "projects", repoID), nil
}

// BranchDir returns the storage directory for a specific branch within a project.
func BranchDir(repoRoot, branch string) (string, error) {
	projDir, err := ProjectDir(repoRoot)
	if err != nil {
		return "", err
	}
	safeBranch := sanitizeBranch(branch)
	return filepath.Join(projDir, safeBranch), nil
}

// GraphPath returns the path for graph.json for the current project + branch.
func GraphPath(repoRoot string) (string, error) {
	branch := detectBranch(repoRoot)
	dir, err := BranchDir(repoRoot, branch)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "graph.json"), nil
}

// ReportDir returns the directory for markdown reports for current branch.
func ReportDir(repoRoot string) (string, error) {
	branch := detectBranch(repoRoot)
	return BranchDir(repoRoot, branch)
}

// Exists checks whether a graph exists for the current branch.
func Exists(repoRoot string) bool {
	p, err := GraphPath(repoRoot)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// WriteGraph writes the graph JSON to the global store for the current branch.
func WriteGraph(repoRoot string, data []byte) error {
	p, err := GraphPath(repoRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o640)
}

// WriteReport writes a markdown report to the global store.
func WriteReport(repoRoot, filename string, data []byte) error {
	dir, err := ReportDir(repoRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, filename), data, 0o640)
}

// ReadGraph reads the graph JSON for the current branch from global store.
func ReadGraph(repoRoot string) ([]byte, error) {
	p, err := GraphPath(repoRoot)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// BranchEntry holds metadata about a stored branch index.
type BranchEntry struct {
	Branch    string    `json:"branch"`
	Size      int64     `json:"size_bytes"`
	ModTime   time.Time `json:"modified"`
	AgeDays   int       `json:"age_days"`
}

// ListBranches lists all indexed branches for a project.
func ListBranches(repoRoot string) ([]BranchEntry, error) {
	projDir, err := ProjectDir(repoRoot)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(projDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var branches []BranchEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		graphPath := filepath.Join(projDir, e.Name(), "graph.json")
		info, err := os.Stat(graphPath)
		if err != nil {
			continue
		}
		branches = append(branches, BranchEntry{
			Branch:  unsanitizeBranch(e.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			AgeDays: int(time.Since(info.ModTime()).Hours() / 24),
		})
	}
	return branches, nil
}

// ListProjects lists all projects in the global store.
func ListProjects() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	projectsDir := filepath.Join(home, ".gograph", "projects")
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var projects []string
	err = filepath.Walk(projectsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.Name() == "graph.json" && !info.IsDir() {
			rel, _ := filepath.Rel(projectsDir, filepath.Dir(filepath.Dir(path)))
			if !contains(projects, rel) {
				projects = append(projects, rel)
			}
		}
		return nil
	})
	return projects, err
}

// CleanStale removes branch indices older than maxAgeDays.
// Protected branches (main, master, develop) are never cleaned.
// Returns the number of branches removed.
func CleanStale(repoRoot string, maxAgeDays int) (removed int, err error) {
	branches, err := ListBranches(repoRoot)
	if err != nil {
		return 0, err
	}
	projDir, err := ProjectDir(repoRoot)
	if err != nil {
		return 0, err
	}

	for _, b := range branches {
		if isProtectedBranch(b.Branch) {
			continue
		}
		if b.AgeDays > maxAgeDays {
			branchDir := filepath.Join(projDir, sanitizeBranch(b.Branch))
			if err := os.RemoveAll(branchDir); err == nil {
				removed++
			}
		}
	}
	return removed, nil
}

// CleanAll removes all branch indices for a project.
func CleanAll(repoRoot string) error {
	projDir, err := ProjectDir(repoRoot)
	if err != nil {
		return err
	}
	return os.RemoveAll(projDir)
}

// --- Helpers ---

func resolveRepoID(repoRoot string) string {
	// Try git remote URL first
	if url := gitCmd(repoRoot, "remote", "get-url", "origin"); url != "" {
		// Normalize: strip .git suffix, protocol prefix
		url = strings.TrimSuffix(url, ".git")
		url = strings.TrimPrefix(url, "https://")
		url = strings.TrimPrefix(url, "http://")
		url = strings.TrimPrefix(url, "git@")
		url = strings.Replace(url, ":", "/", 1) // git@github.com:org/repo → github.com/org/repo
		return url
	}
	// Fallback: use absolute path
	abs, _ := filepath.Abs(repoRoot)
	return strings.ReplaceAll(abs, "/", "_")
}

func detectBranch(repoRoot string) string {
	if branch := gitCmd(repoRoot, "rev-parse", "--abbrev-ref", "HEAD"); branch != "" && branch != "HEAD" {
		return branch
	}
	// Detached HEAD — use short commit
	if commit := gitCmd(repoRoot, "rev-parse", "--short", "HEAD"); commit != "" {
		return "detached-" + commit
	}
	return "unknown"
}

func gitCmd(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func sanitizeBranch(branch string) string {
	return strings.ReplaceAll(branch, "/", "--")
}

func unsanitizeBranch(safe string) string {
	return strings.ReplaceAll(safe, "--", "/")
}

func isProtectedBranch(branch string) bool {
	switch branch {
	case "main", "master", "develop", "release":
		return true
	}
	return false
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// MigrateFromLocal moves a .gograph/graph.json from inside the project to the global store.
// Called once to migrate existing setups. Non-destructive (copies, doesn't delete).
func MigrateFromLocal(repoRoot string) error {
	localPath := filepath.Join(repoRoot, ".gograph", "graph.json")
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil // nothing to migrate
	}

	// Detect current state
	var g struct {
		GeneratedAt string `json:"generated_at"`
	}
	json.Unmarshal(data, &g)

	return WriteGraph(repoRoot, data)
}
