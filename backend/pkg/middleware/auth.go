package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"social-network/pkg/session"
)

// userIDKey is the context key for the logged-in user's ID.
type userIDKey struct{}

// RequireAuth lets a request through only when it carries a valid session
// cookie, and stores the user's ID in the request context. Anything else gets
// a 401 in the API's JSON error shape.
func RequireAuth(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := session.FromRequest(db, r)
			if errors.Is(err, session.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "not logged in")
				return
			}
			if err != nil {
				log.Printf("internal error: read session: %v", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, userID)))
		})
	}
}

// UserID returns the logged-in user's ID. It is only meaningful inside a
// handler wrapped by RequireAuth; ok is false anywhere else.
func UserID(r *http.Request) (id int64, ok bool) {
	id, ok = r.Context().Value(userIDKey{}).(int64)
	return id, ok
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
