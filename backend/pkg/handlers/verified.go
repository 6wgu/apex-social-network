package handlers

import "net/http"

// verifiedOnly lets the request through only when the logged-in user has
// verified their email. Anyone else gets 403, so an unverified account cannot
// reach the feed, posts, chat or anything else by calling the API directly.
// It runs after RequireAuth, so there is always a user id.
func (a *App) verifiedOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var verified bool
		err := a.DB.QueryRow(`SELECT is_verified FROM users WHERE id = ?`, currentUserID(r)).Scan(&verified)
		if err != nil {
			serverError(w, err)
			return
		}
		if !verified {
			writeError(w, http.StatusForbidden, "verify your email first")
			return
		}
		next.ServeHTTP(w, r)
	})
}
