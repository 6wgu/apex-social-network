// Package handlers contains the HTTP API: routing and one file per feature.
package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"social-network/pkg/middleware"
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
	auth := middleware.RequireAuth(a.DB)
	protected := func(h http.HandlerFunc) http.Handler { return auth(h) }

	// Public: the only routes that work without a session.
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("POST /api/register", a.register)
	mux.HandleFunc("POST /api/login", a.login)

	// Everything below needs a valid session cookie (401 otherwise).
	mux.Handle("POST /api/logout", protected(a.logout))
	mux.Handle("GET /api/me", protected(a.me))
	mux.Handle("PATCH /api/me", protected(a.updateMe))
	mux.Handle("PATCH /api/me/privacy", protected(a.updatePrivacy))
	mux.Handle("GET /api/users", protected(a.searchUsers))
	mux.Handle("GET /api/users/{id}", protected(a.getProfile))
	mux.Handle("GET /uploads/", auth(a.uploads()))
	mux.Handle("GET /uploads", auth(a.uploads())) // 404, not a redirect to the folder

	// Everything that matched no route above gets the JSON error shape instead
	// of the default text response.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		a.noRoute(mux, w, r)
	})
	return mux
}

// noRoute answers a request that matched no route: 405 with an Allow header
// when the path exists for other methods, 404 otherwise.
func (a *App) noRoute(mux *http.ServeMux, w http.ResponseWriter, r *http.Request) {
	var allowed []string
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		probe := r.Clone(r.Context())
		probe.Method = method
		if _, pattern := mux.Handler(probe); pattern != "/" && pattern != "" {
			allowed = append(allowed, method)
		}
	}
	if len(allowed) > 0 {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeError(w, http.StatusNotFound, "not found")
}

// uploads serves the stored images. Files live flat in UploadDir, so any path
// that is empty or has a separator is a directory request and gets a 404
// instead of the file server's listing. nosniff stops browsers from guessing
// a different type than the one we send.
func (a *App) uploads() http.Handler {
	files := http.StripPrefix("/uploads/", http.FileServer(http.Dir(a.UploadDir)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		name := strings.TrimPrefix(r.URL.Path, "/uploads/")
		if name == "" || strings.ContainsAny(name, `/\`) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if info, err := os.Stat(filepath.Join(a.UploadDir, name)); err != nil || info.IsDir() {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		files.ServeHTTP(w, r)
	})
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := a.DB.PingContext(r.Context()); err != nil {
		dbStatus = "unreachable"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": dbStatus})
}
