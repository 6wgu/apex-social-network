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

// What an account can do before its email is verified, besides asking for
// a code and entering it (otp.go): fix a mistyped email, or start over. An
// account nobody verifies is deleted after a while.

// UnverifiedAccountTTL is how long a new account has to verify its email.
// One still unverified after that is abandoned (a mistyped address, or
// nobody came back) and DeleteAbandonedAccounts removes it, so its email
// and nickname can be registered again. Verified accounts never expire.
const UnverifiedAccountTTL = 24 * time.Hour

// abandonedCheckEvery is how often a running server looks for them.
const abandonedCheckEvery = time.Hour

// changeEmail handles PATCH /api/me/email {"email"}: "Wrong email? Change
// it" on the verify page. Only an account that is not verified yet can use
// it (403). The address gets the same checks as at register: 400 for a bad
// format (or the address the account already has), 409 when another
// account has it. A new code then goes to the new address, replacing the
// old code, under the usual cooldown (429, and nothing changes). The
// account takes the address only once that code is out.
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
		// The code went to an address this account did not get after all.
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

// deleteMe handles DELETE /api/me: "Start over" on the verify page. Only an
// account that is not verified yet can be deleted (403 otherwise), which
// frees its email and nickname for a new registration. It logs out too: the
// sessions go with the account and the cookie is cleared.
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

// userImagesSQL lists the image files of a user's rows and of the rows ON
// DELETE CASCADE removes with them: the avatar, their posts and comments,
// the comments on their posts, and the posts and comments in groups they
// created. An unverified account has an avatar at most, but accounts made
// before verification was required can have the rest.
const userImagesSQL = `
SELECT avatar_path FROM users WHERE id = ?1
UNION SELECT image_path FROM posts
WHERE user_id = ?1 OR group_id IN (SELECT id FROM groups WHERE creator_id = ?1)
UNION SELECT c.image_path FROM comments c JOIN posts p ON p.id = c.post_id
WHERE c.user_id = ?1 OR p.user_id = ?1 OR p.group_id IN (SELECT id FROM groups WHERE creator_id = ?1)`

// deleteUnverifiedUser deletes the user with this id if their email is not
// verified yet, in one transaction: the users row, with it (ON DELETE
// CASCADE) their sessions and every other row that points at them, and their
// code, which otp_codes keeps without a foreign key. The image files of the
// deleted rows are removed once the transaction has committed. It reports
// false, and changes nothing, when there is no such unverified user.
func (a *App) deleteUnverifiedUser(id int64) (bool, error) {
	defer a.lockEmail(id)()
	tx, err := a.DB.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// Collect the files before the rows that name them disappear.
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

// DeleteAbandonedAccounts deletes every account that is, as of now, still
// unverified more than UnverifiedAccountTTL after it was created, each with
// its sessions and files (deleteUnverifiedUser), and returns how many it
// deleted.
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

// CleanUpAbandonedAccounts runs DeleteAbandonedAccounts right away and then
// every abandonedCheckEvery, until ctx is done. server.go starts it.
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
