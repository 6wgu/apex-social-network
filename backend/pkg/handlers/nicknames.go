package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mattn/go-sqlite3"

	"social-network/pkg/session"
)

const nicknameTaken = "This nickname is already taken"

var conflictMessages = map[string]string{
	"email":    "email already registered",
	"nickname": nicknameTaken,
}

func (a *App) nicknameInUse(nickname string, exceptID int64) (bool, error) {
	if nickname == "" {
		return false, nil
	}
	var inUse bool
	err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users
		WHERE lower(nickname) = lower(?1) AND nickname <> '' AND id <> ?2)`, nickname, exceptID).Scan(&inUse)
	return inUse, err
}

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
