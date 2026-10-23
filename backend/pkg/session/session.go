package session

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	CookieName = "session_id"
	Lifetime   = 7 * 24 * time.Hour

	timeLayout = "2006-01-02 15:04:05"
)

var ErrNotFound = errors.New("session not found")

var SecureCookies = false

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

func Delete(db *sql.DB, id string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func DeleteExpired(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE expires_at <= datetime('now')`)
	return err
}

func FromRequest(db *sql.DB, r *http.Request) (int64, error) {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return 0, ErrNotFound
	}
	return Get(db, c.Value)
}

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
