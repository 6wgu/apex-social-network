package handlers

import (
	"fmt"
	"log"
	"net/http"

	"social-network/pkg/otp"
)

func otpSubject(userID int64) string {
	return fmt.Sprintf("user:%d", userID)
}

func (a *App) sendOTP(w http.ResponseWriter, r *http.Request) {
	user, err := a.userByID(currentUserID(r))
	if err != nil {
		serverError(w, err)
		return
	}
	if user.IsVerified {
		writeError(w, http.StatusConflict, "email already verified")
		return
	}
	res, err := a.OTP.Send(otpSubject(user.ID), user.Email, user.FirstName)
	if err != nil {
		log.Printf("otp send for user %d: %v", user.ID, err)
		writeError(w, http.StatusBadGateway, "could not send the verification email, please try again later")
		return
	}
	if res.Reason == otp.Cooldown {
		w.Header().Set("Retry-After", fmt.Sprint(res.RetryAfter))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":       fmt.Sprintf("please wait %d seconds before asking for a new code", res.RetryAfter),
			"retry_after": res.RetryAfter,
		})
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
	user, err := a.userByID(userID)
	if err != nil {
		serverError(w, err)
		return
	}
	if user.IsVerified {
		writeError(w, http.StatusConflict, "email already verified")
		return
	}
	res, err := a.OTP.Verify(otpSubject(userID), body.Code)
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
