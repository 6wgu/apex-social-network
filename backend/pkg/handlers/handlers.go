// Package handlers contains the HTTP API: routing and one file per feature.
package handlers

import (
	"database/sql"
	"net/http"
)

// App carries the shared dependencies every handler needs.
type App struct {
	DB        *sql.DB
	UploadDir string
}

// Routes registers every endpoint on a standard library ServeMux.
// New features add their routes here, following docs/api.md.
func (a *App) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", a.health)

	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(a.UploadDir))))

	// Unknown /api paths get the JSON error shape instead of the default text 404.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	return mux
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := a.DB.PingContext(r.Context()); err != nil {
		dbStatus = "unreachable"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": dbStatus})
}
