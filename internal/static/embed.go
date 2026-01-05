package static

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var distFS embed.FS

// HasEmbeddedFiles checks if the React app was embedded during build
func HasEmbeddedFiles() bool {
	entries, err := distFS.ReadDir("dist")
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// Handler returns an http.Handler for serving the embedded static files
func Handler() http.Handler {
	// Strip the "dist" prefix
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}

	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Try to serve the file directly
		if path != "/" && !strings.HasSuffix(path, "/") {
			// Check if file exists
			if f, err := sub.Open(strings.TrimPrefix(path, "/")); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// For SPA: serve index.html for all routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

