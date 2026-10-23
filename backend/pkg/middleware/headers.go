package middleware

import "net/http"

// SecurityHeaders adds headers that close off a few browser attacks for free:
// nosniff stops an uploaded file being run as a script or page whatever its
// bytes look like, DENY keeps every response out of frames on other sites,
// and the referrer policy keeps full URLs from leaking to other origins.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
