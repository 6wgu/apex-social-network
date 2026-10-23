package handlers

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"social-network/pkg/otp"
	"social-network/pkg/session"
)

const UnverifiedAccountTTL = 24 * time.Hour

const abandonedCheckEvery = time.Hour

func (a *App) changeEmail(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	defer a.lockEmail(userID)()
	user, err := a.userByID(userID)
	if err != nil {
		serverError(w, err)
		return
	}
	if user.IsVerified {
		writeError(w, http.StatusForbidden, "your email is already verified")
		return
	}

	var body struct {
		Email string `json:"email"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	problem := emailProblem(email)
	if problem == "" && strings.EqualFold(email, user.Email) {
		problem = "that is already your email"
	}
	if problem != "" {
		writeFieldErrors(w, http.StatusBadRequest, fieldErrors{"email": problem})
		return
	}
	var taken bool
	if err := a.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = ?)`, email).Scan(&taken); err != nil {
		serverError(w, err)
		return
	}
	if taken {
		writeFieldErrors(w, http.StatusConflict, fieldErrors{"email": conflictMessages["email"]})
		return
	}

	res, err := a.OTP.Send(otp.VerifyEmail, otpSubject(userID), email, user.FirstName)
	if err != nil {
		log.Printf("otp send for user %d: %v", userID, err)
		writeError(w, http.StatusBadGateway, sendFailed)
		return
	}
	if res.Reason == otp.Cooldown {
		writeCooldown(w, res)
		return
	}
	if _, err := a.DB.Exec(`UPDATE users SET email = ? WHERE id = ?`, email, userID); err != nil {
		if cancelErr := a.OTP.Cancel(otp.VerifyEmail, otpSubject(userID)); cancelErr != nil {
			log.Printf("cancel code for user %d: %v", userID, cancelErr)
		}
		if uniqueConflict(err) == "email" {
			writeFieldErrors(w, http.StatusConflict, fieldErrors{"email": conflictMessages["email"]})
			return
		}
		serverError(w, err)
		return
	}
	user.Email = email
	writeJSON(w, http.StatusOK, user)
}

func (a *App) deleteMe(w http.ResponseWriter, r *http.Request) {
	deleted, err := a.deleteUnverifiedUser(currentUserID(r))
	if err != nil {
		serverError(w, err)
		return
	}
	if !deleted {
		writeError(w, http.StatusForbidden, "only an account that is not verified yet can be deleted")
		return
	}
	session.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

const userImagesSQL = `
SELECT avatar_path FROM users WHERE id = ?1
UNION SELECT image_path FROM posts
WHERE user_id = ?1 OR group_id IN (SELECT id FROM groups WHERE creator_id = ?1)
UNION SELECT c.image_path FROM comments c JOIN posts p ON p.id = c.post_id
WHERE c.user_id = ?1 OR p.user_id = ?1 OR p.group_id IN (SELECT id FROM groups WHERE creator_id = ?1)`

func (a *App) deleteUnverifiedUser(id int64) (bool, error) {
	defer a.lockEmail(id)()
	tx, err := a.DB.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(userImagesSQL, id)
	if err != nil {
		return false, err
	}
	var images []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			rows.Close()
			return false, err
		}
		images = append(images, path)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return false, err
	}

	res, err := tx.Exec(`DELETE FROM users WHERE id = ? AND is_verified = 0`, id)
	if err != nil {
		return false, err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return false, err
	}
	if _, err := tx.Exec(`DELETE FROM otp_codes WHERE subject = ?`, otpSubject(id)); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	for _, path := range images {
		a.removeImage(path)
	}
	return true, nil
}

func (a *App) DeleteAbandonedAccounts(now time.Time) (int, error) {
	cutoff := now.UTC().Add(-UnverifiedAccountTTL).Format(time.DateTime)
	rows, err := a.DB.Query(`SELECT id FROM users WHERE is_verified = 0 AND datetime(created_at) < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	deleted := 0
	for _, id := range ids {
		ok, err := a.deleteUnverifiedUser(id)
		if err != nil {
			return deleted, err
		}
		if ok {
			deleted++
		}
	}
	return deleted, nil
}

func (a *App) CleanUpAbandonedAccounts(ctx context.Context) {
	tick := time.NewTicker(abandonedCheckEvery)
	defer tick.Stop()
	for {
		if n, err := a.DeleteAbandonedAccounts(time.Now()); err != nil {
			log.Printf("abandoned accounts: %v", err)
		} else if n > 0 {
			log.Printf("abandoned accounts: deleted %d, unverified for more than %v", n, UnverifiedAccountTTL)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
