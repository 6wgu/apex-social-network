// Package handlers contains the HTTP API: routing and one file per feature.
package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"social-network/pkg/mail"
	"social-network/pkg/middleware"
	"social-network/pkg/otp"
	"social-network/pkg/ws"
)

// App carries the shared dependencies every handler needs.
type App struct {
	DB        *sql.DB
	UploadDir string
	Hub       *ws.Hub
	Mailer    *mail.Mailer
	OTP       *otp.Service
	// RequireVerified locks every signed-in route except GET and DELETE
	// /api/me, changing the email, the email code routes and logout until
	// the user has verified their email (403).
	RequireVerified bool

	emailLocks [16]sync.Mutex // see lockEmail
}

// Routes registers every endpoint on a standard library ServeMux.
// New features add their routes here, following docs/api.md.
func (a *App) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	// Frames browsers send over /ws (chat messages) are handled here too.
	a.Hub.SetHandler(a.handleSocketEvent)
	auth := middleware.RequireAuth(a.DB)
	// session: a valid session is enough. protected: the user must also have
	// verified their email, when RequireVerified is on.
	session := func(h http.HandlerFunc) http.Handler { return auth(h) }
	gate := func(h http.Handler) http.Handler {
		if a.RequireVerified {
			h = a.verifiedOnly(h)
		}
		return auth(h)
	}
	protected := func(h http.HandlerFunc) http.Handler { return gate(h) }

	// Public: the only routes that work without a session. Logout is public
	// so it can always clear the cookie, even an expired or unknown one.
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("POST /api/register", a.register)
	mux.HandleFunc("POST /api/login", a.login)
	mux.HandleFunc("POST /api/logout", a.logout)
	mux.HandleFunc("GET /api/nicknames/available", a.nicknameAvailable)

	// Everything below needs a valid session cookie (401 otherwise).
	mux.Handle("GET /ws", protected(a.serveWS))
	mux.Handle("GET /api/me", session(a.me))
	mux.Handle("DELETE /api/me", session(a.deleteMe))
	mux.Handle("PATCH /api/me", protected(a.updateMe))
	mux.Handle("PATCH /api/me/email", session(a.changeEmail))
	mux.Handle("PATCH /api/me/privacy", protected(a.updatePrivacy))
	mux.Handle("POST /api/otp/send", session(a.sendOTP))
	mux.Handle("POST /api/otp/verify", session(a.verifyOTP))
	mux.Handle("GET /api/users", protected(a.searchUsers))
	mux.Handle("GET /api/users/{id}", protected(a.getProfile))
	mux.Handle("GET /api/users/{id}/posts", protected(a.userPosts))
	mux.Handle("POST /api/users/{id}/follow", protected(a.follow))
	mux.Handle("DELETE /api/users/{id}/follow", protected(a.unfollow))
	mux.Handle("GET /api/users/{id}/followers", protected(a.listFollows(true)))
	mux.Handle("GET /api/users/{id}/following", protected(a.listFollows(false)))
	mux.Handle("GET /api/follow-requests", protected(a.listFollowRequests))
	mux.Handle("POST /api/follow-requests/{userId}/accept", protected(a.answerFollowRequest(true)))
	mux.Handle("POST /api/follow-requests/{userId}/decline", protected(a.answerFollowRequest(false)))
	mux.Handle("POST /api/posts", protected(a.createPost))
	mux.Handle("GET /api/posts/{id}", protected(a.getPost))
	mux.Handle("GET /api/posts/{id}/comments", protected(a.listComments))
	mux.Handle("POST /api/posts/{id}/comments", protected(a.createComment))
	mux.Handle("DELETE /api/posts/{id}", protected(a.deletePost))
	mux.Handle("DELETE /api/posts/{id}/comments/{commentId}", protected(a.deleteComment))
	mux.Handle("POST /api/posts/{id}/like", protected(a.likePost(true)))
	mux.Handle("DELETE /api/posts/{id}/like", protected(a.likePost(false)))
	mux.Handle("GET /api/feed", protected(a.feed))
	mux.Handle("GET /api/groups/{id}/messages", protected(a.groupMessageHistory))
	mux.Handle("GET /api/conversations", protected(a.listConversations))
	mux.Handle("GET /api/chat-contacts", protected(a.chatContacts))
	mux.Handle("GET /api/messages/unread-count", protected(a.unreadMessages))
	mux.Handle("GET /api/messages/{userId}", protected(a.messageHistory))
	mux.Handle("POST /api/messages/{userId}/read", protected(a.markConversationRead))
	mux.Handle("GET /api/notifications", protected(a.listNotifications))
	mux.Handle("GET /api/notifications/unread-count", protected(a.unreadNotifications))
	mux.Handle("POST /api/notifications/{id}/read", protected(a.readNotification))
	mux.Handle("POST /api/notifications/read-all", protected(a.readAllNotifications))
	mux.Handle("POST /api/groups", protected(a.createGroup))
	mux.Handle("GET /api/groups", protected(a.listGroups))
	mux.Handle("GET /api/groups/{id}", protected(a.getGroup))
	mux.Handle("POST /api/groups/{id}/invite", protected(a.inviteToGroup))
	mux.Handle("POST /api/group-invites/{inviteId}/accept", protected(a.answerGroupInvite(true)))
	mux.Handle("POST /api/group-invites/{inviteId}/decline", protected(a.answerGroupInvite(false)))
	mux.Handle("POST /api/groups/{id}/request", protected(a.requestToJoin))
	mux.Handle("DELETE /api/groups/{id}/request", protected(a.cancelJoinRequest))
	mux.Handle("GET /api/groups/{id}/requests", protected(a.listJoinRequests))
	mux.Handle("POST /api/group-requests/{requestId}/accept", protected(a.answerJoinRequest(true)))
	mux.Handle("POST /api/group-requests/{requestId}/decline", protected(a.answerJoinRequest(false)))
	mux.Handle("GET /api/groups/{id}/posts", protected(a.groupPosts))
	mux.Handle("POST /api/groups/{id}/posts", protected(a.createGroupPost))
	mux.Handle("GET /api/groups/{id}/events", protected(a.listEvents))
	mux.Handle("POST /api/groups/{id}/events", protected(a.createEvent))
	mux.Handle("POST /api/events/{id}/respond", protected(a.respondToEvent))
	mux.Handle("GET /uploads/", gate(a.uploads()))
	mux.Handle("GET /uploads", gate(a.uploads())) // 404, not a redirect to the folder

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
// instead of the file server's listing. A post's image follows the post's
// privacy (canViewUpload): a viewer who may not see the post gets the same
// 404 as for a missing file. middleware.SecurityHeaders adds nosniff, so
// browsers do not guess a different type than the one we send.
func (a *App) uploads() http.Handler {
	files := http.StripPrefix("/uploads/", http.FileServer(http.Dir(a.UploadDir)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/uploads/")
		if name == "" || strings.ContainsAny(name, `/\`) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if info, err := os.Stat(filepath.Join(a.UploadDir, name)); err != nil || info.IsDir() {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		allowed, err := a.canViewUpload(currentUserID(r), "/uploads/"+name)
		if err != nil {
			serverError(w, err)
			return
		}
		if !allowed {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		// Private images must not be kept by shared caches.
		w.Header().Set("Cache-Control", "private, no-cache")
		files.ServeHTTP(w, r)
	})
}

// serveWS handles GET /ws: RequireAuth has checked the session cookie sent
// with the upgrade request, so the connection belongs to that user.
func (a *App) serveWS(w http.ResponseWriter, r *http.Request) {
	a.Hub.Serve(w, r, currentUserID(r))
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := a.DB.PingContext(r.Context()); err != nil {
		dbStatus = "unreachable"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "database": dbStatus})
}
