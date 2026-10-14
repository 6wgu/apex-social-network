package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Unknown emails go through bcrypt too, so they take about as long as a
// wrong password (a plain lookup miss would answer in well under 1 ms).
func TestLoginUnknownEmailRunsBcrypt(t *testing.T) {
	srv := httptest.NewServer(newTestApp(t).Routes())
	defer srv.Close()
	c, _ := register(t, srv.URL, "ada@example.com", "Ada")

	timeLogin := func(email string) time.Duration {
		start := time.Now()
		status, body := c.json("POST", "/api/login", `{"email":"`+email+`","password":"wrong password"}`)
		if status != http.StatusUnauthorized || body["error"] != "wrong email or password" {
			t.Fatalf("login %s = %d %v", email, status, body)
		}
		return time.Since(start)
	}
	known, unknown := timeLogin("ada@example.com"), timeLogin("nobody@example.com")
	if unknown < known/3 {
		t.Errorf("unknown email took %v, wrong password %v: unknown emails skip bcrypt", unknown, known)
	}
}
