package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"social-network/pkg/otp"
)

// Tests for "Forgot password?" (password.go). Run from backend/:
// go test ./pkg/handlers

const newPassword = "Brand-New-Pass2"

var resetMail = regexp.MustCompile(`(?s)^\[console mode\] email to ([^:]+): .*password reset code is (\d{6})`)

// resetCodes waits for the emails sent in the background and returns the
// address and code of every reset code email, oldest first.
func (e *testEnv) resetCodes() [][2]string {
	e.app.mailing.Wait()
	e.mu.Lock()
	defer e.mu.Unlock()
	var out [][2]string
	for _, m := range e.mails {
		if found := resetMail.FindStringSubmatch(m); found != nil {
			out = append(out, [2]string{found[1], found[2]})
		}
	}
	return out
}

// resetCode returns the newest reset code sent to the address.
func (e *testEnv) resetCode(t *testing.T, email string) string {
	t.Helper()
	all := e.resetCodes()
	for i := len(all) - 1; i >= 0; i-- {
		if all[i][0] == email {
			return all[i][1]
		}
	}
	t.Fatalf("no reset code was sent to %s", email)
	return ""
}

// mailsWith counts the emails, sent so far, whose log line has every part.
func (e *testEnv) mailsWith(parts ...string) int {
	e.app.mailing.Wait()
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, m := range e.mails {
		all := true
		for _, p := range parts {
			all = all && strings.Contains(m, p)
		}
		if all {
			n++
		}
	}
	return n
}

// post sends a JSON body from the client address ip ("" for none, as
// through the Next.js proxy) and returns the status, the raw body and the
// headers.
func (c *client) post(path, body, ip string) (int, string, http.Header) {
	c.t.Helper()
	req, err := http.NewRequest("POST", c.base+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
	}
	res, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(raw), res.Header
}

func forgotBody(email string) string { return `{"email":"` + email + `"}` }

func codeBody(email, code string) string {
	return `{"email":"` + email + `","code":"` + code + `"}`
}

func resetBody(token, password string) string {
	return `{"token":"` + token + `","password":"` + password + `"}`
}

// wrongCode returns a code that is not code.
func wrongCode(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}

// verifiedUser registers a verified account and returns its logged-in
// browser and id.
func (e *testEnv) verifiedUser(t *testing.T, email string) (*client, int64) {
	t.Helper()
	c, id := e.register(t, email, nil)
	e.markVerified(t, id)
	return c, id
}

// resetToken asks for a code for email and trades it for a token.
func (e *testEnv) resetToken(t *testing.T, email string) string {
	t.Helper()
	c := e.client(t)
	if status, body, _ := c.post("/api/password/forgot", forgotBody(email), ""); status != http.StatusOK {
		t.Fatalf("forgot %s = %d %s", email, status, body)
	}
	status, body := c.json("POST", "/api/password/verify-code", codeBody(email, e.resetCode(t, email)))
	token, _ := body["token"].(string)
	if status != http.StatusOK || token == "" {
		t.Fatalf("verify-code = %d %v, want 200 with a token", status, body)
	}
	return token
}

func (e *testEnv) login(t *testing.T, email, password string) (*client, int) {
	t.Helper()
	c := e.client(t)
	status, _ := c.json("POST", "/api/login", `{"email":"`+email+`","password":"`+password+`"}`)
	return c, status
}

func TestForgotPasswordAnswersTheSameForAnUnknownEmail(t *testing.T) {
	e := newTestEnv(t)
	e.verifiedUser(t, "ada@example.com")
	// Sending is slow; the answer must not wait for it.
	e.slowMail.Store(int64(500 * time.Millisecond))
	c := e.client(t)

	start := time.Now()
	realStatus, realBody, _ := c.post("/api/password/forgot", forgotBody(" ADA@example.com "), "")
	took := time.Since(start)
	unknownStatus, unknownBody, _ := c.post("/api/password/forgot", forgotBody("nobody@example.com"), "")
	if realStatus != http.StatusOK || unknownStatus != http.StatusOK || realBody != unknownBody {
		t.Fatalf("forgot = %d %s for an account, %d %s for none; want the same 200", realStatus, realBody, unknownStatus, unknownBody)
	}
	if !strings.Contains(realBody, "If an account exists for this email, we sent a code.") {
		t.Errorf("forgot body = %s", realBody)
	}
	if took >= 500*time.Millisecond {
		t.Errorf("forgot for an account took %v: it waited for the email", took)
	}

	codes := e.resetCodes()
	if len(codes) != 1 || codes[0][0] != "ada@example.com" {
		t.Fatalf("reset emails = %v, want one, to ada@example.com", codes)
	}
	if n := e.mailsWith("nobody@example.com"); n != 0 {
		t.Errorf("%d emails to the unknown address, want none", n)
	}

	// A wrong code answers alike too, attempts and all.
	wrong := wrongCode(codes[0][1])
	_, realWrong, _ := c.post("/api/password/verify-code", codeBody("ada@example.com", wrong), "")
	_, unknownWrong, _ := c.post("/api/password/verify-code", codeBody("nobody@example.com", wrong), "")
	if realWrong != unknownWrong || !strings.Contains(realWrong, `"attempts_left":4`) {
		t.Fatalf("wrong code = %s for an account, %s for none; want the same, with 4 attempts left", realWrong, unknownWrong)
	}
	// So does the cooldown.
	realStatus, realBody, _ = c.post("/api/password/forgot", forgotBody("ada@example.com"), "")
	unknownStatus, unknownBody, _ = c.post("/api/password/forgot", forgotBody("nobody@example.com"), "")
	if realStatus != http.StatusTooManyRequests || realStatus != unknownStatus || realBody != unknownBody {
		t.Fatalf("second forgot = %d %s for an account, %d %s for none; want the same 429", realStatus, realBody, unknownStatus, unknownBody)
	}
}

func TestForgotPasswordChecksTheEmail(t *testing.T) {
	e := newTestEnv(t)
	c := e.client(t)
	for body, want := range map[string]string{
		`{"email":""}`:    "email is required",
		`{"email":"a@b"}`: "email must be a valid address like name@example.com",
	} {
		status, got := c.json("POST", "/api/password/forgot", body)
		fields, _ := got["fields"].(map[string]any)
		if status != http.StatusBadRequest || fields["email"] != want {
			t.Errorf("%s = %d %v, want 400 %q", body, status, got, want)
		}
	}
	if status, _ := c.json("POST", "/api/password/forgot", `{"email":"a@example.com","x":1}`); status != http.StatusBadRequest {
		t.Errorf("unknown field = %d, want 400", status)
	}
	if status, _ := c.json("POST", "/api/password/verify-code", `{"email":"a@example.com"}`); status != http.StatusBadRequest {
		t.Errorf("verify-code without a code = %d, want 400", status)
	}
}

func TestCorrectResetCodeGivesAToken(t *testing.T) {
	e := newTestEnv(t)
	e.verifiedUser(t, "ada@example.com")
	c := e.client(t)
	c.post("/api/password/forgot", forgotBody("ada@example.com"), "")
	code := e.resetCode(t, "ada@example.com")

	status, body := c.json("POST", "/api/password/verify-code", codeBody("Ada@Example.com", " "+code+" "))
	token, _ := body["token"].(string)
	if status != http.StatusOK || token == "" || body["expires_in"] != float64(15*60) {
		t.Fatalf("verify-code = %d %v, want 200 with a token for 15 minutes", status, body)
	}
	// Only the token's hash is stored, and the code is spent.
	if e.count(t, `SELECT COUNT(*) FROM password_reset_tokens WHERE token_hash = ?`, token) != 0 ||
		e.count(t, `SELECT COUNT(*) FROM password_reset_tokens WHERE token_hash = ?`, hashToken(token)) != 1 {
		t.Error("password_reset_tokens does not hold just the token's hash")
	}
	status, body = c.json("POST", "/api/password/verify-code", codeBody("ada@example.com", code))
	if status != http.StatusBadRequest || body["reason"] != otp.Missing {
		t.Errorf("the same code again = %d %v, want 400 missing", status, body)
	}
}

func TestWrongResetCodesCountAndLockOut(t *testing.T) {
	e := newTestEnv(t)
	e.verifiedUser(t, "ada@example.com")
	c := e.client(t)
	c.post("/api/password/forgot", forgotBody("ada@example.com"), "")
	code := e.resetCode(t, "ada@example.com")
	wrong := wrongCode(code)

	for left := 4; left > 0; left-- {
		status, body := c.json("POST", "/api/password/verify-code", codeBody("ada@example.com", wrong))
		if status != http.StatusBadRequest || body["reason"] != otp.Wrong || body["attempts_left"] != float64(left) ||
			body["error"] != fmt.Sprintf("wrong code, %d attempts left", left) {
			t.Fatalf("wrong code = %d %v, want 400 with %d attempts left", status, body, left)
		}
	}
	status, body := c.json("POST", "/api/password/verify-code", codeBody("ada@example.com", wrong))
	if status != http.StatusBadRequest || body["reason"] != otp.TooMany {
		t.Fatalf("fifth wrong code = %d %v, want 400 too_many", status, body)
	}
	// The code is gone: even the right one fails now.
	status, body = c.json("POST", "/api/password/verify-code", codeBody("ada@example.com", code))
	if status != http.StatusBadRequest || body["reason"] != otp.Missing {
		t.Fatalf("right code after the lockout = %d %v, want 400 missing", status, body)
	}
}

func TestExpiredResetCodeAndTokenAreRejected(t *testing.T) {
	e := newTestEnv(t)
	e.verifiedUser(t, "ada@example.com")
	c := e.client(t)
	c.post("/api/password/forgot", forgotBody("ada@example.com"), "")
	code := e.resetCode(t, "ada@example.com")
	e.clock.Add(11 * time.Minute)
	status, body := c.json("POST", "/api/password/verify-code", codeBody("ada@example.com", code))
	if status != http.StatusBadRequest || body["reason"] != otp.Expired {
		t.Fatalf("code after 11 minutes = %d %v, want 400 expired", status, body)
	}

	token := e.resetToken(t, "ada@example.com")
	e.clock.Add(resetTokenTTL + time.Second)
	status, body = c.json("POST", "/api/password/reset", resetBody(token, newPassword))
	if status != http.StatusBadRequest || body["reason"] != "invalid_token" {
		t.Fatalf("reset with an expired token = %d %v, want 400 invalid_token", status, body)
	}
	if _, status := e.login(t, "ada@example.com", strongPassword); status != http.StatusOK {
		t.Errorf("login with the old password = %d, want 200: nothing should have changed", status)
	}
}

func TestResetTokenWorksOnce(t *testing.T) {
	e := newTestEnv(t)
	e.verifiedUser(t, "ada@example.com")
	token := e.resetToken(t, "ada@example.com")
	c := e.client(t)
	if status, body := c.json("POST", "/api/password/reset", resetBody(token, newPassword)); status != http.StatusOK || body["status"] != "changed" {
		t.Fatalf("reset = %d %v, want 200", status, body)
	}
	status, body := c.json("POST", "/api/password/reset", resetBody(token, "Another-Pass3"))
	if status != http.StatusBadRequest || body["reason"] != "invalid_token" {
		t.Fatalf("second reset with the same token = %d %v, want 400 invalid_token", status, body)
	}
	if status, body := c.json("POST", "/api/password/reset", resetBody("made-up", newPassword)); status != http.StatusBadRequest || body["reason"] != "invalid_token" {
		t.Errorf("reset with a made-up token = %d %v, want 400 invalid_token", status, body)
	}
	if _, status := e.login(t, "ada@example.com", newPassword); status != http.StatusOK {
		t.Errorf("login with the first new password = %d, want 200", status)
	}
}

func TestVerificationAndResetCodesAreSeparate(t *testing.T) {
	e := newTestEnv(t)
	c, _ := e.register(t, "ada@example.com", nil)
	if status, body := c.json("POST", "/api/otp/send", `{}`); status != http.StatusOK {
		t.Fatalf("otp send = %d %v", status, body)
	}
	_, verifyCode := e.lastCode(t)

	// With no reset code asked for, the verification code finds nothing.
	status, body := c.json("POST", "/api/password/verify-code", codeBody("ada@example.com", verifyCode))
	if status != http.StatusBadRequest || body["reason"] != otp.Missing {
		t.Fatalf("verification code as a reset code = %d %v, want 400 missing", status, body)
	}
	c.post("/api/password/forgot", forgotBody("ada@example.com"), "")
	resetCode := e.resetCode(t, "ada@example.com")
	if resetCode == verifyCode {
		t.Skip("the two random codes happen to be equal")
	}
	status, body = c.json("POST", "/api/password/verify-code", codeBody("ada@example.com", verifyCode))
	if status != http.StatusBadRequest || body["reason"] != otp.Wrong {
		t.Fatalf("verification code against a reset code = %d %v, want 400 wrong", status, body)
	}
	status, body = c.json("POST", "/api/otp/verify", `{"code":"`+resetCode+`"}`)
	if status != http.StatusBadRequest || body["reason"] != otp.Wrong {
		t.Fatalf("reset code as a verification code = %d %v, want 400 wrong", status, body)
	}
	// Each still works for its own purpose.
	if status, body := c.json("POST", "/api/password/verify-code", codeBody("ada@example.com", resetCode)); status != http.StatusOK {
		t.Fatalf("reset code = %d %v, want 200", status, body)
	}
	if status, body := c.json("POST", "/api/otp/verify", `{"code":"`+verifyCode+`"}`); status != http.StatusOK || body["is_verified"] != true {
		t.Fatalf("verification code = %d %v, want 200 verified", status, body)
	}
}

func TestWeakNewPasswordIsRejected(t *testing.T) {
	e := newTestEnv(t)
	e.verifiedUser(t, "ada@example.com")
	token := e.resetToken(t, "ada@example.com")
	c := e.client(t)
	for password, want := range map[string]string{
		"":         "password is required",
		"weakpass": "password needs an uppercase letter, a number and a special character",
	} {
		status, body := c.json("POST", "/api/password/reset", resetBody(token, password))
		fields, _ := body["fields"].(map[string]any)
		if status != http.StatusBadRequest || fields["password"] != want || body["error"] != want {
			t.Errorf("reset to %q = %d %v, want 400 %q", password, status, body, want)
		}
	}
	// The token was not spent on them.
	if status, body := c.json("POST", "/api/password/reset", resetBody(token, newPassword)); status != http.StatusOK {
		t.Fatalf("reset to a strong password = %d %v, want 200", status, body)
	}
}

func TestResetChangesThePasswordAndEndsEverySession(t *testing.T) {
	e := newTestEnv(t)
	first, id := e.verifiedUser(t, "ada@example.com")
	second, status := e.login(t, "ada@example.com", strongPassword)
	if status != http.StatusOK {
		t.Fatalf("login = %d", status)
	}
	other, otherID := e.verifiedUser(t, "bob@example.com")

	// A live connection of the user, which the reset must close.
	dialer := websocket.Dialer{Jar: first.http.Jar}
	conn, _, err := dialer.Dial("ws"+strings.TrimPrefix(e.url, "http")+"/ws", http.Header{"Origin": {"http://localhost:3000"}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for deadline := time.Now().Add(2 * time.Second); e.app.Hub.Connections(id) == 0; {
		if time.Now().After(deadline) {
			t.Fatal("the WebSocket never registered")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// A signed-in browser can use the routes too.
	if status, body, _ := first.post("/api/password/forgot", forgotBody("ada@example.com"), ""); status != http.StatusOK {
		t.Fatalf("forgot with a session = %d %s", status, body)
	}
	status, body := first.json("POST", "/api/password/verify-code", codeBody("ada@example.com", e.resetCode(t, "ada@example.com")))
	token, _ := body["token"].(string)
	if status != http.StatusOK {
		t.Fatalf("verify-code = %d %v", status, body)
	}
	if status, body := first.json("POST", "/api/password/reset", resetBody(token, newPassword)); status != http.StatusOK {
		t.Fatalf("reset = %d %v", status, body)
	}

	for name, c := range map[string]*client{"first": first, "second": second} {
		if status, _ := c.json("GET", "/api/me", ""); status != http.StatusUnauthorized {
			t.Errorf("GET /api/me in the %s browser after the reset = %d, want 401", name, status)
		}
	}
	if n := e.count(t, `SELECT COUNT(*) FROM sessions WHERE user_id = ?`, id); n != 0 {
		t.Errorf("%d sessions left, want 0", n)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil || e.app.Hub.Connections(id) != 0 {
		t.Errorf("the WebSocket is still open (read error %v)", err)
	}
	if _, status := e.login(t, "ada@example.com", strongPassword); status != http.StatusUnauthorized {
		t.Errorf("login with the old password = %d, want 401", status)
	}
	if _, status := e.login(t, "ada@example.com", newPassword); status != http.StatusOK {
		t.Errorf("login with the new password = %d, want 200", status)
	}
	if n := e.mailsWith("ada@example.com", "Your APEX password was changed"); n != 1 {
		t.Errorf("%d password changed emails, want 1", n)
	}
	// Nobody else was logged out.
	if status, _ := other.json("GET", "/api/me", ""); status != http.StatusOK || e.count(t, `SELECT COUNT(*) FROM sessions WHERE user_id = ?`, otherID) != 1 {
		t.Errorf("another user's session = %d, want it kept", status)
	}
}

func TestResetVerifiesTheEmail(t *testing.T) {
	e := newTestEnv(t)
	_, id := e.register(t, "new@example.com", nil)
	token := e.resetToken(t, "new@example.com")
	if status, body := e.client(t).json("POST", "/api/password/reset", resetBody(token, newPassword)); status != http.StatusOK {
		t.Fatalf("reset = %d %v", status, body)
	}
	if e.count(t, `SELECT is_verified FROM users WHERE id = ?`, id) != 1 {
		t.Error("the account is still unverified after the reset")
	}
}

func TestForgotPasswordLimits(t *testing.T) {
	e := newTestEnv(t)
	e.verifiedUser(t, "ada@example.com")
	c := e.client(t)

	// One address: forgotLimit codes an hour, each after the cooldown.
	for i := range forgotLimit {
		if status, body, _ := c.post("/api/password/forgot", forgotBody("ada@example.com"), ""); status != http.StatusOK {
			t.Fatalf("request %d = %d %s, want 200", i+1, status, body)
		}
		e.clock.Add(time.Minute)
	}
	status, body, header := c.post("/api/password/forgot", forgotBody("ada@example.com"), "")
	if status != http.StatusTooManyRequests || header.Get("Retry-After") == "" || !strings.Contains(body, "retry_after") {
		t.Fatalf("request over the limit = %d %s (Retry-After %q), want 429", status, body, header.Get("Retry-After"))
	}
	if n := len(e.resetCodes()); n != forgotLimit {
		t.Errorf("%d reset emails, want %d", n, forgotLimit)
	}
	e.clock.Add(forgotWindow)
	if status, body, _ := c.post("/api/password/forgot", forgotBody("ada@example.com"), ""); status != http.StatusOK {
		t.Fatalf("request an hour later = %d %s, want 200", status, body)
	}

	// One client: forgotLimit requests an hour, whatever the addresses.
	for i := range forgotLimit {
		if status, body, _ := c.post("/api/password/forgot", forgotBody(fmt.Sprintf("x%d@example.com", i)), "203.0.113.7"); status != http.StatusOK {
			t.Fatalf("client request %d = %d %s, want 200", i+1, status, body)
		}
	}
	if status, _, _ := c.post("/api/password/forgot", forgotBody("y@example.com"), "203.0.113.7"); status != http.StatusTooManyRequests {
		t.Fatalf("client request over the limit = %d, want 429", status)
	}
	if status, _, _ := c.post("/api/password/forgot", forgotBody("y@example.com"), "198.51.100.9"); status != http.StatusOK {
		t.Fatalf("another client = %d, want 200", status)
	}
}

func TestClientIP(t *testing.T) {
	for _, tc := range []struct{ remote, forwarded, want string }{
		{"203.0.113.7:5000", "", "203.0.113.7"},
		{"203.0.113.7:5000", "10.0.0.1", "203.0.113.7"}, // a public client cannot pick its address
		{"127.0.0.1:5000", "198.51.100.1, 198.51.100.2", "198.51.100.2"},
		{"172.18.0.3:5000", "198.51.100.1", "198.51.100.1"},
		{"[::1]:5000", "", ""}, // the proxy itself, without a client
		{"127.0.0.1:5000", "not an ip", ""},
	} {
		r := httptest.NewRequest("POST", "/api/password/forgot", nil)
		r.RemoteAddr = tc.remote
		if tc.forwarded != "" {
			r.Header.Set("X-Forwarded-For", tc.forwarded)
		}
		if got := clientIP(r); got != tc.want {
			t.Errorf("clientIP(%s, X-Forwarded-For %q) = %q, want %q", tc.remote, tc.forwarded, got, tc.want)
		}
	}
}
