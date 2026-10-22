package handlers

import (
	"log"
	"net/http"
	"strings"

	"social-network/pkg/otp"
)

// What an account can do before its email is verified, besides asking for
// a code and entering it (otp.go): fix a mistyped email.

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

	res, err := a.OTP.Send(otpSubject(userID), email, user.FirstName)
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
		if cancelErr := a.OTP.Cancel(otpSubject(userID)); cancelErr != nil {
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
