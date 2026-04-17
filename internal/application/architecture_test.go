package application

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/aitoroses/specctl/internal/infrastructure"
)

func TestApplicationLayerDoesNotRegainInfrastructureImports(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(currentFile)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}

	bannedImports := map[string]string{
		"os":               "filesystem access belongs in infrastructure",
		"os/exec":          "git/process access belongs in infrastructure",
		"path/filepath":    "filesystem path handling belongs in infrastructure or use path for slash-only joins",
		"gopkg.in/yaml.v3": "YAML encoding/decoding belongs in infrastructure",
	}
	bannedSnippets := map[string]string{
		"infrastructure.WriteYAMLAtomically(":                "application should persist via infrastructure stores",
		"infrastructure.CommitWritesAtomically(":             "application should persist via infrastructure stores",
		"infrastructure.ReadGitFile(":                        "application should load git snapshots via infrastructure adapters",
		"infrastructure.ResolveGitRevision(":                 "application should resolve checkpoints via infrastructure adapters",
		"infrastructure.ListScopeFiles(":                     "application should load drift inputs via infrastructure adapters",
		"infrastructure.LatestGitCommitTimestamp(":           "application should load drift inputs via infrastructure adapters",
		"infrastructure.LoadProjectConfigLenient(":           "application should aggregate repo reads via infrastructure adapters",
		"infrastructure.FindAllTrackingFiles(":               "application should aggregate repo reads via infrastructure adapters",
		"infrastructure.ReadTrackingFileLenientWithConfig(":  "application should aggregate repo reads via infrastructure adapters",
		"infrastructure.FindAllCharters(":                    "application should aggregate repo reads via infrastructure adapters",
		"infrastructure.ReadCharterLenient(":                 "application should aggregate repo reads via infrastructure adapters",
		"infrastructure.ReadDesignDocFrontmatterWithConfig(": "application should resolve projection metadata via infrastructure adapters",
		"infrastructure.AutoSelectFormat(":                   "application should prepare spec creation via infrastructure adapters",
		"infrastructure.BuildDesignDocMutation(":             "application should prepare spec creation via infrastructure adapters",
		"*infrastructure.Workspace":                          "service dependencies should use infrastructure interfaces, not concrete adapters",
		"*infrastructure.RegistryStore":                      "service dependencies should use infrastructure interfaces, not concrete adapters",
	}
	fileSpecificBans := map[string]map[string]string{
		"adjacent_writes.go": {
			"registryStore().PersistCharter(":   "charter persistence and reload should go through the dedicated mutation adapter",
			"registryStore().PersistConfig(":    "config persistence and reload should go through the dedicated mutation adapter",
			"bufio.NewScanner(":                 "hook stdin normalization belongs in infrastructure adapters",
			"domain.NormalizeRepoDir(":          "config path normalization belongs in infrastructure mutations",
			"parseManagedSpecsPath(":            "managed .specs path parsing belongs in infrastructure adapters",
			"pathAdapter().EnsureSourcePrefix(": "config path normalization belongs in infrastructure mutations",
		},
		"diff.go": {
			"checkpointStore().LoadTrackingAtRevision(": "diff snapshots should be loaded through typed comparison adapters",
			"checkpointStore().ReadGitFile(":            "diff snapshots should be loaded through typed comparison adapters",
			"pathAdapter().ReadRepoFileIfExists(":       "diff snapshots should be loaded through typed comparison adapters",
		},
		"service.go": {
			"inferSpecTargetFromFile(": "file-target inference belongs in infrastructure adapters",
		},
		"spec_writes.go": {
			"checkpointStore().LoadTrackingAtRevision(":                          "revision snapshots should be loaded through typed comparison adapters",
			"checkpointStore().ReadGitFile(":                                     "revision snapshots should be loaded through typed comparison adapters",
			"pathAdapter().ReadRepoFile(":                                        "revision snapshots should be loaded through typed comparison adapters",
			`strings.Contains(err.Error(), "multiple configured formats match")`: "spec create should switch on typed infrastructure planning errors",
			`strings.Contains(err.Error(), "does not match tracking")`:           "spec create should switch on typed infrastructure planning errors",
			`strings.Contains(err.Error(), "must stay within the repository")`:   "verify-file handling should switch on typed infrastructure path errors",
			`strings.Contains(err.Error(), "must be relative")`:                  "verify-file handling should switch on typed infrastructure path errors",
			`strings.HasPrefix(err.Error(), "design document:")`:                 "spec create should switch on typed infrastructure planning errors",
			`strings.HasPrefix(err.Error(), "scope:")`:                           "spec create should switch on typed infrastructure planning errors",
			"SplitFrontmatterForDiff(":                                           "document diff normalization belongs in infrastructure adapters",
		},
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		fullPath := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, fullPath, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", fullPath, err)
		}
		for _, spec := range file.Imports {
			importPath := strings.Trim(spec.Path.Value, `"`)
			if reason, banned := bannedImports[importPath]; banned {
				t.Fatalf("%s imports %q: %s", name, importPath, reason)
			}
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", fullPath, err)
		}
		source := string(content)
		for snippet, reason := range bannedSnippets {
			if strings.Contains(source, snippet) {
				t.Fatalf("%s contains %q: %s", name, snippet, reason)
			}
		}
		for snippet, reason := range fileSpecificBans[name] {
			if strings.Contains(source, snippet) {
				t.Fatalf("%s contains %q: %s", name, snippet, reason)
			}
		}
	}
}

func TestServiceAdaptersUseDedicatedConcreteAdapters(t *testing.T) {
	adapters := infrastructure.NewServiceAdapters("/repo")

	registryType := reflect.TypeOf(adapters.Registry)
	repoReadType := reflect.TypeOf(adapters.RepoReads)
	checkpointType := reflect.TypeOf(adapters.Checkpoints)

	if registryType == repoReadType || registryType == checkpointType || repoReadType == checkpointType {
		t.Fatalf(
			"expected dedicated concrete adapters, got registry=%v repoReads=%v checkpoints=%v",
			registryType,
			repoReadType,
			checkpointType,
		)
	}
}
