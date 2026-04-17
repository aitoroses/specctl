package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aitoroses/specctl/internal/application"
)

// GenerateStatic builds all API responses, injects them as window.__SPECCTL_DATA__
// into index.html, and writes all embedded assets to outputDir.
func GenerateStatic(svc *application.Service, dashboardFS embed.FS, outputDir string) error {
	regRaw, _, err := svc.ReadContext("", "")
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}
	registry, ok := regRaw.(application.RegistryProjection)
	if !ok {
		return fmt.Errorf("unexpected registry type from ReadContext")
	}

	specMap := make(map[string]application.SpecProjection, len(registry.Specs))
	for _, summary := range registry.Specs {
		target := summary.Charter + ":" + summary.Slug
		specRaw, _, serr := svc.ReadContext(target, "")
		if serr != nil {
			continue
		}
		spec, ok := specRaw.(application.SpecProjection)
		if !ok {
			continue
		}
		specMap[target] = spec
	}

	overview := buildOverview(registry, specMap)
	charters := buildCharters(registry, specMap)
	graph := buildGraph(specMap)
	specs := buildSpecDetails(specMap)

	dataObj := map[string]any{
		"overview": overview,
		"charters": charters,
		"graph":    graph,
		"specs":    specs,
	}

	dataJSON, err := json.Marshal(dataObj)
	if err != nil {
		return fmt.Errorf("marshaling dashboard data: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	return fs.WalkDir(dashboardFS, "dashboard/dist", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "dashboard/dist" {
			return nil
		}

		relPath := strings.TrimPrefix(path, "dashboard/dist/")
		outPath := filepath.Join(outputDir, filepath.FromSlash(relPath))

		if d.IsDir() {
			return os.MkdirAll(outPath, 0755)
		}

		data, err := dashboardFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading embedded file %s: %w", path, err)
		}

		if d.Name() == "index.html" {
			script := fmt.Sprintf("<script>window.__SPECCTL_DATA__ = %s</script>", dataJSON)
			data = []byte(strings.Replace(string(data), "</head>", script+"\n</head>", 1))
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}
		return os.WriteFile(outPath, data, 0644)
	})
}
