package handlers

import "net/http"

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
