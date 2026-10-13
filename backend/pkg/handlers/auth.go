package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"

	"social-network/pkg/middleware"
	"social-network/pkg/models"
	"social-network/pkg/session"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 72 // bcrypt ignores everything after 72 bytes
	maxNameLen     = 50
	maxNicknameLen = 30
	maxAboutLen    = 500
	dateLayout     = "2006-01-02"
)

// dummyHash is a bcrypt hash (same cost as real ones) of a password nobody
// has. Login compares against it when the email is unknown, so an unknown
// email takes as long to reject as a wrong password and response times do
// not reveal which emails have accounts.
const dummyHash = "$2a$10$ao.LYsoUHM3TPEfBMuDpQ.rXm.NuBqHqcsLoXoGzgpPRZdOxuUMna"

// userColumns is the column list every query that fills a models.User selects.
// created_at is formatted so it reads like SQLite's own datetime strings.
const userColumns = `id, email, first_name, last_name, date_of_birth, avatar_path,
	nickname, about_me, is_public, strftime('%Y-%m-%d %H:%M:%S', created_at)`

// scanUser reads one row selected with userColumns.
func scanUser(row interface{ Scan(...any) error }) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.DateOfBirth, &u.AvatarPath,
		&u.Nickname, &u.AboutMe, &u.IsPublic, &u.CreatedAt)
	return u, err
}

// userByID loads a user. It returns sql.ErrNoRows when there is none.
func (a *App) userByID(id int64) (models.User, error) {
	return scanUser(a.DB.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// currentUserID returns the ID RequireAuth stored for this request.
func currentUserID(r *http.Request) int64 {
	id, _ := middleware.UserID(r)
	return id
}

// validEmail accepts a plain address like "ada@example.com".
func validEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return false
	}
	at := strings.LastIndex(email, "@")
	domain := email[at+1:]
	return strings.Contains(domain, ".") && !strings.HasPrefix(domain, ".") && !strings.HasSuffix(domain, ".")
}

// validateNickname and validateAbout are shared by register and edit profile.
func validateNickname(s string) string {
	if utf8.RuneCountInString(s) > maxNicknameLen {
		return "nickname must be 30 characters or fewer"
	}
	return ""
}

func validateAbout(s string) string {
	if utf8.RuneCountInString(s) > maxAboutLen {
		return "about me must be 500 characters or fewer"
	}
	return ""
}

// startSession logs the user in on this response: new session row plus cookie.
func (a *App) startSession(w http.ResponseWriter, userID int64) error {
	id, expires, err := session.Create(a.DB, userID)
	if err != nil {
		return err
	}
	session.SetCookie(w, id, expires)
	return nil
}

// register handles POST /api/register (multipart).
func (a *App) register(w http.ResponseWriter, r *http.Request) {
	defer removeFormFiles(r)
	if !parseForm(w, r) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	firstName := strings.TrimSpace(r.FormValue("first_name"))
	lastName := strings.TrimSpace(r.FormValue("last_name"))
	dob := strings.TrimSpace(r.FormValue("date_of_birth"))
	nickname := strings.TrimSpace(r.FormValue("nickname"))
	aboutMe := strings.TrimSpace(r.FormValue("about_me"))

	var problem string
	switch {
	case email == "":
		problem = "email is required"
	case !validEmail(email):
		problem = "email is not a valid address"
	case password == "":
		problem = "password is required"
	case utf8.RuneCountInString(password) < minPasswordLen:
		problem = "password must be at least 8 characters"
	case len(password) > maxPasswordLen:
		problem = "password must be 72 bytes or fewer"
	case firstName == "":
		problem = "first name is required"
	case lastName == "":
		problem = "last name is required"
	case utf8.RuneCountInString(firstName) > maxNameLen || utf8.RuneCountInString(lastName) > maxNameLen:
		problem = "names must be 50 characters or fewer"
	case dob == "":
		problem = "date of birth is required"
	default:
		born, err := time.Parse(dateLayout, dob)
		today := time.Now().UTC().Truncate(24 * time.Hour)
		switch {
		case err != nil:
			problem = "date of birth must be a valid date (YYYY-MM-DD)"
		case !born.Before(today):
			problem = "date of birth must be in the past"
		case born.Year() < 1900:
			problem = "date of birth is too far in the past"
		}
	}
	if problem == "" {
		problem = validateNickname(nickname)
	}
	if problem == "" {
		problem = validateAbout(aboutMe)
	}
	if problem != "" {
		writeError(w, http.StatusBadRequest, problem)
		return
	}

	// Check for a duplicate before storing the avatar, so a rejected
	// registration leaves no file behind. The UNIQUE constraint below still
	// catches two registrations racing each other.
	var exists bool
	if err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = ?)`, email).Scan(&exists); err != nil {
		serverError(w, err)
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		serverError(w, err)
		return
	}

	avatarPath, err := a.saveImage(r, "avatar")
	var uploadErr uploadError
	if errors.As(err, &uploadErr) {
		writeError(w, http.StatusBadRequest, "avatar: "+uploadErr.Error())
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	res, err := a.DB.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth, avatar_path, nickname, about_me)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		email, string(hash), firstName, lastName, dob, avatarPath, nickname, aboutMe)
	if err != nil {
		a.removeImage(avatarPath)
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		serverError(w, err)
		return
	}
	userID, err := res.LastInsertId()
	if err != nil {
		serverError(w, err)
		return
	}

	if err := a.startSession(w, userID); err != nil {
		serverError(w, err)
		return
	}
	user, err := a.userByID(userID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

// login handles POST /api/login.
func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	var userID int64
	var hash string
	err := a.DB.QueryRow(`SELECT id, password_hash FROM users WHERE email = ?`, email).Scan(&userID, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		serverError(w, err)
		return
	}
	// The same message, and the same bcrypt work, for an unknown email and a
	// wrong password, so neither the response nor its timing reveals which
	// emails have accounts.
	unknown := errors.Is(err, sql.ErrNoRows)
	if unknown {
		hash = dummyHash
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil || unknown {
		writeError(w, http.StatusUnauthorized, "wrong email or password")
		return
	}

	// Logging in is a convenient moment to clear out dead sessions.
	if err := session.DeleteExpired(a.DB); err != nil {
		serverError(w, err)
		return
	}
	if err := a.startSession(w, userID); err != nil {
		serverError(w, err)
		return
	}
	user, err := a.userByID(userID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// logout handles POST /api/logout. It is a public route on purpose: with an
// expired or unknown cookie it still answers 204 and clears the cookie, so the
// browser never keeps a dead one. A valid session row is deleted, so the
// cookie is useless even if someone kept a copy of it.
func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	session.ClearCookie(w)
	if c, err := r.Cookie(session.CookieName); err == nil {
		if err := session.Delete(a.DB, c.Value); err != nil {
			serverError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// me handles GET /api/me.
func (a *App) me(w http.ResponseWriter, r *http.Request) {
	user, err := a.userByID(currentUserID(r))
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}
