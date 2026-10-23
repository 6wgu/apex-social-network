package handlers

import (
	"fmt"
	"log"
	"net/http"

	"social-network/pkg/otp"
)

// sendFailed is the 502 message when the code email could not be sent.
const sendFailed = "could not send the verification email, please try again later"

func otpSubject(userID int64) string {
	return fmt.Sprintf("user:%d", userID)
}

// lockEmail makes the requests that send a code, check a code, change the
// email or delete an unverified account run one at a time per user, so a
// code is always checked against the address it was sent to. Use it as
// defer a.lockEmail(userID)(). Users who share a slot of emailLocks only
// wait for each other a moment.
func (a *App) lockEmail(userID int64) (unlock func()) {
	mu := &a.emailLocks[userID%int64(len(a.emailLocks))]
	mu.Lock()
	return mu.Unlock
}

// writeCooldown answers 429 when the last code went out too recently for
// another one.
func writeCooldown(w http.ResponseWriter, res otp.Result) {
	w.Header().Set("Retry-After", fmt.Sprint(res.RetryAfter))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{
		"error":       fmt.Sprintf("please wait %d seconds before asking for a new code", res.RetryAfter),
		"retry_after": res.RetryAfter,
	})
}

func (a *App) sendOTP(w http.ResponseWriter, r *http.Request) {
	userID := currentUserID(r)
	defer a.lockEmail(userID)()
	user, err := a.userByID(userID)
	if err != nil {
		serverError(w, err)
		return
	}
	if user.IsVerified {
		writeError(w, http.StatusConflict, "email already verified")
		return
	}
	res, err := a.OTP.Send(otp.VerifyEmail, otpSubject(user.ID), user.Email, user.FirstName)
	if err != nil {
		log.Printf("otp send for user %d: %v", user.ID, err)
		writeError(w, http.StatusBadGateway, sendFailed)
		return
	}
	if res.Reason == otp.Cooldown {
		writeCooldown(w, res)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent", "email": user.Email})
}

func (a *App) verifyOTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if body.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}
	userID := currentUserID(r)
	defer a.lockEmail(userID)()
	user, err := a.userByID(userID)
	if err != nil {
		serverError(w, err)
		return
	}
	if user.IsVerified {
		writeError(w, http.StatusConflict, "email already verified")
		return
	}
	res, err := a.OTP.Verify(otp.VerifyEmail, otpSubject(userID), body.Code)
	if err != nil {
		serverError(w, err)
		return
	}
	if !res.OK {
		msg := map[string]string{
			otp.Missing: "no active code, ask for a new one",
			otp.Expired: "the code has expired, ask for a new one",
			otp.TooMany: "too many wrong attempts, ask for a new one",
			otp.Wrong:   fmt.Sprintf("wrong code, %d attempts left", res.AttemptsLeft),
		}[res.Reason]
		out := map[string]any{"error": msg, "reason": res.Reason}
		if res.Reason == otp.Wrong {
			out["attempts_left"] = res.AttemptsLeft
		}
		writeJSON(w, http.StatusBadRequest, out)
		return
	}
	if _, err := a.DB.Exec(`UPDATE users SET is_verified = 1 WHERE id = ?`, userID); err != nil {
		serverError(w, err)
		return
	}
	user, err = a.userByID(userID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}
