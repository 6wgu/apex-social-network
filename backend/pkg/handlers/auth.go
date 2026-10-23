package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"social-network/pkg/middleware"
	"social-network/pkg/models"
	"social-network/pkg/session"
)

const dummyHash = "$2a$10$ao.LYsoUHM3TPEfBMuDpQ.rXm.NuBqHqcsLoXoGzgpPRZdOxuUMna"

const userColumns = `id, email, first_name, last_name, date_of_birth, avatar_path,
	nickname, about_me, is_public, is_verified, strftime('%Y-%m-%d %H:%M:%S', created_at)`

func scanUser(row interface{ Scan(...any) error }) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.DateOfBirth, &u.AvatarPath,
		&u.Nickname, &u.AboutMe, &u.IsPublic, &u.IsVerified, &u.CreatedAt)
	return u, err
}

func (a *App) userByID(id int64) (models.User, error) {
	return scanUser(a.DB.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

func currentUserID(r *http.Request) int64 {
	id, _ := middleware.UserID(r)
	return id
}

func (a *App) startSession(w http.ResponseWriter, userID int64) error {
	id, expires, err := session.Create(a.DB, userID)
	if err != nil {
		return err
	}
	session.SetCookie(w, id, expires)
	return nil
}

func (a *App) register(w http.ResponseWriter, r *http.Request) {
	defer removeFormFiles(r)
	if !parseForm(w, r) {
		return
	}
	reg := readRegistration(r)
	if problems := reg.problems(time.Now()); len(problems) > 0 {
		writeFieldErrors(w, http.StatusBadRequest, problems)
		return
	}

	taken := fieldErrors{}
	var exists bool
	if err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = ?)`, reg.Email).Scan(&exists); err != nil {
		serverError(w, err)
		return
	}
	if exists {
		taken["email"] = conflictMessages["email"]
	}
	nicknameInUse, err := a.nicknameInUse(reg.Nickname, 0)
	if err != nil {
		serverError(w, err)
		return
	}
	if nicknameInUse {
		taken["nickname"] = nicknameTaken
	}
	if len(taken) > 0 {
		writeFieldErrors(w, http.StatusConflict, taken)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(reg.Password), bcrypt.DefaultCost)
	if err != nil {
		serverError(w, err)
		return
	}

	avatarPath, err := a.saveImage(r, "avatar")
	var uploadErr uploadError
	if errors.As(err, &uploadErr) {
		writeFieldErrors(w, http.StatusBadRequest, fieldErrors{"avatar": "avatar: " + uploadErr.Error()})
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	res, err := a.DB.Exec(`INSERT INTO users (email, password_hash, first_name, last_name, date_of_birth, avatar_path, nickname, about_me)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		reg.Email, string(hash), reg.FirstName, reg.LastName, reg.DateOfBirth, avatarPath, reg.Nickname, reg.AboutMe)
	if err != nil {
		a.removeImage(avatarPath)
		if field := uniqueConflict(err); field != "" {
			writeFieldErrors(w, http.StatusConflict, fieldErrors{field: conflictMessages[field]})
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
	a.sendWelcome(user)
	writeJSON(w, http.StatusCreated, user)
}

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
	unknown := errors.Is(err, sql.ErrNoRows)
	if unknown {
		hash = dummyHash
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil || unknown {
		writeError(w, http.StatusUnauthorized, "wrong email or password")
		return
	}

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

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	session.ClearCookie(w)
	if c, err := r.Cookie(session.CookieName); err == nil {
		userID, err := session.Get(a.DB, c.Value)
		if err != nil && !errors.Is(err, session.ErrNotFound) {
			serverError(w, err)
			return
		}
		if err := session.Delete(a.DB, c.Value); err != nil {
			serverError(w, err)
			return
		}
		if userID != 0 {
			a.Hub.CloseUser(userID)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	user, err := a.userByID(currentUserID(r))
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}
