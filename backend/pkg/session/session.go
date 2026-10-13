// Package session stores login sessions in the sessions table and manages the
// cookie that carries the session ID. Nothing about the user lives in the
// cookie: it is only a random UUID that points at a row on the server.
package session

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	// CookieName is the cookie holding the session ID.
	CookieName = "session_id"
	// Lifetime is how long a session stays valid after login.
	Lifetime = 7 * 24 * time.Hour

	// timeLayout matches what SQLite's datetime('now') returns, so stored
	// expiry times compare correctly as text.
	timeLayout = "2006-01-02 15:04:05"
)

// ErrNotFound means there is no valid session: missing, unknown or expired.
var ErrNotFound = errors.New("session not found")

// SecureCookies adds the Secure flag to the session cookie, so the browser
// only sends it over HTTPS. server.go turns it on with COOKIE_SECURE=1; it
// stays off by default because the app runs on plain http://localhost.
var SecureCookies = false

// Create starts a session for userID and returns its ID and expiry time.
func Create(db *sql.DB, userID int64) (string, time.Time, error) {
	id := uuid.NewString()
	expires := time.Now().UTC().Add(Lifetime)
	_, err := db.Exec(`INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`,
		id, userID, expires.Format(timeLayout))
	if err != nil {
		return "", time.Time{}, err
	}
	return id, expires, nil
}

// Get returns the user who owns the session, or ErrNotFound when the ID is
// unknown or the session has expired.
func Get(db *sql.DB, id string) (int64, error) {
	if id == "" {
		return 0, ErrNotFound
	}
	var userID int64
	err := db.QueryRow(`SELECT user_id FROM sessions WHERE id = ? AND expires_at > datetime('now')`, id).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return userID, nil
}

// Delete removes one session. Deleting a session that is already gone is not an error.
func Delete(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteExpired removes every session past its expiry time.
func DeleteExpired(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE expires_at <= datetime('now')`)
	return err
}

// FromRequest returns the user behind the request's session cookie, or
// ErrNotFound when there is no cookie or no valid session for it.
func FromRequest(db *sql.DB, r *http.Request) (int64, error) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return 0, ErrNotFound
	}
	return Get(db, c.Value)
}

// SetCookie sends the session cookie. HttpOnly keeps it away from page
// scripts and SameSite=Lax keeps other sites from sending it on POSTs.
func SetCookie(w http.ResponseWriter, id string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    id,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCookie tells the browser to drop the session cookie.
func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}
