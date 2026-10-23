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

const (
	resetTokenTTL = 15 * time.Minute
	forgotLimit   = 5
	forgotWindow  = time.Hour
)

var forgotSent = map[string]string{"status": "sent", "message": "If an account exists for this email, we sent a code."}

const badResetToken = "this password reset has expired or was already used, ask for a new code"

func resetSubject(email string) string {
	return "email:" + email
}

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
		a.mailing.Go(func() {
			log.Printf("reset code for %s: %s", email, code)
			if err := a.OTP.Deliver(otp.ResetPassword, code, email, name); err != nil {
				log.Printf("password reset code for user %d: %v", userID, err)
			}
		})
	}
	writeJSON(w, http.StatusOK, forgotSent)
}

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

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

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

func writeTooManyResets(w http.ResponseWriter, wait time.Duration) {
	seconds := int((wait + time.Second - 1) / time.Second)
	w.Header().Set("Retry-After", fmt.Sprint(seconds))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{
		"error":       fmt.Sprintf("too many password reset requests, try again in %d minutes", (seconds+59)/60),
		"retry_after": seconds,
	})
}

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

type requestLog struct {
	mu    sync.Mutex
	times map[string][]time.Time
	swept time.Time
}

func (l *requestLog) take(now time.Time, keys ...string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.times == nil {
		l.times = map[string][]time.Time{}
	}
	since := now.Add(-forgotWindow)
	if now.Sub(l.swept) > forgotWindow {
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
