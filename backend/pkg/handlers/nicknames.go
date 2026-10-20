package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mattn/go-sqlite3"

	"social-network/pkg/session"
)

// Nicknames are optional but unique when set, without regard to case
// ("Ahmed" and "ahmed" are the same). The unique index from migration
// 000019 (idx_users_nickname) is the final word; the handlers check first
// so they can answer with a clear message, and turn the index's error into
// the same 409 when two requests race for one nickname.

// nicknameTaken is the 409 message for a nickname another user has.
const nicknameTaken = "This nickname is already taken"

// conflictMessages are the 409 messages for the unique user fields.
var conflictMessages = map[string]string{
	"email":    "email already registered",
	"nickname": nicknameTaken,
}

// nicknameInUse reports whether a user other than exceptID (0: no one is
// excepted) has nickname, ignoring case. An empty nickname is never in use.
// The query is the shape idx_users_nickname answers, so it stays cheap.
func (a *App) nicknameInUse(nickname string, exceptID int64) (bool, error) {
	if nickname == "" {
		return false, nil
	}
	var inUse bool
	err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users
		WHERE lower(nickname) = lower(?1) AND nickname <> '' AND id <> ?2)`, nickname, exceptID).Scan(&inUse)
	return inUse, err
}

// uniqueConflict names the field whose unique constraint err broke
// ("email" or "nickname"), or "" when err is something else.
func uniqueConflict(err error) string {
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.ExtendedCode != sqlite3.ErrConstraintUnique {
		return ""
	}
	switch msg := sqliteErr.Error(); {
	case strings.Contains(msg, "idx_users_nickname"):
		return "nickname"
	case strings.Contains(msg, "users.email"):
		return "email"
	}
	return ""
}

// nicknameAvailable handles GET /api/nicknames/available?nickname=. It is
// public (the register page needs it) and answers only {"available": bool}
// for a nickname in the valid format; a bad format is a 400 in the
// register field-error shape. For a signed-in caller their own nickname
// counts as available, so the edit form can change its case.
func (a *App) nicknameAvailable(w http.ResponseWriter, r *http.Request) {
	nickname := strings.TrimSpace(r.URL.Query().Get("nickname"))
	problem := validateNickname(nickname)
	if nickname == "" {
		problem = "nickname is required"
	}
	if problem != "" {
		writeFieldErrors(w, http.StatusBadRequest, fieldErrors{"nickname": problem})
		return
	}
	self, err := session.FromRequest(a.DB, r)
	if err != nil && !errors.Is(err, session.ErrNotFound) {
		serverError(w, err)
		return
	}
	inUse, err := a.nicknameInUse(nickname, self)
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]bool{"available": !inUse})
}
