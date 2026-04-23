package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// InitResult contains the outcome of workspace initialization.
type InitResult struct {
	RepoRoot        string
	SpecsDir        string
	ConfigPath      string
	Created         bool
	DetectedPrefixes []string
	Config          *ProjectConfig
}

// InitWorkspace bootstraps specctl governance in the given directory.
// Creates .specs/ and specctl.yaml with auto-detected source prefixes.
// Idempotent: if specctl.yaml already exists, returns current state.
func InitWorkspace() (*InitResult, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	// Find git root, or use cwd if not in a git repo.
	repoRoot := cwd
	if gitRoot, err := findGitRoot(cwd); err == nil {
		repoRoot = gitRoot
	}

	specsDir := filepath.Join(repoRoot, ".specs")
	configPath := filepath.Join(specsDir, "specctl.yaml")

	// Idempotent: if config already exists, return current state.
	if data, err := os.ReadFile(configPath); err == nil {
		var existing ProjectConfig
		if yamlErr := yaml.Unmarshal(data, &existing); yamlErr != nil {
			return nil, fmt.Errorf("parsing existing config: %w", yamlErr)
		}
		return &InitResult{
			RepoRoot:        repoRoot,
			SpecsDir:        specsDir,
			ConfigPath:      configPath,
			Created:         false,
			DetectedPrefixes: nil,
			Config:          &existing,
		}, nil
	}

	// Create .specs/ directory.
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating .specs directory: %w", err)
	}

	// Auto-detect source prefixes from common directories.
	detected := DetectSourcePrefixes(repoRoot)
	config := DefaultConfig()
	if len(detected) > 0 {
		config.SourcePrefixes = detected
	}
	// New inits enable auto-rebind so req replace keeps open/deferred
	// deltas anchored across supersession. Existing installs without
	// the key keep the backwards-compatible false default.
	config.AutoRebindOnReplace = true

	// Write specctl.yaml.
	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("writing config: %w", err)
	}

	return &InitResult{
		RepoRoot:        repoRoot,
		SpecsDir:        specsDir,
		ConfigPath:      configPath,
		Created:         true,
		DetectedPrefixes: detected,
		Config:          config,
	}, nil
}

// findGitRoot walks up from start looking for a .git directory.
func findGitRoot(start string) (string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if info, err := os.Stat(filepath.Join(current, ".git")); err == nil && info.IsDir() {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("not a git repository")
		}
		current = parent
	}
}

// DetectSourcePrefixes scans common source directory patterns and returns
// those that actually exist on disk, normalized with trailing slash.
var candidatePrefixes = []string{
	"src/",
	"lib/",
	"app/",
	"cmd/",
	"internal/",
	"pkg/",
	"server/",
	"api/",
	"runtime/src/",
	"ui/src/",
	"ui/convex/",
	"ui/server/",
	"backend/src/",
	"frontend/src/",
	"services/",
}

func DetectSourcePrefixes(repoRoot string) []string {
	var found []string
	for _, prefix := range candidatePrefixes {
		if prefixDirExists(repoRoot, prefix) {
			found = append(found, prefix)
		}
	}
	return found
}

// SourcePrefixWarning is a structured warning for a source prefix that
// doesn't resolve to an existing directory on disk.
type SourcePrefixWarning struct {
	Kind         string `json:"kind"`
	Prefix       string `json:"prefix"`
	ResolvedPath string `json:"resolved_path"`
	Severity     string `json:"severity"`
}

// ValidateSourcePrefixes checks each configured source prefix against the
// filesystem and returns structured warnings for directories that don't exist.
func ValidateSourcePrefixes(repoRoot string, prefixes []string) []SourcePrefixWarning {
	var warnings []SourcePrefixWarning
	for _, prefix := range prefixes {
		trimmed := strings.TrimRight(prefix, "/")
		resolved := filepath.Join(repoRoot, filepath.FromSlash(trimmed))
		if !prefixDirExists(repoRoot, prefix) {
			warnings = append(warnings, SourcePrefixWarning{
				Kind:         "MISSING_SOURCE_PREFIX",
				Prefix:       prefix,
				ResolvedPath: resolved,
				Severity:     "warning",
			})
		}
	}
	return warnings
}

// prefixDirExists checks whether a source prefix directory exists on disk.
// Handles trailing slashes and OS path separators.
func prefixDirExists(repoRoot, prefix string) bool {
	trimmed := strings.TrimRight(prefix, "/")
	dir := filepath.Join(repoRoot, filepath.FromSlash(trimmed))
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
