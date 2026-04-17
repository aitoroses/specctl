package dashboard

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aitoroses/specctl/internal/application"
	"github.com/fsnotify/fsnotify"
)

type dashboardState struct {
	overview OverviewResponse
	charters []CharterSummary
	graph    GraphResponse
	specs    map[string]SpecDetail
}

// Server serves the specctl dashboard over HTTP.
type Server struct {
	svc      *application.Service
	fs       embed.FS
	specsDir string

	mu    sync.RWMutex
	state *dashboardState
}

// NewServer creates a new dashboard HTTP server.
func NewServer(svc *application.Service, dashboardFS embed.FS, specsDir string) *Server {
	return &Server{
		svc:      svc,
		fs:       dashboardFS,
		specsDir: specsDir,
	}
}

// Start loads state, starts the fsnotify watcher, and begins serving on addr.
// It blocks until the server exits.
func (s *Server) Start(addr string) error {
	s.loadState()
	go s.startWatcher()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/overview", s.handleOverview)
	mux.HandleFunc("GET /api/charters", s.handleCharters)
	mux.HandleFunc("GET /api/specs/{charter}/{slug}", s.handleSpec)
	mux.HandleFunc("GET /api/graph", s.handleGraph)

	sub, err := fs.Sub(s.fs, "dashboard/dist")
	if err == nil {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			data, ferr := fs.ReadFile(sub, path)
			if ferr != nil {
				// SPA fallback: return index.html for unknown paths
				data, ferr = fs.ReadFile(sub, "index.html")
				if ferr != nil {
					http.Error(w, "not found", http.StatusNotFound)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(data) //nolint:errcheck
				return
			}

			http.ServeContent(w, r, path, time.Time{}, bytes.NewReader(data))
		})
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(mux),
	}
	return srv.ListenAndServe()
}

// loadState fetches all spec data via the service and rebuilds the cached state.
func (s *Server) loadState() {
	regRaw, _, err := s.svc.ReadContext("", "")
	if err != nil {
		return
	}
	registry, ok := regRaw.(application.RegistryProjection)
	if !ok {
		return
	}

	specMap := make(map[string]application.SpecProjection, len(registry.Specs))
	for _, summary := range registry.Specs {
		target := summary.Charter + ":" + summary.Slug
		specRaw, _, serr := s.svc.ReadContext(target, "")
		if serr != nil {
			continue
		}
		spec, ok := specRaw.(application.SpecProjection)
		if !ok {
			continue
		}
		specMap[target] = spec
	}

	state := &dashboardState{
		overview: buildOverview(registry, specMap),
		charters: buildCharters(registry, specMap),
		graph:    buildGraph(specMap),
		specs:    buildSpecDetails(specMap),
	}

	s.mu.Lock()
	s.state = state
	s.mu.Unlock()
}

// startWatcher monitors specsDir for YAML changes and debounces reload.
func (s *Server) startWatcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()

	// Add specsDir and all subdirectories
	_ = filepath.WalkDir(s.specsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		_ = watcher.Add(path)
		return nil
	})

	var (
		debounceTimer   *time.Timer
		debounceTimerMu sync.Mutex
	)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if strings.HasSuffix(event.Name, ".yaml") {
				debounceTimerMu.Lock()
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(time.Second, s.loadState)
				debounceTimerMu.Unlock()
			}
		case _, ok := <-watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
