package querier

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func registerConsoleStaticRoutes(mux *http.ServeMux, dist string) {
	dist = filepath.Clean(dist)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/console/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /console", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/console/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /console/", func(w http.ResponseWriter, r *http.Request) {
		serveConsoleAsset(w, r, dist)
	})
}

func serveConsoleAsset(w http.ResponseWriter, r *http.Request, dist string) {
	rel := strings.TrimPrefix(r.URL.Path, "/console/")
	if rel == "" {
		rel = "index.html"
	}
	candidate := filepath.Join(dist, filepath.FromSlash(rel))
	if fileInfo, err := os.Stat(candidate); err == nil && !fileInfo.IsDir() {
		http.ServeFile(w, r, candidate)
		return
	}
	http.ServeFile(w, r, filepath.Join(dist, "index.html"))
}
