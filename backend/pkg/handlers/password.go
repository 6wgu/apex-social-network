package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"social-network/pkg/otp"
)

// "Forgot password?": three public routes, used one after the other.
//
//  1. POST /api/password/forgot {email} emails a reset code to the address.
//  2. POST /api/password/verify-code {email, code} trades the code for a
//     short-lived reset token.
//  3. POST /api/password/reset {token, password} sets the new password.
//
// The first two never tell whether an account has the address. Every
// address gets the same 200, cooldown and limits, and a code to check
// against: for an address with no account it is a code nobody was sent,
// so wrong guesses count down and run out just the same.

const (
	// resetTokenTTL is how long a correct code's token can set a password.
	resetTokenTTL = 15 * time.Minute
	// forgotLimit is how many codes one address, or one client, can ask
	// for within forgotWindow.
	forgotLimit  = 5
	forgotWindow = time.Hour
)

// forgotSent is the answer to every request for a code that is let through.
var forgotSent = map[string]string{"status": "sent", "message": "If an account exists for this email, we sent a code."}

const badResetToken = "this password reset has expired or was already used, ask for a new code"

// resetSubject names the reset code of an address. Reset codes belong to
// the address, not to an account, so an address without one has a code too.
func resetSubject(email string) string {
	return "email:" + email
}

// lockAddress is lockEmail for the reset codes, which belong to an
// address: sending and checking the code of one address run one at a
// time, so racing guesses cannot share an attempt. Use it as
// defer a.lockAddress(email)().
func (a *App) lockAddress(email string) (unlock func()) {
	h := fnv.New32a()
	h.Write([]byte(email))
	mu := &a.addressLocks[h.Sum32()%uint32(len(a.addressLocks))]
	mu.Lock()
	return mu.Unlock
}

func (a *App) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

// forgotPassword handles POST /api/password/forgot {"email"}. A badly
// formed address is a 400 with fields.email, as at register. Otherwise the
// answer is the same 200 whether or not an account has the address, and
// only an account's owner is emailed. 429 (with Retry-After) during the
// resend cooldown of the address, and when the address or the client has
// asked forgotLimit times within forgotWindow.
func (a *App) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if problem := emailProblem(email); problem != "" {
		writeFieldErrors(w, http.StatusBadRequest, fieldErrors{"email": problem})
		return
	}
	now := a.now()
	keys := []string{"email:" + email}
	if ip := clientIP(r); ip != "" {
		keys = append(keys, "ip:"+ip)
	}
	if wait := a.forgotRequests.take(now, keys...); wait > 0 {
		writeTooManyResets(w, wait)
		return
	}

	defer a.lockAddress(email)()
	// Asking for a code is a convenient moment to clear out dead ones.
	if err := a.deleteExpiredResets(now); err != nil {
		serverError(w, err)
		return
	}
	var userID int64
	var name string
	err := a.DB.QueryRow(`SELECT id, first_name FROM users WHERE email = ?`, email).Scan(&userID, &name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		serverError(w, err)
		return
	}
	code, res, err := a.OTP.Issue(otp.ResetPassword, resetSubject(email))
	if err != nil {
		serverError(w, err)
		return
	}
	if res.Reason == otp.Cooldown {
		writeCooldown(w, res)
		return
	}
	if userID != 0 {
		// Sending takes seconds. Waiting for it would make an answer about
		// an existing account slower than one about a missing account.
		a.mailing.Go(func() {
			if err := a.OTP.Deliver(otp.ResetPassword, code, email, name); err != nil {
				log.Printf("password reset code for user %d: %v", userID, err)
			}
		})
	}
	writeJSON(w, http.StatusOK, forgotSent)
}

// checkResetCode handles POST /api/password/verify-code {"email","code"}.
// A code that does not work gets the same 400 as on /api/otp/verify
// (writeCodeProblem), for an address with or without an account. The right
// code works once and is traded for a token that can set a new password
// within resetTokenTTL: 200 {"token","expires_in"}.
func (a *App) checkResetCode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" || strings.TrimSpace(body.Code) == "" {
		writeError(w, http.StatusBadRequest, "email and code are required")
		return
	}
	defer a.lockAddress(email)()
	res, err := a.OTP.Verify(otp.ResetPassword, resetSubject(email), body.Code)
	if err != nil {
		serverError(w, err)
		return
	}
	if !res.OK {
		writeCodeProblem(w, res)
		return
	}
	// Only addresses with an account are sent a code, so the account is
	// there unless it went in the meantime.
	var userID int64
	err = a.DB.QueryRow(`SELECT id FROM users WHERE email = ?`, email).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		writeCodeProblem(w, otp.Result{Reason: otp.Missing})
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	token, err := a.newResetToken(userID)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expires_in": int(resetTokenTTL / time.Second)})
}

// resetPassword handles POST /api/password/reset {"token","password"}. The
// password must pass the register rules (400 with fields.password). The
// token works once and only until it expires (400 with reason
// "invalid_token" otherwise). The new password, the spent token and the end
// of every session of the user are one transaction; then the user's live
// connections are closed and they get an email about the change. An
// account that was not verified is verified now: the code proved the
// address is theirs.
func (a *App) resetPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if problem := passwordProblem(body.Password); problem != "" {
		writeFieldErrors(w, http.StatusBadRequest, fieldErrors{"password": problem})
		return
	}
	tokenHash := hashToken(body.Token)
	now := a.now().UTC().Format(time.DateTime)
	// Look first, so a made-up token costs no bcrypt work.
	var userID int64
	err := a.DB.QueryRow(`SELECT user_id FROM password_reset_tokens WHERE token_hash = ? AND expires_at > ?`, tokenHash, now).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		writeBadResetToken(w)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		serverError(w, err)
		return
	}

	tx, err := a.DB.Begin()
	if err != nil {
		serverError(w, err)
		return
	}
	defer tx.Rollback()
	// Of two requests with the same token, only the first still finds it.
	res, err := tx.Exec(`DELETE FROM password_reset_tokens WHERE token_hash = ? AND expires_at > ?`, tokenHash, now)
	if err != nil {
		serverError(w, err)
		return
	}
	n, err := res.RowsAffected()
	if err != nil {
		serverError(w, err)
		return
	}
	if n == 0 {
		writeBadResetToken(w)
		return
	}
	if _, err := tx.Exec(`UPDATE users SET password_hash = ?, is_verified = 1 WHERE id = ?`, string(hash), userID); err != nil {
		serverError(w, err)
		return
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		serverError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		serverError(w, err)
		return
	}
	a.Hub.CloseUser(userID)
	a.sendPasswordChanged(userID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "changed"})
}

// newResetToken makes the random token a correct code is traded for and
// stores its hash, replacing any earlier token of the user.
func (a *App) newResetToken(userID int64) (string, error) {
	token := rand.Text()
	tx, err := a.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ?`, userID); err != nil {
		return "", err
	}
	expires := a.now().UTC().Add(resetTokenTTL).Format(time.DateTime)
	if _, err := tx.Exec(`INSERT INTO password_reset_tokens (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		hashToken(token), userID, expires); err != nil {
		return "", err
	}
	return token, tx.Commit()
}

// hashToken is what password_reset_tokens stores of a token. The token is
// random and long, so a plain SHA-256 is enough.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// deleteExpiredResets removes the reset codes and tokens that have expired.
func (a *App) deleteExpiredResets(now time.Time) error {
	if _, err := a.DB.Exec(`DELETE FROM otp_codes WHERE purpose = ? AND expires_at <= ?`, otp.ResetPassword, now.UTC()); err != nil {
		return err
	}
	_, err := a.DB.Exec(`DELETE FROM password_reset_tokens WHERE expires_at <= ?`, now.UTC().Format(time.DateTime))
	return err
}

func writeBadResetToken(w http.ResponseWriter) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": badResetToken, "reason": "invalid_token"})
}

// writeTooManyResets answers 429 when an address or a client has asked
// for forgotLimit codes within forgotWindow.
func writeTooManyResets(w http.ResponseWriter, wait time.Duration) {
	seconds := int((wait + time.Second - 1) / time.Second)
	w.Header().Set("Retry-After", fmt.Sprint(seconds))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{
		"error":       fmt.Sprintf("too many password reset requests, try again in %d minutes", (seconds+59)/60),
		"retry_after": seconds,
	})
}

// clientIP is the client a request for a code counts against, or "" when
// it is not known. Browsers reach the backend through the Next.js server,
// so most requests come from its address, which says nothing about the
// browser: counting it would put every visitor in one bucket that anyone
// could fill. From a loopback or private address, where such a proxy
// runs, the client is the last address in X-Forwarded-For, the one the
// proxy nearest the backend added, if there is one. Anyone else is
// counted by their own address, whatever the header says.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer == nil {
		return ""
	}
	if !peer.IsLoopback() && !peer.IsPrivate() {
		return peer.String()
	}
	forwarded := r.Header.Values("X-Forwarded-For")
	if len(forwarded) == 0 {
		return ""
	}
	last := forwarded[len(forwarded)-1]
	if i := strings.LastIndexByte(last, ','); i >= 0 {
		last = last[i+1:]
	}
	if ip := net.ParseIP(strings.TrimSpace(last)); ip != nil {
		return ip.String()
	}
	return ""
}

// requestLog remembers when each key asked for a reset code, for the
// limit of forgotLimit within forgotWindow. The zero value is ready to use.
type requestLog struct {
	mu    sync.Mutex
	times map[string][]time.Time // oldest first
	swept time.Time
}

// take counts a request at now for every key and returns 0, unless a key
// already has forgotLimit requests within forgotWindow: then it counts
// nothing and returns how long until that key may ask again.
func (l *requestLog) take(now time.Time, keys ...string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.times == nil {
		l.times = map[string][]time.Time{}
	}
	since := now.Add(-forgotWindow)
	if now.Sub(l.swept) > forgotWindow {
		// Forget the keys with nothing recent, so the map does not grow.
		for key, times := range l.times {
			if len(times) == 0 || !times[len(times)-1].After(since) {
				delete(l.times, key)
			}
		}
		l.swept = now
	}
	var wait time.Duration
	for _, key := range keys {
		times := l.times[key]
		for len(times) > 0 && !times[0].After(since) {
			times = times[1:]
		}
		l.times[key] = times
		if len(times) >= forgotLimit {
			wait = max(wait, times[0].Sub(since))
		}
	}
	if wait > 0 {
		return wait
	}
	for _, key := range keys {
		l.times[key] = append(l.times[key], now)
	}
	return 0
}
