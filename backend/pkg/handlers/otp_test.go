package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
)

type mailLog struct {
	mu   sync.Mutex
	logs []string
}

func (l *mailLog) add(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, fmt.Sprintf(format, args...))
}

var codeInMail = regexp.MustCompile(`verification code is (\d{6})`)

func (l *mailLog) lastCode(t *testing.T) string {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.logs) - 1; i >= 0; i-- {
		if m := codeInMail.FindStringSubmatch(l.logs[i]); m != nil {
			return m[1]
		}
	}
	t.Fatal("no code email was sent")
	return ""
}

func TestOTPVerifiesEmail(t *testing.T) {
	app := newTestApp(t)
	mails := &mailLog{}
	app.Mailer.Log = mails.add
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()

	anon := newClient(t, srv.URL)
	if status, _ := anon.json("POST", "/api/otp/send", `{}`); status != http.StatusUnauthorized {
		t.Fatalf("send without session = %d, want 401", status)
	}

	alice, _ := register(t, srv.URL, "alice@example.com", "Alice")
	if _, me := alice.get("/api/me"); me["is_verified"] != false {
		t.Fatalf("new user is_verified = %v, want false", me["is_verified"])
	}

	status, body := alice.json("POST", "/api/otp/send", `{}`)
	if status != http.StatusOK || body["email"] != "alice@example.com" {
		t.Fatalf("send = %d %v", status, body)
	}
	code := mails.lastCode(t)

	status, body = alice.json("POST", "/api/otp/send", `{}`)
	if status != http.StatusTooManyRequests || body["retry_after"] == nil {
		t.Fatalf("second send = %d %v, want 429 with retry_after", status, body)
	}

	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	status, body = alice.json("POST", "/api/otp/verify", `{"code":"`+wrong+`"}`)
	if status != http.StatusBadRequest || body["reason"] != "wrong" || body["attempts_left"] != float64(4) {
		t.Fatalf("wrong code = %d %v", status, body)
	}

	bob, _ := register(t, srv.URL, "bob@example.com", "Bob")
	if status, body := bob.json("POST", "/api/otp/verify", `{"code":"`+code+`"}`); status != http.StatusBadRequest || body["reason"] != "missing" {
		t.Fatalf("another user's code = %d %v, want 400 missing", status, body)
	}

	status, body = alice.json("POST", "/api/otp/verify", `{"code":"`+code+`"}`)
	if status != http.StatusOK || body["is_verified"] != true {
		t.Fatalf("right code = %d %v", status, body)
	}
	if _, me := alice.get("/api/me"); me["is_verified"] != true {
		t.Fatalf("after verify is_verified = %v", me["is_verified"])
	}
	if status, _ := alice.json("POST", "/api/otp/verify", `{"code":"`+code+`"}`); status != http.StatusConflict {
		t.Fatalf("verify again = %d, want 409", status)
	}
	if status, _ := alice.json("POST", "/api/otp/send", `{}`); status != http.StatusConflict {
		t.Fatalf("send after verified = %d, want 409", status)
	}
}

func TestOTPVerifyNeedsCode(t *testing.T) {
	app := newTestApp(t)
	srv := httptest.NewServer(app.Routes())
	defer srv.Close()
	c, _ := register(t, srv.URL, "carol@example.com", "Carol")
	if status, _ := c.json("POST", "/api/otp/verify", `{"code":""}`); status != http.StatusBadRequest {
		t.Fatalf("empty code = %d, want 400", status)
	}
	if status, _ := c.json("POST", "/api/otp/verify", `{"code":"123456","extra":1}`); status != http.StatusBadRequest {
		t.Fatalf("unknown field = %d, want 400", status)
	}
}
